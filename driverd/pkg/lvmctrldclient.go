// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package driverd

import (
	"context"
	"github.com/aleofreddi/csi-sanlock-lvm/lvmctrld/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"net"
	"strings"
	"time"
)

type LvmCtrldClientConnection struct {
	conn *grpc.ClientConn
	proto.LvmCtrldClient
}

type LvmCtrldClientFactory interface {
	NewLocal() (*LvmCtrldClientConnection, error)
	NewRemote(address string) (*LvmCtrldClientConnection, error)
	NewForVolume(volumeId string, ctx context.Context) (*LvmCtrldClientConnection, error)
}

type concreteLvmCtrldClientFactory struct {
	lvmAddr string
	timeout time.Duration
}

func NewLvmCtrldClientFactory(lvmAddr string, timeout time.Duration) (LvmCtrldClientFactory, error) {
	return &concreteLvmCtrldClientFactory{
		lvmAddr: lvmAddr,
		timeout: timeout,
	}, nil
}

func (f *concreteLvmCtrldClientFactory) NewForVolume(volumeId string, ctx context.Context) (*LvmCtrldClientConnection, error) {
	// Connect to local lvmctrld
	client, err := f.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	// Retrieve locking node if any
	lv, err := client.Lvs(ctx, &proto.LvsRequest{
		Select: "lv_role!=snapshot",
		Target: volumeId,
	})
	if err != nil && status.Code(err) != codes.NotFound || lv != nil && len(lv.Lvs) > 1 {
		return nil, status.Errorf(codes.Internal, "failed to list volumes")
	}
	ownerAddr := ""
	if lv != nil && len(lv.Lvs) == 1 {
		for _, tag := range lv.Lvs[0].LvTags {
			decodedTag, _ := decodeTag(tag)
			if strings.HasPrefix(decodedTag, ownerTag) {
				if len(ownerAddr) > 0 {
					return nil, status.Errorf(codes.Internal, "volume %s has multiple owners", volumeId)
				}
				i := strings.Index(decodedTag, "@")
				if i == -1 || i == len(decodedTag)-1 {
					return nil, status.Errorf(codes.Internal, "volume %s has an invalid tag", volumeId)
				}
				ownerAddr = decodedTag[i+1:]
			}
		}
	}

	// If there is no owner return a local lvmctrld client, else connect to the remote one
	if len(ownerAddr) == 0 {
		client, err := f.NewLocal()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
		}
		return client, nil
	}
	rclient, err := f.NewRemote(ownerAddr)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld at %s: %s", ownerAddr, err.Error())
	}
	return rclient, nil
}

func (f *concreteLvmCtrldClientFactory) NewLocal() (*LvmCtrldClientConnection, error) {
	return f.NewRemote(f.lvmAddr)
}

func (f *concreteLvmCtrldClientFactory) NewRemote(address string) (*LvmCtrldClientConnection, error) {
	conn, err := connect(address, f.timeout)
	if err != nil {
		return nil, err
	}
	return &LvmCtrldClientConnection{
		LvmCtrldClient: proto.NewLvmCtrldClient(conn),
		conn:           conn,
	}, nil
}

func (c *LvmCtrldClientConnection) Close() error {
	return c.conn.Close()
}

func connect(address string, timeout time.Duration) (*grpc.ClientConn, error) {
	klog.V(2).Infof("Connecting to %s", address)
	dialOptions := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBackoffMaxDelay(time.Second),
		//grpc.WithUnaryInterceptor(LvmctrldLog),
	}
	dialOptions = append(dialOptions, grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
		proto, target, err := parseAddress(addr)
		if err != nil {
			return nil, err
		}
		return net.DialTimeout(proto, target, timeout)
	}))
	conn, err := grpc.Dial(address, dialOptions...)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		if !conn.WaitForStateChange(ctx, conn.GetState()) {
			klog.V(4).Infof("Connection timed out (%s)", conn.GetState())
			return conn, nil
		}
		if conn.GetState() == connectivity.Ready {
			klog.V(3).Infof("Connected")
			return conn, nil
		}
		klog.V(4).Infof("Still trying, connection %s", conn.GetState())
	}
}
