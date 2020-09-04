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
	"os"

	driverd "github.com/aleofreddi/csi-sanlock-lvm/driverd/pkg"
	"k8s.io/klog"
)

var (
	listen   = flag.String("listen", "unix:///tmp/csi.sock", "listen address")
	lvmctrld = flag.String("lvmctrld", "tcp://127.0.0.1:9000", "lvmctrld address")
	name     = flag.String("name", "csi-lvm-sanlock.vleo.net", "name of the driver")
	nodeId   = flag.String("node-id", "", "node id")
	version  string
	commit   string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	klog.Infof("Starting driverd %s (%s)", version, commit)

	// Start server
	listener, err := driverd.NewListener(*name, version, *nodeId, *listen, *lvmctrld)
	if err != nil {
		klog.Errorf("Failed to instance listener: %s", err.Error())
		os.Exit(1)
	}
	if err = listener.Init(); err != nil {
		klog.Errorf("Failed to initialize listener: %s", err.Error())
		os.Exit(1)
	}
	if err = listener.Run(); err != nil {
		klog.Errorf("Failed to run listener: %s", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
