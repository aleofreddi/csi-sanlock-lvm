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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/aleofreddi/csi-sanlock-lvm/lvmctrld/proto"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"regexp"
	"strings"
)

const (
	// Prefix to be used for volume logical volumes
	volumeLvPrefix = "csi-v-"

	// Prefix to be used for snapshot logical volumes
	snapshotLvPrefix = "csi-s-"

	vgParamKey = "volumeGroup"
	fsParamKey = "filesystem"
)

var (
	vgRe = regexp.MustCompile("^[a-zA-Z0-9+_.][a-zA-Z0-9+_.-]*$")
)

type volumeAccessType int

const (
	MOUNT_ACCESS_TYPE volumeAccessType = iota
	BLOCK_ACCESS_TYPE
)

var controllerCapabilities = map[csi.ControllerServiceCapability_RPC_Type]struct{}{
	csi.ControllerServiceCapability_RPC_CLONE_VOLUME:           {},
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT: {},
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:   {},
	csi.ControllerServiceCapability_RPC_EXPAND_VOLUME:          {},
	csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS:         {},
}

type controllerServer struct {
	nodeId                string
	lvmctrldAddr          string
	lvmctrldClientFactory LvmCtrldClientFactory
}

func NewControllerServer(nodeId string, lvmctrldAddr string, factory LvmCtrldClientFactory) (*controllerServer, error) {
	return &controllerServer{
		nodeId:                nodeId,
		lvmctrldAddr:          lvmctrldAddr,
		lvmctrldClientFactory: factory,
	}, nil
}

func (cs *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	ctrlCpbs := make([]*csi.ControllerServiceCapability, 0, len(controllerCapabilities))
	for cpb, _ := range controllerCapabilities {
		ctrlCpbs = append(ctrlCpbs, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cpb,
				},
			},
		})
	}
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: ctrlCpbs,
	}, nil
}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// Check arguments
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume name")
	}
	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing volume capabilities")
	}

	// Parse capabilities
	var accessMode *csi.VolumeCapability_AccessMode_Mode
	var accessType *volumeAccessType
	for _, cap := range req.GetVolumeCapabilities() {
		var capAccessMode *csi.VolumeCapability_AccessMode_Mode
		if cap.GetAccessMode() != nil {
			v := cap.GetAccessMode().GetMode()
			capAccessMode = &v
		}
		if capAccessMode != nil {
			if accessMode != nil && *capAccessMode != *accessMode {
				return nil, status.Errorf(codes.InvalidArgument, "inconsistent access mode: both %s and %s specified", *capAccessMode, *accessMode)
			} else {
				accessMode = capAccessMode
			}
		}

		var capAccessType *volumeAccessType
		if cap.GetMount() != nil {
			v := MOUNT_ACCESS_TYPE
			capAccessType = &v
		} else if cap.GetBlock() != nil {
			v := BLOCK_ACCESS_TYPE
			capAccessType = &v
		}
		if capAccessType != nil {
			if accessType != nil && *capAccessType != *accessType {
				return nil, status.Error(codes.InvalidArgument, "inconsistent access type")
			} else {
				accessType = capAccessType
			}
		}
	}
	if accessType == nil {
		return nil, status.Error(codes.InvalidArgument, "missing access type")
	}
	if accessMode == nil {
		return nil, status.Error(codes.InvalidArgument, "missing access mode")
	}
	if *accessMode != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER && *accessMode != csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported access mode %s", *accessMode)
	}

	// Parse parameters
	vgName, present := req.Parameters[vgParamKey]
	if !present || vgName == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume group parameter")
	}
	if !vgRe.MatchString(vgName) {
		return nil, status.Error(codes.InvalidArgument, "invalid volume group parameter")
	}
	fsName, present := req.Parameters[fsParamKey]
	if !present || fsName == "" {
		return nil, status.Error(codes.InvalidArgument, "missing filesystem parameter")
	}
	fs, err := NewFileSystem(fsName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lookup filesystem: %s", err.Error())
	}

	// Connect to lvmctrld
	client, err := cs.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	// Create volume
	lvName := volumeNameToLvName(req.GetName())
	volumeId := fmt.Sprintf("%s/%s", vgName, lvName)
	_, err = client.LvCreate(
		ctx,
		&proto.LvCreateRequest{
			VgName:   vgName,
			LvName:   lvName,
			Activate: proto.LvActivationMode_ACTIVE_EXCLUSIVE,
			Size:     uint64(req.GetCapacityRange().GetRequiredBytes()),
			LvTags: []string{
				encodeTag(nameTag + req.GetName()),
				encodeTag(getOwnerTag(cs.nodeId, cs.lvmctrldAddr)),
				encodeTag(getTransientTag(cs.nodeId)),
				encodeTag(fsTag + fsName),
			},
		},
	)
	if err != nil {
		if status.Code(err) == codes.OutOfRange {
			return nil, status.Errorf(codes.OutOfRange, "insufficient free space")
		}
		if status.Code(err) == codes.AlreadyExists {
			lvs, err := client.Lvs(ctx, &proto.LvsRequest{
				Select: "lv_role!=snapshot",
				Target: volumeId,
			})
			if err != nil || len(lvs.Lvs) != 1 {
				return nil, status.Errorf(codes.Internal, "failed to list volumes")
			}
			existing := lvs.Lvs[0]
			if uint64(req.GetCapacityRange().GetRequiredBytes()) > existing.LvSize {
				return nil, status.Errorf(codes.AlreadyExists, "volume with the same name %s but with different size already exist", req.GetName())
			}
			return &csi.CreateVolumeResponse{
				Volume: lvToVolume(existing),
			}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to create volume %s: %s", req.GetName(), err.Error())
	}

	// If creation was successful, format the volume
	klog.Infof("Formatting volume")
	err = fs.Make("/dev/" + volumeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to format volume: %s", err.Error())
	}

	// Deactivate the volume
	_, err = client.LvChange(ctx, &proto.LvChangeRequest{
		Target:   volumeId,
		Activate: proto.LvActivationMode_DEACTIVATE,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deactivate volume: %s", err.Error())
	}

	// Remove transient and owner tag
	_, err = client.LvChange(ctx, &proto.LvChangeRequest{
		Target: volumeId,
		DelTag: []string{
			encodeTag(getTransientTag(cs.nodeId)),
			encodeTag(getOwnerTag(cs.nodeId, cs.lvmctrldAddr)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to remove tags: %s", err.Error())
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeId,
			CapacityBytes: req.GetCapacityRange().GetRequiredBytes(),
			VolumeContext: req.GetParameters(),
			ContentSource: req.GetVolumeContentSource(),
			//AccessibleTopology: (use vgname here)
		},
	}, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// Check arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}

	// Connect to lvmctrld
	client, err := cs.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	volumeId := req.GetVolumeId()
	vgName, lvName := volumeIdToVgLv(volumeId)

	// Try to remove the volume. If it's an origin for some snapshot, it will be skipped
	_, err = client.LvRemove(ctx, &proto.LvRemoveRequest{
		Select: "lv_role!=origin",
		VgName: vgName,
		LvName: lvName,
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, status.Errorf(codes.Internal, "failed to delete volume %s: %s", volumeId, err.Error())
	}

	// Check if the volume still exists. It could be that lvremove didn't fail but didn't match the volume either (because it is a snapshot origin)
	if err == nil {
		_, err = client.Lvs(ctx, &proto.LvsRequest{
			Select: "lv_role!=snapshot",
			Target: req.GetVolumeId(),
		})
		if err == nil {
			return nil, status.Error(codes.FailedPrecondition, "failed to delete volume because of dependant snapshot")
		}
		if status.Code(err) != codes.NotFound {
			return nil, status.Errorf(codes.Internal, "failed to list volumes")
		}
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	// Check arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if len(req.VolumeCapabilities) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "missing volume capabilities")
	}

	// Connect to lvmctrld
	client, err := cs.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	volume, err := client.Lvs(ctx, &proto.LvsRequest{
		Select: "lv_role!=snapshot",
		Target: req.GetVolumeId(),
	})
	if err != nil {
		return nil, status.Error(codes.NotFound, req.GetVolumeId())
	}

	//for _, cap := range req.GetVolumeCapabilities() {
	// FIXME - to implement!
	//}
	klog.Infof("TO IMPLEMENT %s", volume)

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeContext:      req.GetVolumeContext(),
			VolumeCapabilities: req.GetVolumeCapabilities(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

func (cs *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	filters := make([]string, 0)
	vgName, vgFilter := req.Parameters[vgParamKey]
	if vgFilter {
		if !vgRe.MatchString(vgName) {
			return nil, status.Error(codes.InvalidArgument, "invalid volume group name")
		}
		filters = append(filters, "vg_name="+vgName)
	}

	// Connect to lvmctrld
	client, err := cs.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	vgs, err := client.Vgs(ctx, &proto.VgsRequest{
		Select: strings.Join(filters, " && "),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list volumes")
	}

	var free int64
	for _, vg := range vgs.Vgs {
		free += int64(vg.VgFree)
	}

	return &csi.GetCapacityResponse{
		AvailableCapacity: free,
	}, nil
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if req.GetCapacityRange() == nil {
		return nil, status.Error(codes.InvalidArgument, "missing capacity range")
	}

	// Connect to lvmctrld
	client, err := cs.lvmctrldClientFactory.NewForVolume(req.GetVolumeId(), ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	volumeId := req.GetVolumeId()
	vgName, lvName := volumeIdToVgLv(volumeId)

	// Retrieve current size
	lvs, err := client.Lvs(ctx, &proto.LvsRequest{
		Select: "lv_role!=snapshot",
		Target: volumeId,
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, status.Errorf(codes.Internal, "failed to list volumes")
	}
	if err != nil && status.Code(err) == codes.NotFound || len(lvs.Lvs) != 1 {
		return nil, status.Error(codes.NotFound, "logical volume not found")
	}

	lv := lvs.Lvs[0]
	requiredBytes := uint64(req.CapacityRange.RequiredBytes)
	if lv.LvSize >= requiredBytes {
		// Logical volume size is already greater or equal to the one requested. However, we issue a NodeExpansionRequired
		// to ensure that the node resize is processed if CO looses state.
		return &csi.ControllerExpandVolumeResponse{CapacityBytes: int64(lv.LvSize), NodeExpansionRequired: true}, nil
	}

	// Issue the resize
	_, err = client.LvResize(ctx, &proto.LvResizeRequest{VgName: vgName, LvName: lvName, Size: requiredBytes})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resize volume: %s", err.Error())
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         int64(requiredBytes),
		NodeExpansionRequired: true,
	}, nil
}

func (cs *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	// Connect to lvmctrld
	client, err := cs.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	volumes, err := client.Lvs(ctx, &proto.LvsRequest{
		Select: "lv_role!=snapshot",
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list volumes")
	}

	entries := make([]*csi.ListVolumesResponse_Entry, len(volumes.Lvs))
	for i, lv := range volumes.Lvs {
		entries[i] = &csi.ListVolumesResponse_Entry{Volume: lvToVolume(lv)}
	}

	return &csi.ListVolumesResponse{
		Entries: entries,
	}, nil
}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	// Check arguments
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing snapshot name")
	}
	if req.GetSourceVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing source volume id")
	}

	// Connect to lvmctrld
	client, err := cs.lvmctrldClientFactory.NewForVolume(req.GetSourceVolumeId(), ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	// Create snapshot
	originId := req.GetSourceVolumeId()
	vgName, origLvName := volumeIdToVgLv(originId)
	lvName := snapshotNameToLvName(req.GetName())
	volumeId := fmt.Sprintf("%s/%s", vgName, lvName)
	_, err = client.LvCreate(
		ctx,
		&proto.LvCreateRequest{
			VgName:   vgName,
			LvName:   lvName,
			Activate: proto.LvActivationMode_DEACTIVATE,
			Size:     uint64(20 * (1 << 20)), // FIXME
			Origin:   origLvName,
			LvTags: []string{
				encodeTag(nameTag + req.GetName()),
				encodeTag(getOwnerTag(cs.nodeId, cs.lvmctrldAddr)),
			},
		},
	)
	if status.Code(err) == codes.OutOfRange {
		return nil, status.Errorf(codes.OutOfRange, "insufficient free space")
	}
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return nil, status.Errorf(codes.Internal, "failed to create snapshot %s: %s", req.GetName(), err.Error())
	}

	// Read back the snapshot and return it
	lvs, err := client.Lvs(ctx, &proto.LvsRequest{
		Select: "lv_role=snapshot",
		Target: volumeId,
	})
	if err != nil || len(lvs.Lvs) != 1 {
		return nil, status.Errorf(codes.Internal, "failed to list volumes")
	}
	existing := lvs.Lvs[0]
	if origLvName != existing.Origin {
		return nil, status.Errorf(codes.AlreadyExists, "snapshot with the same name: %s but with different SourceVolumeId already exist", req.GetName())
	}
	return &csi.CreateSnapshotResponse{Snapshot: lvToSnapshot(lvs.Lvs[0])}, nil
}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	// Check arguments
	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing snapshot id")
	}

	// Connect to lvmctrld
	client, err := cs.lvmctrldClientFactory.NewForVolume(req.GetSnapshotId(), ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	volumeId := req.GetSnapshotId()
	vgName, lvName := volumeIdToVgLv(volumeId)

	// Remove volume
	_, err = client.LvRemove(ctx, &proto.LvRemoveRequest{
		VgName: vgName,
		LvName: lvName,
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, status.Errorf(codes.Internal, "failed to delete snapshot %s: %s", volumeId, err.Error())
	}

	return &csi.DeleteSnapshotResponse{}, nil
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	filters := []string{"lv_role=snapshot"}
	if req.GetSnapshotId() != "" {
		vgName, lvName := volumeIdToVgLv(req.GetSnapshotId())
		filters = append(filters, "vg_name="+vgName, "lv_name="+lvName)
	}
	if req.GetSourceVolumeId() != "" {
		vgName, lvName := volumeIdToVgLv(req.GetSourceVolumeId())
		filters = append(filters, "vg_name="+vgName, "origin="+lvName)
	}

	// FIXME - pagination is a hard requirement as per spec!
	// see https://github.com/container-storage-interface/spec/blob/master/spec.md#listsnapshots

	// Connect to lvmctrld
	client, err := cs.lvmctrldClientFactory.NewLocal()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to connect to lvmctrld: %s", err.Error())
	}
	defer client.Close()

	// List snapshots
	volumes, err := client.Lvs(ctx, &proto.LvsRequest{
		Select: strings.Join(filters, " && "),
	})
	entries := make([]*csi.ListSnapshotsResponse_Entry, len(volumes.Lvs))
	for i, lv := range volumes.Lvs {
		entries[i] = &csi.ListSnapshotsResponse_Entry{Snapshot: lvToSnapshot(lv)}
	}

	return &csi.ListSnapshotsResponse{
		Entries: entries,
	}, nil
}

func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func volumeIdToVgLv(volumeId string) (string, string) {
	tokens := strings.Split(volumeId, "/")
	return tokens[0], tokens[1]
}

func volumeNameToLvName(volumeName string) string {
	return objectNameToLvName(volumeLvPrefix, volumeName)
}

func snapshotNameToLvName(volumeName string) string {
	return objectNameToLvName(snapshotLvPrefix, volumeName)
}

func objectNameToLvName(prefix, volumeName string) string {
	h := sha256.New()
	h.Write([]byte(volumeName))
	b64 := base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(h.Sum(nil))
	// LVM doesn't like the '/' character, replace with '_'
	return prefix + strings.ReplaceAll(b64, "/", "_")
}

func lvToVolume(lv *proto.LogicalVolume) *csi.Volume {
	return &csi.Volume{
		CapacityBytes: int64(lv.LvSize),
		VolumeId:      fmt.Sprintf("%s/%s", lv.VgName, lv.LvName),
		//VolumeContext:        nil,
		//ContentSource:        nil,
		//AccessibleTopology:   nil,
	}
}

func lvToSnapshot(lv *proto.LogicalVolume) *csi.Snapshot {
	return &csi.Snapshot{
		SnapshotId:     fmt.Sprintf("%s/%s", lv.VgName, lv.LvName),
		SourceVolumeId: fmt.Sprintf("%s/%s", lv.VgName, lv.Origin),
		ReadyToUse:     true,
		CreationTime:   lv.LvTime,
		SizeBytes:      int64(lv.LvSize),
	}
}
