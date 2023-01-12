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
	"fmt"
	"net"
	"net/url"
	"os"

	logger "github.com/aleofreddi/csi-sanlock-lvm/pkg/grpclogger"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog"
)

type Listener struct {
	addr string

	is *identityServer
	ns *nodeServer
	cs *controllerServer
}

func NewListener(addr string, is *identityServer, ns *nodeServer, cs *controllerServer) (*Listener, error) {
	if addr == "" {
		return nil, fmt.Errorf("missing listen address")
	}
	return &Listener{
		addr: addr,
		is:   is,
		ns:   ns,
		cs:   cs,
	}, nil
}

func (l *Listener) Run() error {
	// Start gRPC server
	lsProto, lsAddr, err := parseAddress(l.addr)
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
		grpc.UnaryInterceptor(logger.GrpcLogger),
	}
	grpcServer := grpc.NewServer(opts...)
	csi.RegisterIdentityServer(grpcServer, l.is)
	csi.RegisterNodeServer(grpcServer, l.ns)
	csi.RegisterControllerServer(grpcServer, l.cs)
	klog.Infof("Starting gRPC server")
	if err := grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to start server: %s", err.Error())
	}
	return nil
}

func parseAddress(addr string) (string, string, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse listen address: %s", err.Error())
	}
	if u.Host != "" && u.Path != "" {
		return "", "", fmt.Errorf("failed to parse listen address: invalid address")
	}
	if u.Host != "" {
		return u.Scheme, u.Host, nil
	}
	return u.Scheme, u.Path, nil
}
