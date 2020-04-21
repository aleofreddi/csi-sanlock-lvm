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
	lvmctrld "github.com/aleofreddi/csi-sanlock-lvm/lvmctrld/pkg"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog"
	"net"
	"net/url"
	"os"
	"time"
)

type listener struct {
	name    string
	nodeId  string
	lsAddr  string
	lvmAddr string
	lvmFact LvmCtrldClientFactory

	iSrv *identityServer
	nSrv *nodeServer
	cSrv *controllerServer
}

func NewListener(name, version, nodeId, lsAddr, lvmAddr string) (*listener, error) {
	if name == "" {
		return nil, fmt.Errorf("missing driver name")
	}
	if version == "" {
		return nil, fmt.Errorf("missing driver version")
	}
	if lsAddr == "" {
		return nil, fmt.Errorf("missing listen address")
	}
	if nodeId == "" {
		return nil, fmt.Errorf("missing node id")
	}
	if lvmAddr == "" {
		return nil, fmt.Errorf("missing lvmctrld address")
	}

	cf, err := NewLvmCtrldClientFactory(lvmAddr, 30 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to instance lvmctrld client factory: %s", err.Error())
	}
	is, err := NewIdentityServer(name, version)
	if err != nil {
		return nil, fmt.Errorf("failed to instance identity server: %s", err.Error())
	}
	ns, err := NewNodeServer(nodeId, lvmAddr, cf)
	if err != nil {
		return nil, fmt.Errorf("failed to instance identity server: %s", err.Error())
	}
	cs, err := NewControllerServer(nodeId, lvmAddr, cf)
	if err != nil {
		return nil, fmt.Errorf("failed to instance controller server: %s", err.Error())
	}

	return &listener{
		name:    name,
		nodeId:  nodeId,
		lsAddr:  lsAddr,
		lvmAddr: lvmAddr,
		lvmFact: cf,
		iSrv:    is,
		nSrv:    ns,
		cSrv:    cs,
	}, nil
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
		grpc.UnaryInterceptor(lvmctrld.GrpcLogger),
	}
	grpcServer := grpc.NewServer(opts...)
	csi.RegisterIdentityServer(grpcServer, l.iSrv)
	csi.RegisterNodeServer(grpcServer, l.nSrv)
	csi.RegisterControllerServer(grpcServer, l.cSrv)
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
