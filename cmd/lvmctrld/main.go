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
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/aleofreddi/csi-sanlock-lvm/pkg/lvmctrld"
	"k8s.io/klog"
)

var (
	noLock      = flag.Bool("no-lock", false, "disable locking, use the given host id. This option is mutually exclusive with lock-host-addr and lock-host-id")
	lockAddr    = flag.String("lock-host-addr", "", "enable locking, compute host id from the given ip address. This options is mutually exclusive with lock-host-id and no-lock")
	lockId      = flag.Uint("lock-host-id", 0, "enable locking, use the given host id. This option is mutually exclusive with lock-host-addr and no-lock")
	sanlockArgs = flag.String("sanlock-args", "", "additional arguments to pass to sanlock")
	listen      = flag.String("listen", "tcp://0.0.0.0:9000", "listen address")
	version     string
	commit      string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	klog.Infof("Starting lvmctrld %s (%s)", version, commit)

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

func bootstrap() (*lvmctrld.Listener, error) {
	if (*noLock == (*lockId != 0)) == ((*lockId != 0) == (*lockAddr != "")) {
		return nil, fmt.Errorf("invalid lock configuration, expected one of: no-lock|lock-host-addr|lock-host-id flag")
	}
	// Start global lock if needed.
	var id uint16
	var err error
	if *noLock {
		id = 1
	} else {
		id, err = parseHostId(*lockId, *lockAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse host id: %v", err)
		}
		if err = lvmctrld.StartLock(id, *sanlockArgs); err != nil {
			return nil, fmt.Errorf("failed to start lock: %v", err)
		}
	}
	// Start server.
	listener, err := lvmctrld.NewListener(*listen, id)
	if err != nil {
		return nil, fmt.Errorf("failed to instance listener: %v", err)
	}
	if err = listener.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize listener: %v", err)
	}
	return listener, nil
}

func parseHostId(hostId uint, hostAddr string) (uint16, error) {
	if hostId != 0 {
		if hostId > 2000 {
			return 0, fmt.Errorf("invalid host id %d, expected a value in range [1, 2000]", hostId)
		}
		return uint16(hostId), nil
	}
	id, err := addressToHostId(hostAddr)
	if err != nil {
		return 0, fmt.Errorf("invalid address: %v", err)
	}
	return id, nil
}

func addressToHostId(ip string) (uint16, error) {
	address := net.ParseIP(ip)
	if address == nil {
		return 0, fmt.Errorf("invalid ip address: %s", ip)
	}
	return uint16((binary.BigEndian.Uint32(address[len(address)-4:])&0x7ff)%1999 + 1), nil
}
