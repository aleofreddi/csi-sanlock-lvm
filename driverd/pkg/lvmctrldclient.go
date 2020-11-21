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
	"net"
	"time"

	pb "github.com/aleofreddi/csi-sanlock-lvm/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"k8s.io/klog"
)

type LvmCtrldClientConnection struct {
	pb.LvmCtrldClient
	conn *grpc.ClientConn
}

func NewLvmCtrldClient(address string) (*LvmCtrldClientConnection, error) {
	conn, err := connect(address, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	return &LvmCtrldClientConnection{
		LvmCtrldClient: pb.NewLvmCtrldClient(conn),
		conn:           conn,
	}, nil
}

func (c *LvmCtrldClientConnection) Wait() error { // FIXME: add a timeout here!
	for {
		vgs, err := c.Vgs(context.Background(), &pb.VgsRequest{})
		if err != nil {
			klog.Infof("Failed to connect to lvmctrld (%s), retrying...", err.Error())
			time.Sleep(1 * time.Second)
			continue
		}
		klog.Infof("lvmctrld startup complete, found %d volume group(s)", len(vgs.Vgs))
		break
	}
	klog.Infof("Connected to lvmctrld")
	return nil
}

func (c *LvmCtrldClientConnection) Close() error {
	if c.conn == nil {
		return nil
	}
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
		protocol, target, err := parseAddress(addr)
		if err != nil {
			return nil, err
		}
		return net.DialTimeout(protocol, target, timeout)
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
