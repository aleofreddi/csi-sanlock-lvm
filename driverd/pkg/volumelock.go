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
	"sync"

	"github.com/aleofreddi/csi-sanlock-lvm/proto"
	pb "github.com/aleofreddi/csi-sanlock-lvm/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

type volumeLock struct {
	baseServer
	nodeName string

	locks map[string]map[string]struct{}
	mutex *sync.Mutex
}

const (
	defaultLockOp = "stage"
)

func NewVolumeLock(lvmctrld pb.LvmCtrldClient, nodeName string) (*volumeLock, error) {
	bs, err := newBaseServer(lvmctrld)
	if err != nil {
		return nil, err
	}
	vl := &volumeLock{
		baseServer: *bs,
		nodeName:   nodeName,
		locks:      map[string]map[string]struct{}{},
		mutex:      &sync.Mutex{},
	}
	ctx := context.Background()
	if err = vl.sync(ctx); err != nil {
		return nil, fmt.Errorf("failed to sync LVM state: %v", err)
	}
	return vl, nil
}

func (vl *volumeLock) LockVolume(ctx context.Context, vol VolumeRef, op string) error {
	vl.mutex.Lock()
	defer vl.mutex.Unlock()

	key := vol.VgLv()
	m, ok := vl.locks[key]
	if !ok {
		m = make(map[string]struct{})
		vl.locks[key] = m
	}
	if len(m) > 0 {
		m[op] = struct{}{}
		return nil
	}

	// Lock the volume
	_, err := vl.lvmctrld.LvChange(ctx, &proto.LvChangeRequest{
		Target:   []string{vol.VgLv()},
		Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE,
	})
	if err != nil {
		return status.Errorf(status.Code(err), "failed to lock volume %s: %v", vol, err)
	}

	// Update tags
	err = vl.setOwner(ctx, vol, &vl.nodeID, &vl.nodeName)
	if err != nil {
		// Try to unlock the volume
		_, err2 := vl.lvmctrld.LvChange(ctx, &proto.LvChangeRequest{
			Target:   []string{vol.VgLv()},
			Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_NONE,
		})
		if err2 != nil {
			klog.Errorf("Failed to unlock volume %s: %v (ignoring error)", vol, err2)
		}
		return status.Errorf(status.Code(err), "failed to update owner tags on volume %s: %v", vol, err)
	}

	m[op] = struct{}{}
	return nil
}

func (vl *volumeLock) UnlockVolume(ctx context.Context, vol VolumeRef, op string) error {
	vl.mutex.Lock()
	defer vl.mutex.Unlock()

	// If volume is not locked, return
	key := vol.VgLv()
	m, ok := vl.locks[key]
	if !ok {
		return nil
	}
	delete(m, op)
	if len(m) > 0 {
		return nil
	}

	// Remove owner tags
	err := vl.setOwner(ctx, vol, nil, nil)
	if err != nil {
		return err
	}

	// Unlock the volume
	_, err = vl.lvmctrld.LvChange(ctx, &proto.LvChangeRequest{
		Target:   []string{vol.VgLv()},
		Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_DEACTIVATE,
	})
	if err != nil {
		return status.Errorf(status.Code(err), "failed to unlock volume %s: %v", vol, err)
	}

	delete(vl.locks, key)
	return nil
}

// Retrieve the owner node for a given volume.
func (vl *volumeLock) GetOwner(ctx context.Context, ref VolumeRef) (uint16, string, error) {
	// Fetch the volume and get its tags
	vol, err := vl.fetch(ctx, &ref)
	if err != nil {
		return 0, "", status.Errorf(codes.Internal, "failed to list logical volume %s: %v", vol, err)
	}
	tags, err := vol.Tags()
	if err != nil {
		return 0, "", status.Errorf(codes.Internal, "failed to decode tags on volume %s: %v", vol, err)
	}

	var nodeID uint16
	if v, ok := tags[ownerIdTagKey]; ok {
		nodeID, err = nodeIDFromString(v)
		if err != nil {
			return 0, "", status.Errorf(codes.Internal, "failed to parse owner id tag on volume %s: %v", vol, err)
		}
	}
	nodeName, _ := tags[ownerNodeTagKey]
	return nodeID, nodeName, nil
}

// Update the owner tag of a volume, removing any stale owner tag and replacing
// them with a new ownerID entry (if present).
//
// Calling this function with a nil ownerID will cause it to remove all owner tags.
func (vl *volumeLock) setOwner(ctx context.Context, ref VolumeRef, ownerID *uint16, ownerNode *string) error {
	// Fetch the volume and get its tags
	vol, err := vl.fetch(ctx, &ref)
	if err != nil {
		return err
	}
	tags, err := vol.Tags()
	if err != nil {
		return err
	}

	// Decide which tags to add/remove
	var addTags, delTags []string
	pOwnerID, ok := tags[ownerIdTagKey]
	if ok && (ownerID == nil || pOwnerID != nodeIDToString(*ownerID)) {
		delTags = append(delTags, encodeTagKV(ownerIdTagKey, pOwnerID))
	}
	pOwnerNode, ok := tags[ownerNodeTagKey]
	if ok && (ownerNode == nil || pOwnerNode != *ownerNode) {
		delTags = append(delTags, encodeTagKV(ownerNodeTagKey, pOwnerNode))
	}
	if ownerID != nil {
		addTags = append(addTags, encodeTagKV(ownerIdTagKey, nodeIDToString(*ownerID)))
	}
	if ownerNode != nil {
		addTags = append(addTags, encodeTagKV(ownerNodeTagKey, *ownerNode))
	}

	// Update volume tags
	if len(addTags) == 0 && len(delTags) == 0 {
		return nil
	}
	_, err = vl.lvmctrld.LvChange(ctx, &pb.LvChangeRequest{
		Target: []string{vol.VgLv()},
		AddTag: addTags,
		DelTag: delTags,
	})
	return err
}

func (vl *volumeLock) sync(ctx context.Context) error {
	vl.mutex.Lock()
	defer vl.mutex.Unlock()

	klog.Infof("Syncing LVM status")

	// Find all active volumes.
	lvs, err := vl.lvmctrld.Lvs(ctx, &pb.LvsRequest{
		Select: fmt.Sprintf("lv_name=~^%s && lv_active=active", volumeLvPrefix),
	})
	if err != nil {
		return fmt.Errorf("failed to list logical volumes: %v", err)
	}

	for _, lv := range lvs.Lvs {
		vol := NewVolumeRefFromLv(*lv)
		// Try to deactivate volume if it is not opened
		if lv.LvDeviceOpen != pb.LvDeviceOpen_LV_DEVICE_OPEN_OPEN {
			_, err = vl.lvmctrld.LvChange(ctx, &pb.LvChangeRequest{
				Target:   []string{vol.VgLv()},
				Activate: pb.LvActivationMode_LV_ACTIVATION_MODE_DEACTIVATE,
			})
			if err != nil {
				klog.Warningf("Failed to unlock volume %q: %v", *vol, err)
			}
			// Try to remove owner tag. We could race here and have another host lock the
			// volume. In such case, the update will fail with permission denied.
			err := vl.setOwner(ctx, *vol, nil, nil)
			if err != nil && status.Code(err) != codes.PermissionDenied {
				klog.Warningf("failed to update tags on volume %q: %v", *vol, err)
			}
			continue
		}
		// The volume is not locked, take note
		vl.locks[vol.VgLv()] = map[string]struct{}{defaultLockOp: {}}
	}
	return nil
}
