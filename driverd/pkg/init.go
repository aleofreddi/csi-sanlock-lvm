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
	"fmt"
	"github.com/aleofreddi/csi-sanlock-lvm/lvmctrld/proto"
	"k8s.io/klog"
	"strings"
	"time"
)

func (d *listener) Init() error {
	if err := waitForLvmctrld(d.lvmFact, d.lvmAddr); err != nil {
		return fmt.Errorf("failed while waiting for lvmctrld to be ready: %s", err.Error())
	}
	if err := updateLvmctrldAddr(d.lvmFact, d.nodeId, d.lvmAddr); err != nil {
		return fmt.Errorf("failed to update lvmctrld address: %s", err.Error())
	}
	return nil
}

func waitForLvmctrld(factory LvmCtrldClientFactory, lvmctrldAddr string) error {
	klog.Infof("Waiting for lvmctrld to be ready")

	client, err := factory.NewRemote(lvmctrldAddr)
	if err != nil {
		return err
	}
	defer client.Close()

	for {
		vgs, err := client.Vgs(context.Background(), &proto.VgsRequest{})
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

func updateLvmctrldAddr(factory LvmCtrldClientFactory, nodeId string, lvmctrldAddr string) error {
	klog.Infof("Updating tag for owned volumes")

	client, err := factory.NewRemote(lvmctrldAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	ctx := context.Background()
	lvs, err := client.Lvs(ctx, &proto.LvsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list logical volumes: %s", err.Error())
	}

	for _, lv := range lvs.Lvs {
		for _, encodedTag := range lv.LvTags {
			tag, err := decodeTag(encodedTag)
			if err != nil {
				return fmt.Errorf("invalid tag value %q: %s", encodedTag, err.Error())
			}
			if strings.HasPrefix(tag, fmt.Sprintf("%s%s@", ownerTag, nodeId)) {
				volumeId := fmt.Sprintf("%s/%s", lv.VgName, lv.LvName)
				// Update address
				_, err := client.LvChange(ctx, &proto.LvChangeRequest{
					Target: volumeId,
					DelTag: []string{encodedTag},
					AddTag: []string{encodeTag(getOwnerTag(nodeId, lvmctrldAddr))},
				})
				if err != nil {
					return fmt.Errorf("failed to update tags on %s: %s", volumeId, err.Error())
				}
			}
		}
	}
	return nil
}
