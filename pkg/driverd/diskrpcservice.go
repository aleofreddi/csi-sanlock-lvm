// Copyright 2021 Google LLC
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
	"time"

	diskrpc "github.com/aleofreddi/csi-sanlock-lvm/pkg/diskrpc"
	pb "github.com/aleofreddi/csi-sanlock-lvm/pkg/proto"
	"k8s.io/klog"
)

type DiskRpcService interface {
	diskrpc.DiskRpc

	// Start the DiskRPC service.
	Start(ctx context.Context) error
}

// DiskRpcService adapts DiskRpc to use volume locking.
type diskRpcService struct {
	baseServer
	diskrpc.DiskRpc
	mailBox diskrpc.MailBox
}

type RpcRole string

const (
	rpcRoleLock RpcRole = "lock"
	rpcRoleData RpcRole = "data"

	diskRpcOp = "diskRpc"

	maxRpcRetryDelay = 120 * time.Second
)

type volumeLockerAdapter struct {
	locker VolumeLocker
	ctx    context.Context
	vol    VolumeRef
}

func (vl *volumeLockerAdapter) Lock() {
	for delay := 1 * time.Second; ; {
		err := vl.locker.LockVolume(vl.ctx, vl.vol, diskRpcOp)
		if err == nil {
			return
		}
		klog.Warningf("Failed to acquire RPC lock (retry in %s): %v", delay, err)
		time.Sleep(delay)
		if delay < maxRpcRetryDelay {
			delay *= 2
		}
	}
}

func (vl *volumeLockerAdapter) Unlock() {
	for delay := time.Duration(1); ; {
		err := vl.locker.UnlockVolume(vl.ctx, vl.vol, diskRpcOp)
		if err == nil {
			return
		}
		klog.Warningf("Failed to release RPC lock (retry in %s): %v", delay, err)
		time.Sleep(delay)
		if delay < maxRpcRetryDelay {
			delay *= 2
		}
	}
}

func NewDiskRpcService(lvmctrld pb.LvmCtrldClient, locker VolumeLocker) (DiskRpcService, error) {
	bs, err := newBaseServer(lvmctrld)
	if err != nil {
		return nil, err
	}
	// Find rpc lock and data logical volumes.
	ctx := context.Background()
	lvs, err := lvmctrld.Lvs(ctx, &pb.LvsRequest{
		Select: fmt.Sprintf("lv_tags=%s || lv_tags=%s",
			encodeTagKV(rpcRoleTagKey, string(rpcRoleData)),
			encodeTagKV(rpcRoleTagKey, string(rpcRoleLock))),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list logical volumes: %v", err)
	}
	var data *VolumeInfo
	var lock *VolumeInfo
	for _, lv := range lvs.Lvs {
		lvRef := NewVolumeInfoFromLv(lv)
		tags, err := lvRef.Tags()
		if err != nil {
			return nil, fmt.Errorf("failed to decode tags for volume %q: %v", lvRef, err)
		}
		role := RpcRole(tags[rpcRoleTagKey])
		if role == rpcRoleLock {
			if lock != nil {
				return nil, fmt.Errorf("found multiple logical volumes tagged with the %s RPC role", rpcRoleLock)
			}
			lock = lvRef
		} else if role == rpcRoleData {
			if data != nil {
				return nil, fmt.Errorf("found multiple logical volumes tagged with the %s RPC role", rpcRoleData)
			}
			data = lvRef
		}
	}
	if lock == nil || data == nil {
		return nil, fmt.Errorf("missing lock or data logical volumes (expected [%s%s, %s%s] tags)",
			encodeTagKeyPrefix(rpcRoleTagKey), rpcRoleData,
			encodeTagKeyPrefix(rpcRoleTagKey), rpcRoleLock)
	}
	// Activate data as shared.
	_, err = lvmctrld.LvChange(ctx, &pb.LvChangeRequest{
		Target:   []string{data.VgLv()},
		Activate: pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_SHARED,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to activate rpc data logical volume: %v", err)
	}
	// Instance mailbox.
	mb, err := diskrpc.NewMailBox(
		diskrpc.MailBoxID(bs.nodeID),
		&volumeLockerAdapter{
			ctx:    ctx,
			locker: locker,
			vol:    lock.VolumeRef,
		},
		data.DevPath(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to instance mailbox: %v", err)
	}
	dRpc, err := diskrpc.NewDiskRpc(mb)
	if err != nil {
		return nil, fmt.Errorf("failed to instance diskrpc: %v", err)
	}
	return &diskRpcService{
		baseServer: *bs,
		DiskRpc:    dRpc,
		mailBox:    mb,
	}, nil
}

func (s *diskRpcService) Start(ctx context.Context) error {
	go func() {
		for {
			err := s.Handle(ctx)
			if err != nil {
				klog.Errorf("DiskRpc failed to handle messages: %v", err)
			}
			select {
			//case <-done: return // FIXME!
			case <-time.After(1 * time.Second):
			}
		}
	}()
	return nil
}
