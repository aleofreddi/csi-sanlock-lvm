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

package lvmctrld

import (
	"fmt"
	"github.com/aleofreddi/csi-sanlock-lvm/lvmctrld/proto"
	"google.golang.org/grpc"
	"k8s.io/klog"
	"net"
	"net/url"
	"os"
)

type listener struct {
	hostId uint16
	lsAddr string

	ls *lvmctrldServer
}

func NewListener(hostId uint16, lsAddr string) (*listener, error) {
	return &listener{
		hostId: hostId,
		lsAddr: lsAddr,

		ls: NewLvmctrldServer(),
	}, nil
}

func (l *listener) Init() error {
	if err := StartLock(l.hostId, []string{}); err != nil {
		return fmt.Errorf("failed to initialize lock: %s", err.Error())
	}
	return nil
}

func (l *listener) Run() error {
	// Start gRPC server
	lsProto, lsAddr, err := parseAddress(l.lsAddr)
	if err != nil {
		return fmt.Errorf("invalid listen address: %s", err.Error())
	}
	if lsProto == "unix" {
		if err := os.Remove(lsAddr); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %s", lsAddr, err.Error())
		}
	}
	klog.Infof("Binding proto %s, address %s", lsProto, lsAddr)
	listener, err := net.Listen(lsProto, lsAddr)
	if err != nil {
		return fmt.Errorf("failed to listen %s://%s: %s", lsProto, lsAddr, err.Error())
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(GrpcLogger),
	}
	grpcServer := grpc.NewServer(opts...)
	proto.RegisterLvmCtrldServer(grpcServer, l.ls)
	klog.Infof("Starting gRPC server")
	if err := grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to start server: %s", err.Error())
	}
	return nil
}

func parseAddress(addr string) (string, string, error) {
	u, err := url.Parse(addr)
	if err != nil || u.Host != "" && u.Path != "" {
		return "", "", fmt.Errorf("failed to parse listen address: %s", err.Error())
	}
	if u.Host != "" {
		return u.Scheme, u.Host, nil
	}
	return u.Scheme, u.Path, nil
}
