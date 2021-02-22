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
	"flag"
	"fmt"
	"os"

	driverd "github.com/aleofreddi/csi-sanlock-lvm/driverd/pkg"
	"k8s.io/klog"
)

var (
	drvName  = flag.String("driver-name", "csi-lvm-sanlock.vleo.net", "driverName of the driver")
	listen   = flag.String("listen", "unix:///var/run/csi.sock", "listen address")
	lvmctrld = flag.String("lvmctrld", "unix:///var/run/lvmctrld.sock", "lvmctrld address")
	nodeName = flag.String("node-name", "", "node name")
	defaultFs = flag.String("default-fs", "ext4", "default filesystem to use when none is specified")
	version  string
	commit   string
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
	// Start lvmctrld client
	client, err := driverd.NewLvmCtrldClient(*lvmctrld)
	if err != nil {
		return nil, fmt.Errorf("failed to instance lvmctrld client: %v", err)
	}

	// Wait for lvmctrld to be ready
	if err := client.Wait(); err != nil {
		return nil, fmt.Errorf("lvmctrld startup failed: %v", err)
	}

	// Retrieve hostname
	var node string
	if *nodeName != "" {
		node = *nodeName
	} else {
		node, err = os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve hostname: %v", err)
		}
	}

	// Start lock
	vl, err := driverd.NewVolumeLock(client, node)
	if err != nil {
		return nil, fmt.Errorf("failed to instance volume lock: %v", err)
	}
	// Start DiskRPC
	drpc, err := driverd.NewDiskRpc(client)
	if err != nil {
		return nil, fmt.Errorf("failed to instance disk rpc: %v", err)
	}
	// Instance servers
	is, err := driverd.NewIdentityServer(*drvName, version)
	if err != nil {
		return nil, fmt.Errorf("failed to instance identity server: %v", err)
	}
	ns, err := driverd.NewNodeServer(client, vl, driverd.NewFileSystem)
	if err != nil {
		return nil, fmt.Errorf("failed to instance identity server: %v", err)
	}
	cs, err := driverd.NewControllerServer(client, vl, drpc, *defaultFs)
	if err != nil {
		return nil, fmt.Errorf("failed to instance controller server: %v", err)
	}
	// Start DiskRPC
	if err := drpc.Start(); err != nil {
		return nil, fmt.Errorf("failed to start disk rpc: %v", err)
	}
	// Start server
	listener, err := driverd.NewListener(*listen, is, ns, cs)
	if err != nil {
		return nil, fmt.Errorf("failed to instance listener: %s", err.Error())
	}
	return listener, nil
}
