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
	"bufio"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/aleofreddi/csi-sanlock-lvm/pkg/diskrpc"
	pb "github.com/aleofreddi/csi-sanlock-lvm/pkg/proto"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"github.com/pkg/math"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

const (
	// Volume parameter
	maxSizeParamKey    = "maxSize"
	maxSizePctParamKey = "maxSizePct"
	vgParamKey         = "volumeGroup"

	// Default volume capacity.
	defaultCapacity = 1 << 20

	// DiskRPC channel.
	controllerServerDiskRPCID diskrpc.Channel = 0
)

var (
	volumeIdRe = regexp.MustCompile("^[a-zA-Z0-9+_.][a-zA-Z0-9+_.-]*@[a-zA-Z0-9+_.][a-zA-Z0-9+_.-]*$")
	vgRe       = regexp.MustCompile("^[a-zA-Z0-9+_.][a-zA-Z0-9+_.-]*$")
)

type VolumeAccessType int

const (
	MountAccessType VolumeAccessType = iota
	BlockAccessType

	BlockAccessFsName = "$raw" // the $ prefix is to avoid colliding with a real filesystem name.
)

var controllerCapabilities = map[csi.ControllerServiceCapability_RPC_Type]struct{}{
	csi.ControllerServiceCapability_RPC_CLONE_VOLUME:           {},
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT: {},
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:   {},
	csi.ControllerServiceCapability_RPC_EXPAND_VOLUME:          {},
	csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS:         {},
	csi.ControllerServiceCapability_RPC_LIST_VOLUMES:           {},
	// FIXME: check if it's worth adding csi.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES
	// See https://github.com/container-storage-interface/spec/blob/master/spec.md#listvolumes
}

// Volume capability compatibility map. This map declares for each access mode by supported by this
// driver, all the compatible, narrower access modes.
var volCapAccessModeCompat = map[csi.VolumeCapability_AccessMode_Mode]map[csi.VolumeCapability_AccessMode_Mode]interface{}{
	csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER: {
		csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER:  nil,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER: nil,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:        nil,
	},
	csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY: {
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY: nil,
	},
	csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER: {
		csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER: nil,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:       nil,
	},
	csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER: {
		csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER:  nil,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER: nil,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:        nil,
	},
}

type controllerServer struct {
	baseServer
	volumeLock VolumeLocker
	diskRpc    diskrpc.DiskRpc
	fsRegistry FileSystemRegistry
	defaultFs  string
}

func NewControllerServer(lvmctrld pb.LvmCtrldClient, volumeLock VolumeLocker, diskRpc diskrpc.DiskRpc, fsRegistry FileSystemRegistry, defaultFs string) (*controllerServer, error) {
	bs, err := newBaseServer(lvmctrld)
	if err != nil {
		return nil, err
	}
	cs := &controllerServer{
		baseServer: *bs,
		volumeLock: volumeLock,
		diskRpc:    diskRpc,
		fsRegistry: fsRegistry,
		defaultFs:  defaultFs,
	}
	if err = diskRpc.Register(controllerServerDiskRPCID, cs); err != nil {
		return nil, fmt.Errorf("failed to register controller server channel: %v", err)
	}
	return cs, nil
}

func (cs *controllerServer) ControllerGetCapabilities(_ context.Context, _ *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
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
	// Check arguments.
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume name")
	}
	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing volume capabilities")
	}

	// Parse capabilities.
	cap, err2 := reduceVolumeCapabilities(req.GetVolumeCapabilities())
	if err2 != nil {
		return nil, err2
	}
	fsName := cs.defaultFs
	if cap.GetBlock() != nil {
		fsName = BlockAccessFsName
	}

	// Parse parameters.
	vgName, present := req.Parameters[vgParamKey]
	if !present || vgName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing %s parameter", vgParamKey)
	}
	if !vgRe.MatchString(vgName) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid %s parameter", vgParamKey)
	}

	// Compute volume size.
	size := uint64(0)
	if req.GetCapacityRange() != nil {
		if req.GetCapacityRange().GetRequiredBytes() > 0 {
			size = uint64(req.GetCapacityRange().GetRequiredBytes())
			size = (size + 511) / 512 * 512
			if req.GetCapacityRange().GetLimitBytes() > 0 && size > uint64(req.GetCapacityRange().GetLimitBytes()) {
				return nil, status.Errorf(codes.InvalidArgument, "unsatisfiable volume capacity range: required %d bytes <= size %d bytes <= limit %d bytes", req.GetCapacityRange().GetRequiredBytes(), size, req.GetCapacityRange().GetLimitBytes())
			}
		} else if req.GetCapacityRange().GetLimitBytes() > 0 {
			size = uint64(req.GetCapacityRange().GetLimitBytes())
			size = (size - 511) / 512 * 512
		} else {
			return nil, status.Error(codes.InvalidArgument, "missing volume capacity range")
		}
	} else {
		size = defaultCapacity
	}

	vol := NewVolumeRefFromVgTypeName(vgName, VolumeVolType, req.GetName())
	// Retrieve source if any
	var srcVol, srcSnap, lockVol *VolumeInfo
	var srcSize uint64
	var err error
	if src := req.VolumeContentSource; src != nil {
		if src.GetSnapshot() != nil && src.GetVolume() != nil || src.GetSnapshot() == nil && src.GetVolume() == nil {
			return nil, status.Errorf(codes.InvalidArgument, "a source snapshot or a source volume is required when content source is specified")
		}
		if s := src.GetSnapshot(); s != nil {
			srcRef, err := NewVolumeRefFromID(s.GetSnapshotId())
			if err != nil {
				return nil, status.Errorf(codes.NotFound, "invalid source snapshot: %v", err)
			}
			if srcRef != nil && srcRef.Vg() != vol.Vg() {
				return nil, status.Errorf(codes.InvalidArgument, "volume group %q must match the one of the source snapshot %q", vol.Vg(), srcRef.Vg())
			}
			srcSnap, err = cs.fetch(ctx, srcRef)
			if err != nil {
				return nil, err
			}
		} else if s := src.GetVolume(); s != nil {
			srcRef, err := NewVolumeRefFromID(s.GetVolumeId())
			if err != nil {
				return nil, status.Errorf(codes.NotFound, "invalid source volume: %v", err)
			}
			if srcRef != nil && srcRef.Vg() != vol.Vg() {
				return nil, status.Errorf(codes.InvalidArgument, "volume group %q must match the one of the source volume %q", vol.Vg(), srcRef.Vg())
			}
			srcVol, err = cs.fetch(ctx, srcRef)
			if err != nil {
				return nil, err
			}
		}
		// Set the volume to lock. For snapshots that would be the origin volume.
		if srcVol != nil {
			lockVol = srcVol
		} else {
			orig := srcSnap.OriginRef()
			lockVol, err = cs.fetch(ctx, orig)
			if err != nil {
				return nil, err
			}
		}
		tags, err := lockVol.Tags()
		if err != nil {
			return nil, err
		}
		srcFsName, ok := tags[fsTagKey]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "missing %s tag on volume %q", fsTagKey, lockVol)
		}
		if srcFsName != fsName {
			return nil, status.Errorf(codes.InvalidArgument, "incompatible filesystem: source filesystem %s, target filesystem %s", srcFsName, fsName)
		}
		srcSize = lockVol.LvSize
		if size < srcSize {
			return nil, status.Errorf(codes.InvalidArgument, "source content requires at least %d bytes, got only %d bytes", srcSize, size)
		}
	}

	// Retrieve filesystem.
	fs, err := cs.fsRegistry.GetFileSystem(fsName)
	if err != nil {
		return nil, err
	}
	at := MountAccessType
	if cap.GetBlock() != nil {
		at = BlockAccessType
	}
	if !fs.Accepts(at) {
		return nil, status.Error(codes.InvalidArgument, "filesystem is not compatible with the given access type")
	}

	// Create a map of temporary filesystems
	cleanupVols := make(map[string]struct{})
	defer func() {
		for v, _ := range cleanupVols {
			klog.V(5).Infof("Cleaning up temporary volume %q", v)
			_, err = cs.lvmctrld.LvRemove(ctx, &pb.LvRemoveRequest{
				Target: []string{v},
			})
			if err != nil {
				klog.Warningf("Failed to remove temporary volume %q: %v", v, err)
			}
		}
	}()

	// Create a temporary snapshot from src volume, if specified.
	var dataVol *VolumeRef
	if lockVol != nil {
		// Try to acquire exclusive lock on orig, or delegate to owner node if already locked.
		klog.V(3).Infof("Trying to acquire lock on %q to read source data", lockVol)
		// We add a uuid because we don't want this lock to be reentrant.
		lockId := fmt.Sprintf("CreateVolume(%s,%s)", vol, uuid.New().String())
		err = cs.volumeLock.LockVolume(ctx, lockVol.VolumeRef, lockId)
		if status.Code(err) == codes.PermissionDenied {
			ownerId, ownerNode, err := cs.volumeLock.GetOwner(ctx, lockVol.VolumeRef)
			if err != nil {
				return nil, err
			}
			klog.V(3).Infof("Delegating CreateVolume(%s) to node %s because it owns %s", vol, ownerNode, lockVol)
			res := csi.CreateVolumeResponse{}
			err = cs.diskRpc.Invoke(ctx, ownerId, controllerServerDiskRPCID, "CreateVolume", req, &res)
			klog.V(5).Infof("CreateVolume(%s) on node %s returned (%+v, %v)", vol, ownerNode, res, err)
			return &res, err
		} else if err != nil {
			return nil, err
		}
		defer cs.tryVolumeUnlock(ctx, lockVol.VolumeRef, lockId)

		if srcVol != nil {
			// When cloning a volume, we'll create a temporary snapshot to have a consistent data view.
			dataVol = NewVolumeRefFromVgTypeName(srcVol.Vg(), TemporaryVolType, uuid.New().String())
			klog.V(3).Infof("Content source is a volume, using a temporary snapshot %s as content source", dataVol)
			_, err = cs.lvmctrld.LvCreate(
				ctx,
				&pb.LvCreateRequest{
					VgName:   dataVol.Vg(),
					LvName:   dataVol.Lv(),
					Origin:   srcVol.Lv(),
					Activate: pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE,
					Size:     srcSize, // FIXME: we should support tweaking this!
				},
			)
			if err != nil {
				return nil, status.Errorf(status.Code(err), "failed to snapshot %v to create volume %s: %v", srcVol, vol, err)
			}
			// Add dataVol data lv to cleanup list
			cleanupVols[dataVol.VgLv()] = struct{}{}
		} else {
			klog.V(3).Infof("Using the snapshot %s as content source", dataVol)
			dataVol = &srcSnap.VolumeRef
		}
	}

	// Create data volume
	tags := map[TagKey]string{
		fsTagKey:   fsName,
		nameTagKey: req.GetName(),
	}
	if srcVol != nil {
		tags[sourceTagKey] = srcVol.ID()
	}
	_, err = cs.lvmctrld.LvCreate(
		ctx,
		&pb.LvCreateRequest{
			VgName:   vol.Vg(),
			LvName:   vol.Lv(),
			Activate: pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE,
			Size:     size,
			LvTags:   encodeTags(tags),
		},
	)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			d, err := cs.fetch(ctx, vol)
			if status.Code(err) == codes.NotFound {
				// The volume was there, but now it disappeared due to a concurrent operation.
				return nil, status.Errorf(codes.Aborted, "operation already in progress on volume %s", vol)
			} else if err != nil {
				return nil, err
			}
			// FIXME: check that `existing` is a volume, not a snapshot
			// FIXME: check that sourceTag matches srcVol
			if uint64(req.GetCapacityRange().GetRequiredBytes()) > d.LvSize {
				return nil, status.Errorf(codes.AlreadyExists, "volume with the same name but with different size already exist")
			}
			return &csi.CreateVolumeResponse{
				Volume: lvToVolume(d.LogicalVolume),
			}, nil
		}
		return nil, status.Errorf(status.Code(err), "failed to create volume %s: %v", vol, err)
	}
	// Add new volume to cleanup list
	cleanupVols[vol.VgLv()] = struct{}{}
	// Now lock the new volume
	lockId := fmt.Sprintf("CreateVolume(%s,%s)", vol, uuid.New().String())
	if err := cs.volumeLock.LockVolume(ctx, *vol, lockId); err != nil {
		return nil, err
	}
	defer cs.tryVolumeUnlock(ctx, *vol, lockId)

	if dataVol == nil {
		// Format the volume if no source is specified, else copy the data
		klog.Infof("Formatting the volume %s with %s", vol, fsName)
		err = fs.Make(vol.DevPath())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to format volume: %v", err)
		}
	} else {
		// Copy data from dataVol
		klog.Infof("Starting to copy %d bytes from source volume %s to target volume %s", srcSize, dataVol, vol)
		srcFile, err := os.Open(dataVol.DevPath())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to open source volume %s: %v", dataVol, err)
		}
		defer func() {
			if srcFile != nil {
				if err := srcFile.Close(); err != nil {
					klog.Warningf("Failed to close %s: %v", srcFile.Name(), err)
				}
			}
		}()
		dstFile, err := os.OpenFile(vol.DevPath(), os.O_RDWR, 0700)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to open destination volume %s: %v", vol, err)
		}
		defer func() {
			if dstFile != nil {
				if err := dstFile.Close(); err != nil {
					klog.Warningf("Failed to close %s: %v", dstFile.Name(), err)
				}
			}
		}()

		written, err := io.Copy(bufio.NewWriter(dstFile), bufio.NewReader(srcFile))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to copy source content: %v", err)
		}
		if uint64(written) != srcSize {
			return nil, status.Errorf(codes.Internal, "failed to copy source content: copied %d bytes, expected %d bytes", written, srcSize)
		}
		if err = srcFile.Close(); err != nil {
			klog.Warningf("Failed to close %s: %v", srcFile.Name(), err)
		}
		if err = dstFile.Close(); err != nil {
			klog.Warningf("Failed to close %s: %v", dstFile.Name(), err)
		}
		srcFile = nil
		dstFile = nil
		klog.Infof("Copied %d bytes from source volume %s to target volume %s", srcSize, dataVol, vol)
	}

	delete(cleanupVols, vol.VgLv())
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      vol.ID(),
			CapacityBytes: int64(size),
			VolumeContext: req.GetParameters(),
			ContentSource: req.GetVolumeContentSource(),
			//AccessibleTopology: (use vgname here)
		},
	}, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// Check arguments
	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if !volumeIdRe.MatchString(volumeId) {
		return &csi.DeleteVolumeResponse{}, nil
	}

	vol, err := NewVolumeRefFromID(volumeId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume id: %v", err)
	}

	// Try to remove the volume. If it's an origin for some snapshot it will be skipped.
	_, err = cs.lvmctrld.LvRemove(ctx, &pb.LvRemoveRequest{
		Select: "lv_role!=origin",
		Target: []string{vol.VgLv()},
	})
	if status.Code(err) == codes.NotFound {
		return &csi.DeleteVolumeResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete volume %s: %v", vol, err)
	}

	// Check if the volume still exists. It could be that lvremove didn't return
	// an error but didn't match the volume (because it is a snapshot origin).
	_, err = cs.lvmctrld.Lvs(ctx, &pb.LvsRequest{
		Target: []string{vol.VgLv()},
	})
	if err == nil {
		return nil, status.Error(codes.FailedPrecondition, "failed to delete volume because of dependant snapshot")
	}
	if status.Code(err) != codes.NotFound {
		return nil, status.Errorf(codes.Internal, "failed to list volume %s: %v", vol, err)
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	// Check arguments
	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if !volumeIdRe.MatchString(volumeId) {
		return nil, status.Errorf(codes.NotFound, "volume %s not found", req.GetVolumeId())
	}
	if len(req.VolumeCapabilities) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "missing volume capabilities")
	}

	vol, err := NewVolumeRefFromID(volumeId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume id: %v", err)
	}

	// Retrieve volume details.
	d, err := cs.fetch(ctx, vol)
	if err != nil {
		return nil, err
	}
	tags, err := d.Tags()
	if err != nil {
		return nil, err
	}
	// Validate capabilities.
	valid := true
	var validCaps []*csi.VolumeCapability
	for _, cpb := range req.GetVolumeCapabilities() {
		validCap := &csi.VolumeCapability{}
		if cpb.GetAccessMode() != nil {
			accessMode := cpb.GetAccessMode().GetMode()
			valid = valid &&
				(accessMode == csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER ||
					accessMode == csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY)
			validCap.AccessMode = &csi.VolumeCapability_AccessMode{
				Mode: accessMode,
			}
		}
		if m := cpb.GetMount(); m != nil {
			valid = valid && tags[fsTagKey] == m.GetFsType()
			validCap.AccessType = &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{
					FsType: tags[fsTagKey],
				},
			}
		} else if cpb.GetBlock() != nil {
			valid = valid && tags[fsTagKey] == BlockAccessFsName
			validCap.AccessType = &csi.VolumeCapability_Block{
				Block: &csi.VolumeCapability_BlockVolume{},
			}
		}
		validCaps = append(validCaps, validCap)
	}

	if !valid {
		return &csi.ValidateVolumeCapabilitiesResponse{}, nil
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: validCaps,
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

	vgs, err := cs.lvmctrld.Vgs(ctx, &pb.VgsRequest{
		Select: strings.Join(filters, " && "),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to volume groups: %v", err)
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
	// Check arguments
	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "missing volume id")
	}
	if !volumeIdRe.MatchString(volumeId) {
		return nil, status.Errorf(codes.NotFound, "invalid volume id %q", volumeId)
	}
	if req.GetCapacityRange() == nil {
		return nil, status.Error(codes.InvalidArgument, "missing capacity range")
	}

	vol, err := NewVolumeRefFromID(volumeId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume id: %v", err)
	}

	// Retrieve current size
	d, err := cs.fetch(ctx, vol)
	if err != nil {
		return nil, err
	}

	// We always return NodeExpansionRequired to ensure that the filesystem size
	// matches the volume size, in case the CO looses state.
	requiredBytes := uint64(req.CapacityRange.RequiredBytes)
	if d.LvSize >= requiredBytes {
		return &csi.ControllerExpandVolumeResponse{CapacityBytes: int64(d.LvSize), NodeExpansionRequired: true}, nil
	}
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         int64(requiredBytes), //int64(lv.LvSize),
		NodeExpansionRequired: true,
	}, nil
}

func (cs *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	// List volumes
	volumes, err := cs.lvmctrld.Lvs(ctx, &pb.LvsRequest{
		Select: "lv_name=~^" + volumeLvPrefix,
		Sort:   []string{"vg_name", "lv_name"},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list volumes: %v", err)
	}
	lvs := volumes.Lvs

	// Paginate
	i, s := 0, len(lvs)
	if volumeId := req.StartingToken; volumeId != "" {
		if !volumeIdRe.MatchString(volumeId) {
			return nil, status.Errorf(codes.Aborted, "invalid starting token")
		}
		vol, err := NewVolumeRefFromID(volumeId)
		if err != nil {
			return nil, err
		}
		for ; i < len(lvs); i++ {
			if lvs[i].VgName == vol.Vg() && lvs[i].LvName == vol.Lv() {
				break
			}
		}
		if i == s {
			return nil, status.Errorf(codes.Aborted, "invalid starting token")
		}
	}
	if req.MaxEntries > 0 {
		s = math.Min(s, i+int(req.MaxEntries))
	}

	// Map entries
	entries := make([]*csi.ListVolumesResponse_Entry, s-i)
	for j := 0; i < s; {
		entries[j] = &csi.ListVolumesResponse_Entry{Volume: lvToVolume(lvs[i])}
		j++
		i++
	}

	// Set next page token if any
	next := ""
	if s < len(lvs) {
		vol := NewVolumeInfoFromLv(lvs[s])
		next = vol.ID()
	}

	return &csi.ListVolumesResponse{
		Entries:   entries,
		NextToken: next,
	}, nil
}

func (cs *controllerServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerGetVolume not implemented")
}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	// Check arguments.
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "missing snapshot name")
	}
	if !volumeIdRe.MatchString(req.GetSourceVolumeId()) {
		return nil, status.Error(codes.InvalidArgument, "invalid source volume id")
	}

	// Retrieve origin and snapshot volumes.
	orig, err := NewVolumeRefFromID(req.GetSourceVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid source volume: %v", err)
	}
	snap := NewVolumeRefFromVgTypeName(orig.Vg(), SnapshotVolType, req.GetName())

	// Acquire exclusive lock on orig, or delegate to owner node if already locked
	// We add a uuid because we don't want this lock to be reentrant.
	lockId := fmt.Sprintf("CreateSnapshot(%s,%s)", snap, uuid.New().String())
	err = cs.volumeLock.LockVolume(ctx, *orig, lockId)
	if status.Code(err) == codes.PermissionDenied {
		ownerId, ownerNode, err := cs.volumeLock.GetOwner(ctx, *orig)
		if err != nil {
			return nil, err
		}
		klog.V(3).Infof("Delegating CreateSnapshot(%s) to node %s because it owns %s", snap, ownerNode, orig)
		res := csi.CreateSnapshotResponse{}
		err = cs.diskRpc.Invoke(ctx, ownerId, controllerServerDiskRPCID, "CreateSnapshot", req, &res)
		klog.V(5).Infof("CreateSnapshot(%s) on node %s returned (%+v, %v)", snap, ownerNode, res, err)
		return &res, err
	} else if err != nil {
		return nil, err
	}
	defer cs.tryVolumeUnlock(ctx, *orig, lockId)

	// Retrieve origin information.
	origInfo, err := cs.fetch(ctx, orig)
	if err != nil {
		return nil, err
	}
	size := origInfo.LvSize
	if value, present := req.Parameters[maxSizePctParamKey]; present {
		maxPctSize, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid %s parameter %q: %v", maxSizePctParamKey, value, err)
		}
		if maxPctSize == 0 || maxPctSize > 1 {
			return nil, status.Errorf(codes.InvalidArgument, "invalid %s parameter %q: expected a value in range (0, 1]", maxSizePctParamKey, value)
		}
		maxSize := (uint64(float64(origInfo.LvSize)*maxPctSize) + 511) / 512 * 512
		size = math.MinUint64(size, maxSize)
	}
	if value, present := req.Parameters[maxSizeParamKey]; present {
		maxSize, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid %s parameter %q: %v", maxSizeParamKey, value, err)
		}
		if maxSize == 0 {
			return nil, status.Errorf(codes.InvalidArgument, "invalid %s parameter %q: expected a positive value", maxSizeParamKey, value)
		}
		maxSize = (maxSize + 511) / 512 * 512
		size = math.MinUint64(size, maxSize)
	}

	// Create snapshot.
	_, err = cs.lvmctrld.LvCreate(
		ctx,
		&pb.LvCreateRequest{
			VgName: snap.Vg(),
			LvName: snap.Lv(),
			Size:   size,
			Origin: orig.Lv(),
			LvTags: encodeTags(map[TagKey]string{
				nameTagKey:   req.GetName(),
				sourceTagKey: orig.ID(),
			}),
		},
	)
	if err != nil && status.Code(err) != codes.AlreadyExists {
		// Creation failed with a code != AlreadyExists.
		return nil, err
	}

	d, err := cs.fetch(ctx, snap)
	if status.Code(err) == codes.NotFound {
		// The volume was there, but now it disappeared due to a concurrent operation.
		return nil, status.Errorf(codes.Aborted, "operation already in progress on snapshot %s", snap)
	} else if err != nil {
		return nil, err
	}

	if d.Origin != orig.Lv() {
		return nil, status.Errorf(codes.AlreadyExists, "snapshot with the same name %q but different origin already exist", req.GetName())
	}
	return &csi.CreateSnapshotResponse{Snapshot: lvToSnapshot(d.LogicalVolume)}, nil
}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	// Check arguments
	volumeId := req.GetSnapshotId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "missing snapshot id")
	}
	if !volumeIdRe.MatchString(volumeId) {
		return &csi.DeleteSnapshotResponse{}, nil
	}

	// Retrieve snapshot and its origin
	snapRef, err := NewVolumeRefFromID(volumeId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid volume id: %v", err)
	}
	snap, err := cs.fetch(ctx, snapRef)
	if status.Code(err) == codes.NotFound {
		// Snapshot not found: return success.
		return &csi.DeleteSnapshotResponse{}, nil
	} else if err != nil {
		return nil, err
	}
	orig := snap.OriginRef()
	lockVol, err := cs.fetch(ctx, orig)
	if status.Code(err) == codes.NotFound {
		// Snapshot origin not found: return success - if origin disappeared, it is
		// safe to assume that its dependent snapshot got removed.
		return &csi.DeleteSnapshotResponse{}, nil
	} else if err != nil {
		return nil, err
	}
	klog.V(3).Infof("Resolved origin for %s to %s", snap, lockVol)

	// Acquire exclusive lock on orig, or delegate to owner node if already locked
	// We add a uuid because we don't want this lock to be reentrant.
	lockId := fmt.Sprintf("DeleteSnapshot(%s,%s)", snap, uuid.New().String())
	err = cs.volumeLock.LockVolume(ctx, lockVol.VolumeRef, lockId)
	if status.Code(err) == codes.PermissionDenied {
		ownerId, ownerNode, err := cs.volumeLock.GetOwner(ctx, *orig)
		if err != nil {
			return nil, status.Errorf(codes.Aborted, "failed to retrieve owner node for locked volume %q: %v", orig, err)
		}
		klog.V(3).Infof("Delegating CreateSnapshot(%s) to node %s because it owns %s", snap, ownerNode, orig)
		res := csi.DeleteSnapshotResponse{}
		err = cs.diskRpc.Invoke(ctx, ownerId, controllerServerDiskRPCID, "DeleteSnapshot", req, &res)
		klog.V(5).Infof("DeleteSnapshot(%s) on node %s returned (%+v, %v)", snap, ownerNode, res, err)
		return &res, err
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lock %s: %v", orig, err)
	}
	defer cs.tryVolumeUnlock(ctx, lockVol.VolumeRef, lockId)

	// Remove snapshot.
	_, err = cs.lvmctrld.LvRemove(ctx, &pb.LvRemoveRequest{
		Target: []string{snap.VgLv()},
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, status.Errorf(codes.Internal, "failed to delete snapshot %s: %v", volumeId, err)
	}
	return &csi.DeleteSnapshotResponse{}, nil
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	filters := []string{"lv_name=~^" + snapshotLvPrefix}
	if req.GetSnapshotId() != "" {
		if !volumeIdRe.MatchString(req.GetSnapshotId()) {
			return &csi.ListSnapshotsResponse{
				Entries: nil,
			}, nil
		}
		vol, err := NewVolumeRefFromID(req.GetSnapshotId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid snapshot id: %v", err)
		}
		filters = append(filters, "vg_name="+vol.Vg(), "lv_name="+vol.Lv())
	}
	if req.GetSourceVolumeId() != "" {
		if !volumeIdRe.MatchString(req.GetSourceVolumeId()) {
			return &csi.ListSnapshotsResponse{
				Entries: nil,
			}, nil
		}
		vol, err := NewVolumeRefFromID(req.GetSourceVolumeId())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid source volume id: %v", err)
		}
		filters = append(filters, "vg_name="+vol.Vg(), "origin="+vol.Lv())
	}

	// List snapshots
	volumes, err := cs.lvmctrld.Lvs(ctx, &pb.LvsRequest{
		Select: strings.Join(filters, " && "),
		Sort:   []string{"vg_name", "lv_name"},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list snapshots: %v", err)
	}
	lvs := volumes.Lvs

	// Paginate
	i, s := 0, len(lvs)
	if volumeId := req.StartingToken; volumeId != "" {
		if !volumeIdRe.MatchString(volumeId) {
			return nil, status.Errorf(codes.Aborted, "invalid starting token")
		}
		vol, err := NewVolumeRefFromID(volumeId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid volume id: %v", err)
		}
		for ; i < len(lvs); i++ {
			if lvs[i].VgName == vol.Vg() && lvs[i].LvName == vol.Lv() {
				break
			}
		}
		if i == s {
			return nil, status.Errorf(codes.Aborted, "invalid starting token")
		}
	}
	if req.MaxEntries > 0 {
		s = math.Min(s, i+int(req.MaxEntries))
	}

	// Map entries
	entries := make([]*csi.ListSnapshotsResponse_Entry, s-i)
	for j := 0; i < s; {
		entries[j] = &csi.ListSnapshotsResponse_Entry{Snapshot: lvToSnapshot(lvs[i])}
		j++
		i++
	}

	// Set next page token if any
	next := ""
	if s < len(lvs) {
		vol := NewVolumeRefFromLv(lvs[s])
		next = vol.ID()
	}

	return &csi.ListSnapshotsResponse{
		Entries:   entries,
		NextToken: next,
	}, nil
}

func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (cs *controllerServer) tryVolumeUnlock(ctx context.Context, vol VolumeRef, lockId string) {
	if err := cs.volumeLock.UnlockVolume(ctx, vol, lockId); err != nil {
		klog.Warningf("Failed to unlock volume %s, unlock skipped: %v", vol, err)
	}
}

func lvToVolume(lv *pb.LogicalVolume) *csi.Volume {
	vol := NewVolumeInfoFromLv(lv)
	return &csi.Volume{
		VolumeId:      vol.ID(),
		CapacityBytes: int64(lv.LvSize),
		// FIXME:
		//VolumeContext:        nil,
		//ContentSource:        nil,
		//AccessibleTopology:   nil,
	}
}

func lvToSnapshot(lv *pb.LogicalVolume) *csi.Snapshot {
	snap := NewVolumeInfoFromLv(lv)
	orig := snap.OriginRef()
	return &csi.Snapshot{
		SnapshotId:     snap.ID(),
		SourceVolumeId: orig.ID(),
		ReadyToUse:     true,
		CreationTime:   lv.LvTime,
		SizeBytes:      int64(lv.LvSize),
	}
}

func nodeIDToString(nodeID uint16) string {
	return strconv.Itoa(int(nodeID))
}

func nodeIDFromString(node string) (uint16, error) {
	nodeID, err := strconv.Atoi(node)
	if err != nil || nodeID < 0 || nodeID > 2000 {
		return 0, fmt.Errorf("invalid node id %q: %v", node, err)
	}
	return uint16(nodeID), nil
}

func reduceVolumeCapabilities(capabilities []*csi.VolumeCapability) (*csi.VolumeCapability, error) {
	// Aggregate capabilities.
	var aggCapability *csi.VolumeCapability
	for _, currCapability := range capabilities {
		if aggCapability == nil {
			aggCapability = currCapability
			// Initial validations.
			_, ok := volCapAccessModeCompat[aggCapability.AccessMode.Mode]
			if !ok {
				return nil, status.Errorf(codes.InvalidArgument, "unsupported access mode %s", aggCapability.AccessMode.Mode)
			}
			continue
		}
		// Aggregate access modes.
		aggAccessMode := aggCapability.AccessMode.Mode
		currAccessMode := currCapability.AccessMode.Mode
		if _, ok := volCapAccessModeCompat[aggAccessMode][currAccessMode]; !ok {
			if m, ok := volCapAccessModeCompat[currAccessMode]; !ok {
				return nil, status.Errorf(codes.InvalidArgument, "unsupported access mode %s", currAccessMode)
			} else {
				if _, ok := m[aggAccessMode]; !ok {
					return nil, status.Errorf(codes.InvalidArgument, "incompatible access mode specified: %s, %s", aggAccessMode, currAccessMode)
				}
				// Current access mode is broader than result's one, upgrade.
				aggCapability.AccessMode = currCapability.AccessMode
			}
		}
		// Aggregate mount.
		if !reflect.DeepEqual(aggCapability.GetAccessType(), currCapability.GetAccessType()) {
			return nil, status.Errorf(codes.InvalidArgument, "inconsistent volume access types")
		}
	}
	return aggCapability, nil
}
