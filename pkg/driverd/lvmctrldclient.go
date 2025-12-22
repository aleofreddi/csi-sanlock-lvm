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

	pb "github.com/aleofreddi/csi-sanlock-lvm/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
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

func (c *LvmCtrldClientConnection) IsReady() bool {
	vgs, err := c.Vgs(context.Background(), &pb.VgsRequest{
		Select: "",
	})
	if err != nil {
		return false
	}
	klog.V(4).Infof("lvmctrld check: found %d volume group(s)", len(vgs.Vgs))
	return true
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
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// lvmcltrd is running in the same pod, backoff to at most 5 seconds.
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  time.Second,
				Multiplier: 1.2,
				MaxDelay:   5 * time.Second,
			},
		}),
		//grpc.WithUnaryInterceptor(LvmctrldLog),
	}
	dialOptions = append(dialOptions, grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		protocol, target, err := parseAddress(addr)
		if err != nil {
			return nil, err
		}
		return (&net.Dialer{}).DialContext(ctx, protocol, target)
	}))
	conn, err := grpc.NewClient(address, dialOptions...)
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
