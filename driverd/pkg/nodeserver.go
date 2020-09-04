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
	"strings"

	"github.com/aleofreddi/csi-sanlock-lvm/lvmctrld/proto"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const topologyKeyNode = "csi-sanlock-lvm/topology"

var nodeCapabilities = map[csi.NodeServiceCapability_RPC_Type]struct{}{
	csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME: {},
	csi.NodeServiceCapability_RPC_EXPAND_VOLUME:        {},
}

type nodeServer struct {
	nodeId                string
	lvmctrldAddr          string
	lvmctrldClientFactory LvmCtrldClientFactory
	newFileSystem         FileSystemFactory
}

func NewNodeServer(nodeId, lvmctrldAddr string, factory LvmCtrldClientFactory, newFileSystem FileSystemFactory) (*nodeServer, error) {
	return &nodeServer{
		nodeId:                nodeId,
		lvmctrldAddr:          lvmctrldAddr,
		lvmctrldClientFactory: factory,
		newFileSystem:         newFileSystem,
	}, nil
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	topology := &csi.Topology{
		Segments: map[string]string{topologyKeyNode: ns.nodeId},
	}
	return &csi.NodeGetInfoResponse{
		NodeId:             ns.nodeId,
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
	// Check arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "missing volume capability")
	}
	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing target path")
	}
	if (req.GetVolumeCapability().GetBlock() == nil) == (req.GetVolumeCapability().GetMount() == nil) {
		return nil, status.Error(codes.InvalidArgument, "inconsistent access type")
	}

	volumeId := req.GetVolumeId()
	devicePath := fmt.Sprintf("/dev/%s", volumeId)
	targetPath := req.GetTargetPath()

	// Decode access type from request
	var accessType VolumeAccessType
	if req.GetVolumeCapability().GetMount() != nil {
		accessType = MountAccessType
	} else {
		accessType = BlockAccessType
	}

	// Connect to lvmctrld
	client, err := ns.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	// Retrieve the filesystem type for the volume
	lv, err := findLogicalVolume(ctx, client, volumeId)
	if err != nil {
		return nil, err
	}

	// Extract the filesystem
	fsName, err := getFileSystemName(lv)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	fs, err := ns.newFileSystem(fsName)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !fs.Accepts(accessType) {
		return nil, status.Error(codes.InvalidArgument, "incompatible access type for this volume")
	}

	mountFlags := make([]string, 0)
	if accessType == MountAccessType {
		mountFlags = append(mountFlags, req.GetVolumeCapability().GetMount().GetMountFlags()...)
		if req.GetReadonly() {
			mountFlags = append(mountFlags, "ro")
		} else {
			mountFlags = append(mountFlags, "rw")
		}
	}

	if err = fs.Mount(devicePath, targetPath, mountFlags); err != nil {
		return nil, err
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	// Check arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing target path")
	}

	volumeId := req.GetVolumeId()

	// Connect to lvmctrld
	client, err := ns.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	// Retrieve the filesystem type for the volume
	lv, err := findLogicalVolume(ctx, client, volumeId)
	if err != nil {
		return nil, err
	}

	// Extract the filesystem
	fsName, err := getFileSystemName(lv)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	fs, err := ns.newFileSystem(fsName)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Unmount
	err = fs.Umount(req.GetTargetPath())
	if err != nil {
		return nil, err
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	// Check arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "missing volume capability")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing staging target path")
	}

	volumeId := req.GetVolumeId()

	// Connect to lvmctrld
	client, err := ns.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	// Activate volume
	_, err = client.LvChange(ctx, &proto.LvChangeRequest{
		Target:   volumeId,
		Activate: proto.LvActivationMode_ACTIVE_EXCLUSIVE,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to activate logical volume: %s", err.Error())
	}

	// Retrieve logical volume
	lv, err := findLogicalVolume(ctx, client, volumeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list logical volume: %s", err.Error())
	}

	// Collect any stale owner tag
	expectedOwnerTag := encodeTag(getOwnerTag(ns.nodeId, ns.lvmctrldAddr))
	var delTags []string
	for _, encodedTag := range lv.LvTags {
		if encodedTag == expectedOwnerTag {
			continue
		}
		decodedTag, _ := decodeTag(encodedTag)
		if strings.HasPrefix(decodedTag, ownerTag) {
			delTags = append(delTags, encodedTag)
		}
	}

	// Add owner tag and remove stale owner tags if any
	_, err = client.LvChange(ctx, &proto.LvChangeRequest{
		Target: volumeId,
		AddTag: []string{expectedOwnerTag},
		DelTag: delTags,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update tags: %s", err.Error())
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	// Check arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing staging target path")
	}

	volumeId := req.GetVolumeId()

	// Connect to lvmctrld
	client, err := ns.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	// Deactivate volume
	_, err = client.LvChange(ctx, &proto.LvChangeRequest{
		Target:   volumeId,
		Activate: proto.LvActivationMode_DEACTIVATE,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to deactivate logical volume: %s", err.Error())
	}

	// Remove owner tag
	ownerTag := encodeTag(getOwnerTag(ns.nodeId, ns.lvmctrldAddr))
	_, err = client.LvChange(ctx, &proto.LvChangeRequest{
		Target: volumeId,
		DelTag: []string{ownerTag},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove tag: %v", err)
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
	if req.VolumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume path")
	}

	// Connect to lvmctrld
	client, err := ns.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %v", err)
	}
	defer client.Close()

	// Retrieve the filesystem type for the volume
	lv, err := findLogicalVolume(ctx, client, volumeId)
	if err != nil {
		return nil, err
	}

	// Extract the filesystem
	fsName, err := getFileSystemName(lv)
	if err != nil {
		return nil, err
	}
	fs, err := ns.newFileSystem(fsName)
	if err != nil {
		return nil, err
	}

	// Resize the filesystem
	err = fs.Grow("/dev/" + volumeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resize filesystem: %s", err.Error())
	}

	return &csi.NodeExpandVolumeResponse{}, nil
}

func findLogicalVolume(ctx context.Context, client *LvmCtrldClientConnection, volumeId string) (*proto.LogicalVolume, error) {
	lvs, err := client.Lvs(ctx, &proto.LvsRequest{
		Select: "lv_role!=snapshot",
		Target: volumeId,
	})
	if err != nil && status.Code(err) == codes.NotFound || lvs != nil && len(lvs.Lvs) != 1 {
		return nil, status.Errorf(codes.NotFound, "volume not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list volumes: %s", err.Error())
	}
	return lvs.Lvs[0], nil
}

func getFileSystemName(lv *proto.LogicalVolume) (string, error) {
	fsName := ""
	for _, encodedTag := range lv.LvTags {
		decodedTag, _ := decodeTag(encodedTag)
		if strings.HasPrefix(decodedTag, fsTag) {
			if len(fsName) > 0 {
				return "", fmt.Errorf("volume %s/%s has multiple filesystem tags", lv.VgName, lv.LvName)
			}
			fsName = decodedTag[len(fsTag):]
		}
	}
	if fsName == "" {
		return "", fmt.Errorf("volume %s/%s is missing filesystem tags", lv.VgName, lv.LvName)
	}
	return fsName, nil
}
