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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	driverd "github.com/aleofreddi/csi-sanlock-lvm/driverd/pkg"
	"k8s.io/klog"
)

var (
	drvName   = flag.String("driver-name", "csi-lvm-sanlock.vleo.net", "driverName of the driver")
	listen    = flag.String("listen", "unix:///var/run/csi.sock", "listen address")
	lvmctrld  = flag.String("lvmctrld", "unix:///var/run/lvmctrld.sock", "lvmctrld address")
	nodeName  = flag.String("node-name", "", "node name")
	defaultFs = flag.String("default-fs", "ext4", "default filesystem to use when none is specified")
	version   string
	commit    string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	klog.Infof("Starting driverd %s (%s)", version, commit)

	listener, err := bootstrap()
	if err != nil {
		klog.Errorf("Bootstrap failed: %v", err)
		os.Exit(2)
	}
	if err = listener.Run(); err != nil {
		klog.Errorf("Execution failed: %v", err)
		os.Exit(3)
	}
	os.Exit(0)
}

func bootstrap() (*driverd.Listener, error) {
	// Retrieve hostname.
	var (
		node string
		err  error
	)
	if *nodeName != "" {
		node = *nodeName
	} else {
		node, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve hostname: %v", err)
		}
	}
	// Setup lvmctrld client.
	lvm, err := driverd.NewLvmCtrldClient(*lvmctrld)
	if err != nil {
		return nil, fmt.Errorf("failed to instance lvmctrld client: %v", err)
	}
	// Setup lock manager.
	vl, err := driverd.NewVolumeLocker(lvm, node)
	if err != nil {
		return nil, fmt.Errorf("failed to instance volume lock: %v", err)
	}
	// Setup DiskRPC.
	drpc, err := driverd.NewDiskRpcService(lvm, vl)
	if err != nil {
		return nil, fmt.Errorf("failed to instance disk rpc service: %v", err)
	}
	// Setup CSI servers.
	is, err := driverd.NewIdentityServer(*drvName, version, lvm)
	if err != nil {
		return nil, fmt.Errorf("failed to instance identity server: %v", err)
	}
	fsr, err := driverd.NewFileSystemRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to instance filesystem registry: %v", err)
	}
	ns, err := driverd.NewNodeServer(lvm, vl, fsr)
	if err != nil {
		return nil, fmt.Errorf("failed to instance identity server: %v", err)
	}
	cs, err := driverd.NewControllerServer(lvm, vl, drpc, *defaultFs)
	if err != nil {
		return nil, fmt.Errorf("failed to instance controller server: %v", err)
	}
	// Start server.
	listener, err := driverd.NewListener(*listen, is, ns, cs)
	if err != nil {
		return nil, fmt.Errorf("failed to instance listener: %s", err.Error())
	}
	// Start services.
	go func() {
		klog.Infof("Starting services")
		err := start(lvm, vl, drpc)
		if err != nil {
			klog.Fatalf("Startup failed: %v", err)
		} else {
			klog.Infof("Startup complete")
		}
	}()
	return listener, nil
}

func start(lvm *driverd.LvmCtrldClientConnection, vl driverd.VolumeLocker, drpc driverd.DiskRpcService) error {
	ctx := context.Background()
	// Wait for lvmctrld to be ready.
	if err := lvm.Wait(ctx); err != nil {
		return fmt.Errorf("lvmctrld startup failed: %v", err)
	}
	// Start VolumeLocker.
	if err := vl.Start(ctx); err != nil {
		return fmt.Errorf("failed to start disk rpc: %v", err)
	}
	// Start DiskRPC.
	if err := drpc.Start(ctx); err != nil {
		return fmt.Errorf("failed to start disk rpc: %v", err)
	}
	return nil
}
