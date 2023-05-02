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
	"strconv"

	pb "github.com/aleofreddi/csi-sanlock-lvm/pkg/proto"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const topologyKeyNode = "csi-sanlock-lvm/topology"

var nodeCapabilities = map[csi.NodeServiceCapability_RPC_Type]struct{}{
	csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME: {},
	csi.NodeServiceCapability_RPC_EXPAND_VOLUME:        {},
	csi.NodeServiceCapability_RPC_VOLUME_MOUNT_GROUP:   {},
}

type nodeServer struct {
	nodeID     uint16
	lvmctrld   pb.LvmCtrldClient
	volumeLock VolumeLocker
	fsRegistry FileSystemRegistry
}

func NewNodeServer(lvmctrld pb.LvmCtrldClient, volumeLock VolumeLocker, fsRegistry FileSystemRegistry) (*nodeServer, error) {
	st, err := lvmctrld.GetStatus(context.Background(), &pb.GetStatusRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve status from lvmctrld: %v", err)
	}
	ns := &nodeServer{
		nodeID:     uint16(st.NodeId),
		lvmctrld:   lvmctrld,
		volumeLock: volumeLock,
		fsRegistry: fsRegistry,
	}
	return ns, nil
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	id := strconv.Itoa(int(ns.nodeID))
	topology := &csi.Topology{
		Segments: map[string]string{topologyKeyNode: id},
	}
	return &csi.NodeGetInfoResponse{
		NodeId:             id,
		AccessibleTopology: topology,
	}, nil
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	nsCpbs := make([]*csi.NodeServiceCapability, 0, len(nodeCapabilities))
	for cpb, _ := range nodeCapabilities {
		nsCpbs = append(nsCpbs, &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cpb,
				},
			},
		})
	}
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: nsCpbs,
	}, nil
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// Check arguments.
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "missing volume capability")
	}
	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing target path")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing staging target path")
	}
	accessType, err := getAccessType(req.GetVolumeCapability())
	if err != nil {
		return nil, err
	}
	vol, err := NewVolumeRefFromID(req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume id: %v", err)
	}

	// Retrieve the filesystem type for the volume.
	lv, err := lvsVolume(ctx, ns.lvmctrld, *vol)
	if err != nil {
		return nil, err
	}
	tags, err := decodeTags(lv.LvTags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to decode tags for volume %q: %v", vol, err)
	}

	// Extract the filesystem.
	fsName, ok := tags[fsTagKey]
	if !ok {
		return nil, status.Errorf(codes.Internal, "volume %q is missing filesystem type", vol)
	}
	fs, err := ns.fsRegistry.GetFileSystem(fsName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "volume %q has an invalid filesystem: %v", vol, err)
	}
	if !fs.Accepts(accessType) {
		return nil, status.Error(codes.InvalidArgument, "incompatible access type for this volume")
	}

	// Mount.
	err = fs.Publish(vol.DevPath(), req.GetStagingTargetPath(), req.GetTargetPath(), req.GetReadonly())
	if err != nil {
		return nil, err
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	// Check arguments.
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing target path")
	}
	vol, err := NewVolumeRefFromID(req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume id: %v", err)
	}

	// Retrieve the filesystem type for the volume.
	lv, err := lvsVolume(ctx, ns.lvmctrld, *vol)
	if err != nil {
		return nil, err
	}
	tags, err := decodeTags(lv.LvTags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to decode tags for volume %q: %v", vol, err)
	}

	// Extract the filesystem.
	fsName, ok := tags[fsTagKey]
	if !ok {
		return nil, status.Errorf(codes.Internal, "volume %q is missing filesystem type", vol)
	}
	fs, err := ns.fsRegistry.GetFileSystem(fsName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "volume %q has an invalid filesystem: %v", vol, err)
	}

	// Unmount.
	err = fs.Unpublish(req.GetTargetPath())
	if err != nil {
		return nil, err
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	// Check arguments.
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "missing volume capability")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing staging target path")
	}
	accessType, err := getAccessType(req.GetVolumeCapability())
	if err != nil {
		return nil, err
	}
	var volumeGid *int
	if accessType == MountAccessType {
		if volumeMountGroup := req.GetVolumeCapability().GetMount().GetVolumeMountGroup(); volumeMountGroup != "" {
			val, err := strconv.Atoi(volumeMountGroup)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "invalid volume mount group %q: %v", volumeMountGroup, err)
			}
			volumeGid = &val
		}
	}
	vol, err := NewVolumeRefFromID(req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume id: %v", err)
	}

	// Use shared mode
	shared := false
	if req.VolumeCapability.AccessMode.Mode == csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER ||
		req.VolumeCapability.AccessMode.Mode == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY ||
		req.VolumeCapability.AccessMode.Mode == csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER {
		shared = true
	}

	// Lock the volume.
	err = ns.volumeLock.LockVolume(ctx, *vol, shared, defaultLockOp)
	if err != nil {
		return nil, err
	}

	// Retrieve the filesystem type for the volume.
	lv, err := lvsVolume(ctx, ns.lvmctrld, *vol)
	if err != nil {
		return nil, err
	}
	tags, err := decodeTags(lv.LvTags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to decode tags for volume %q: %v", vol, err)
	}

	// Extract the filesystem.
	fsName, ok := tags[fsTagKey]
	if !ok {
		return nil, status.Errorf(codes.Internal, "volume %q is missing filesystem type", vol)
	}
	fs, err := ns.fsRegistry.GetFileSystem(fsName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "volume %q has an invalid filesystem: %v", vol, err)
	}
	if !fs.Accepts(accessType) {
		return nil, status.Error(codes.InvalidArgument, "incompatible access type for this volume")
	}
	mountFlags := make([]string, 0)
	if accessType == MountAccessType {
		mountFlags = append(mountFlags, req.GetVolumeCapability().GetMount().GetMountFlags()...)
	}

	// Mount.
	if err = fs.Stage(vol.DevPath(), req.GetStagingTargetPath(), mountFlags, volumeGid); err != nil {
		return nil, err
	}
	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	// Check arguments.
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing staging target path")
	}

	vol, err := NewVolumeRefFromID(req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume id: %v", err)
	}

	// Retrieve the filesystem type for the volume.
	lv, err := lvsVolume(ctx, ns.lvmctrld, *vol)
	if err != nil {
		return nil, err
	}
	tags, err := decodeTags(lv.LvTags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to decode tags for volume %q: %v", vol, err)
	}

	// Extract the filesystem.
	fsName, ok := tags[fsTagKey]
	if !ok {
		return nil, status.Errorf(codes.Internal, "volume %q is missing filesystem type", vol)
	}
	fs, err := ns.fsRegistry.GetFileSystem(fsName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "volume %q has an invalid filesystem: %v", vol, err)
	}

	// Unmount.
	err = fs.Unstage(req.GetStagingTargetPath())
	if err != nil {
		return nil, err
	}

	// Lock the volume.
	err = ns.volumeLock.UnlockVolume(ctx, *vol, defaultLockOp)
	if err != nil {
		return nil, err
	}
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, in *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	// Validate arguments
	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if !volumeIdRe.MatchString(volumeId) {
		return nil, status.Error(codes.NotFound, "invalid volume id")
	}
	if req.VolumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume path")
	}

	vol, err := NewVolumeRefFromID(req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume id: %v", err)
	}

	// Retrieve the filesystem type for the volume
	lv, err := lvsVolume(ctx, ns.lvmctrld, *vol)
	if err != nil {
		return nil, err
	}
	tags, err := decodeTags(lv.LvTags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to decode tags for volume %q: %v", vol, err)
	}

	// Extract the filesystem
	fsName, ok := tags[fsTagKey]
	if !ok {
		return nil, status.Errorf(codes.Internal, "volume %q is missing filesystem type", vol)
	}
	fs, err := ns.fsRegistry.GetFileSystem(fsName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "volume %q has an invalid filesystem: %v", vol, err)
	}
	// Issue the resize if the logical volume is smaller than required bytes
	requiredBytes := uint64(req.CapacityRange.RequiredBytes)
	if lv.LvSize < requiredBytes {

		// Shared volumes have no owner tag
		if _, ownerTag := tags[ownerNodeTagKey]; !ownerTag {
			defer func() {
				// Re-lock the volume into shared mode.
				_, _ = ns.lvmctrld.LvChange(ctx, &pb.LvChangeRequest{
					Target:   []string{vol.VgLv()},
					Activate: pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_SHARED,
				})
			}()

			// Re-lock the volume in exclusive mode.
			_, err = ns.lvmctrld.LvChange(ctx, &pb.LvChangeRequest{
				Target:   []string{vol.VgLv()},
				Activate: pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE,
			})
			if err != nil {
				return nil, status.Errorf(status.Code(err), "failed to lock volume %s: %v", vol, err)
			}
		}

		_, err = ns.lvmctrld.LvResize(ctx, &pb.LvResizeRequest{VgName: vol.Vg(), LvName: vol.Lv(), Size: requiredBytes})
		if status.Code(err) == codes.OutOfRange {
			return nil, status.Errorf(codes.OutOfRange, "insufficient free space")
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to resize volume %q: %v", vol.ID(), err)
		}
	}

	// Resize the filesystem
	err = fs.Grow(vol.DevPath())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resize filesystem: %s", err.Error())
	}

	return &csi.NodeExpandVolumeResponse{}, nil
}

func getAccessType(volCap *csi.VolumeCapability) (VolumeAccessType, error) {
	if (volCap.GetBlock() == nil) == (volCap.GetMount() == nil) {
		return 0, status.Error(codes.InvalidArgument, "inconsistent access type")
	}
	if volCap.GetMount() != nil {
		return MountAccessType, nil
	}
	return BlockAccessType, nil
}

func lvsVolume(ctx context.Context, client pb.LvmCtrldClient, vol VolumeRef) (*pb.LogicalVolume, error) {
	lvs, err := client.Lvs(ctx, &pb.LvsRequest{
		Target: []string{vol.VgLv()},
	})
	if status.Code(err) == codes.NotFound {
		return nil, status.Errorf(codes.NotFound, "volume %v not found", vol)
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list volumes: %v", err)
	}
	return lvs.Lvs[0], nil
}
