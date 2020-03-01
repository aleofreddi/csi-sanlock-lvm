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
	"github.com/aleofreddi/csi-sanlock-lvm/lvmctrld/pkg"
	"k8s.io/klog"
	"net"
	"os"
	"strconv"
)

var (
	hostAddr = flag.String("host-id-addr", "", "compute host id from ip address, mutually exclusive with host-id")
	hostId   = flag.String("host-id", "", "host id, mutually exclusive with host-id-addr")
	listen   = flag.String("listen", "tcp://0.0.0.0:9000", "listen address")
	version  string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	klog.Infof("Starting lvmctrld %s", version)

	// Parse host id
	id, err := parseHostId(*hostId, *hostAddr)
	if err != nil {
		klog.Errorf("Failed to parse host id: %s", err.Error())
		os.Exit(1)
	}

	// Start server
	listener, err := lvmctrld.NewListener(id, *listen)
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

func parseHostId(hostId string, hostAddr string) (uint16, error) {
	if hostId != "" && hostAddr != "" {
		klog.Errorf("Host id and node ip are mutually exclusive")
		os.Exit(1)
	}
	if hostId == "" && hostAddr == "" {
		klog.Errorf("Host id or node ip required")
		os.Exit(1)
	}

	if hostId != "" {
		v, err := strconv.ParseInt(hostId, 10, 16)
		if err != nil || v < 1 || v > 2000 {
			return 0, fmt.Errorf("invalid host id %s, expected a decimal in range [1, 2000]", hostId)
		}
		return uint16(v), nil
	}
	v, err := addressToHostId(hostAddr)
	if err != nil {
		return 0, fmt.Errorf("invalid address")
	}
	return v, nil
}

func addressToHostId(ip string) (uint16, error) {
	address := net.ParseIP(ip)
	if address == nil {
		return 0, fmt.Errorf("invalid ip address: %s", ip)
	}
	return uint16((binary.BigEndian.Uint32(address[len(address)-4:])&0x7ff)%1999 + 1), nil
}
