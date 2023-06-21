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

package driverd_test

import (
	"github.com/aleofreddi/csi-sanlock-lvm/pkg/proto/prototest"
	"reflect"
	"testing"

	"github.com/aleofreddi/csi-sanlock-lvm/pkg/diskrpc"
	"github.com/aleofreddi/csi-sanlock-lvm/pkg/driverd"
	pkg "github.com/aleofreddi/csi-sanlock-lvm/pkg/driverd"
	"github.com/aleofreddi/csi-sanlock-lvm/pkg/mock"
	"github.com/aleofreddi/csi-sanlock-lvm/pkg/proto"
	pb "github.com/aleofreddi/csi-sanlock-lvm/pkg/proto"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
)

func Test_controllerServer_CreateVolume(t *testing.T) {
	req := &csi.CreateVolumeRequest{
		Name: "volume1@vg00",
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 1 << 20,
			LimitBytes:    1 << 20,
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						VolumeMountGroup: "1000",
						MountFlags:       []string{"noatime"},
					},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
		Parameters: map[string]string{
			"volumeGroup": "vg00",
		},
	}
	type deps struct {
		lvmctrld     pb.LvmCtrldClient
		volumeLocker driverd.VolumeLocker
		diskRpc      diskrpc.DiskRpc
		fsRegistry   driverd.FileSystemRegistry
		defaultFs    string
	}
	type args struct {
		ctx context.Context
		req *csi.CreateVolumeRequest
	}
	tests := []struct {
		name        string
		setup       func(controller *gomock.Controller) *deps
		args        args
		want        *csi.CreateVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when VolumeId is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "ext4",
				}
			},
			args{
				context.Background(),
				prototest.Without(req, "Name"),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when VolumeCapabilities is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "ext4",
				}
			},
			args{
				context.Background(),
				prototest.Without(req, "VolumeCapabilities"),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when filesystem doesn't accept access type",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "ext4",
				}
			},
			args{
				context.Background(),
				prototest.Merge(req,
					&csi.CreateVolumeRequest{
						VolumeCapabilities: []*csi.VolumeCapability{
							{
								AccessType: &csi.VolumeCapability_Mount{
									Mount: &csi.VolumeCapability_MountVolume{
										FsType:           "raw",
										MountFlags:       []string{"noatime"},
										VolumeMountGroup: "1000",
									},
								},
								AccessMode: &csi.VolumeCapability_AccessMode{
									Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
								},
							},
						},
					},
				),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when capacity range is not satisfiable (required = limit, not aligned)",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "ext4",
				}
			},
			args{
				context.Background(),
				prototest.Merge(req,
					&csi.CreateVolumeRequest{
						CapacityRange: &csi.CapacityRange{
							RequiredBytes: 1<<20 + 1,
							LimitBytes:    1<<20 + 1,
						},
					},
				),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when capacity range is not satisfiable (required != limit, not 512 multiple in between)",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "ext4",
				}
			},
			args{
				context.Background(),
				prototest.Merge(req,
					&csi.CreateVolumeRequest{
						CapacityRange: &csi.CapacityRange{
							RequiredBytes: 1<<20 + 1,
							LimitBytes:    1<<20 + 511,
						},
					},
				),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when lvcreate fails (not found)",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),
					expectGetFileSystem(t, fsRegistry, "ext4", fs, nil),
					expectFsAccepts(t, fs, pkg.MountAccessType, true),
					expectLvCreate(t, client, "vg00", "csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk", 1<<20+512, []string{"csi-sanlock-lvm.vleo.net/fs=ext4", "csi-sanlock-lvm.vleo.net/name=volume1&40vg00"}, "", pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE, status.Error(codes.ResourceExhausted, "not found")),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "ext4",
				}
			},
			args{
				context.Background(),
				prototest.Merge(req,
					&csi.CreateVolumeRequest{
						CapacityRange: &csi.CapacityRange{
							RequiredBytes: 1<<20 + 1,
							LimitBytes:    1<<20 + 4096,
						},
					},
				),
			},
			nil,
			true,
			codes.ResourceExhausted,
		},
		{
			"Should fail and rollback when volume lock fails",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),
					expectGetFileSystem(t, fsRegistry, "ext4", fs, nil),
					expectFsAccepts(t, fs, pkg.MountAccessType, true),
					expectLvCreate(t, client, "vg00", "csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk", 1<<20+512, []string{"csi-sanlock-lvm.vleo.net/fs=ext4", "csi-sanlock-lvm.vleo.net/name=volume1&40vg00"}, "", pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE, nil),
					expectLockVolume(t, locker, *MustVolumeRefFromID("csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00"), "CreateVolume\\(csl-v-WMqg\\+\\+utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00,.*\\)", status.Error(codes.Internal, "internal error")),
					expectLvRemove(t, client, []string{"vg00/csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk"}, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "ext4",
				}
			},
			args{
				context.Background(),
				prototest.Merge(req,
					&csi.CreateVolumeRequest{
						CapacityRange: &csi.CapacityRange{
							RequiredBytes: 1<<20 + 1,
							LimitBytes:    1<<20 + 4096,
						},
					},
				),
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail and rollback when mkfs fails",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),
					expectGetFileSystem(t, fsRegistry, "ext4", fs, nil),
					expectFsAccepts(t, fs, pkg.MountAccessType, true),
					expectLvCreate(t, client, "vg00", "csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk", 1<<20+512, []string{"csi-sanlock-lvm.vleo.net/fs=ext4", "csi-sanlock-lvm.vleo.net/name=volume1&40vg00"}, "", pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE, nil),
					expectLockVolume(t, locker, *MustVolumeRefFromID("csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00"), "CreateVolume\\(csl-v-WMqg\\+\\+utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00,.*\\)", nil),
					expectFsMake(t, fs, "/dev/vg00/csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk", status.Error(codes.Internal, "internal error")),
					expectUnlockVolume(t, locker, *MustVolumeRefFromID("csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00"), "CreateVolume\\(csl-v-WMqg\\+\\+utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00,.*\\)", status.Error(codes.NotFound, "not found")),
					expectLvRemove(t, client, []string{"vg00/csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk"}, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "ext4",
				}
			},
			args{
				context.Background(),
				prototest.Merge(req,
					&csi.CreateVolumeRequest{
						CapacityRange: &csi.CapacityRange{
							RequiredBytes: 1<<20 + 1,
							LimitBytes:    1<<20 + 4096,
						},
					},
				),
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should round up size to the nearest multiple of 512",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),
					expectGetFileSystem(t, fsRegistry, "ext4", fs, nil),
					expectFsAccepts(t, fs, pkg.MountAccessType, true),
					expectLvCreate(t, client, "vg00", "csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk", 1<<20+512, []string{"csi-sanlock-lvm.vleo.net/fs=ext4", "csi-sanlock-lvm.vleo.net/name=volume1&40vg00"}, "", pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE, nil),
					expectLockVolume(t, locker, *MustVolumeRefFromID("csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00"), "CreateVolume\\(csl-v-WMqg\\+\\+utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00,.*\\)", nil),
					expectFsMake(t, fs, "/dev/vg00/csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk", nil),
					expectUnlockVolume(t, locker, *MustVolumeRefFromID("csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00"), "CreateVolume\\(csl-v-WMqg\\+\\+utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00,.*\\)", nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "ext4",
				}
			},
			args{
				context.Background(),
				prototest.Merge(req,
					&csi.CreateVolumeRequest{
						CapacityRange: &csi.CapacityRange{
							RequiredBytes: 1<<20 + 1,
							LimitBytes:    1<<20 + 4096,
						},
					},
				),
			},
			&csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId:      "csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00",
					CapacityBytes: 1<<20 + 512,
					VolumeContext: map[string]string{
						"volumeGroup": "vg00",
					},
				},
			},
			false,
			codes.OK,
		},
		{
			"Should create the volume",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),
					expectGetFileSystem(t, fsRegistry, "ext4", fs, nil),
					expectFsAccepts(t, fs, pkg.MountAccessType, true),
					expectLvCreate(t, client, "vg00", "csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk", 1<<20, []string{"csi-sanlock-lvm.vleo.net/fs=ext4", "csi-sanlock-lvm.vleo.net/name=volume1&40vg00"}, "", pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE, nil),
					expectLockVolume(t, locker, *MustVolumeRefFromID("csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00"), "CreateVolume\\(csl-v-WMqg\\+\\+utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00,.*\\)", nil),
					expectFsMake(t, fs, "/dev/vg00/csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk", nil),
					expectUnlockVolume(t, locker, *MustVolumeRefFromID("csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00"), "CreateVolume\\(csl-v-WMqg\\+\\+utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00,.*\\)", nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "ext4",
				}
			},
			args{
				context.Background(),
				req,
			},
			&csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId:      "csl-v-WMqg++utkdTco2QDWtQItAzbBmUsSjjxJkM2EP6nxpk@vg00",
					CapacityBytes: 1 << 20,
					VolumeContext: map[string]string{
						"volumeGroup": "vg00",
					},
				},
			},
			false,
			codes.OK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			fields := tt.setup(mockCtrl)
			cs, _ := driverd.NewControllerServer(
				fields.lvmctrld,
				fields.volumeLocker,
				fields.diskRpc,
				fields.fsRegistry,
				fields.defaultFs,
			)
			got, err := cs.CreateVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("CreateVolume() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerServer_ListVolumes(t *testing.T) {
	type fields struct {
		lvmctrld     proto.LvmCtrldClient
		volumeLocker pkg.VolumeLocker
		diskRpc      diskrpc.DiskRpc
		fsRegistry   driverd.FileSystemRegistry
		defaultFs    string
	}
	type args struct {
		ctx context.Context
		req *csi.ListVolumesRequest
	}
	tests := []struct {
		name        string
		fields      func(controller *gomock.Controller) *fields
		args        args
		want        *csi.ListVolumesResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail with abort when starting token is invalid",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),

					client.EXPECT().
						Lvs(
							gomock.Any(),
							CmpMatcher(t, &proto.LvsRequest{Select: "lv_name=~^csl-v-", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
							gomock.Any(),
						).
						Return(
							&proto.LvsResponse{Lvs: []*proto.LogicalVolume{
								{VgName: "vg1", LvName: "lv1"},
								{VgName: "vg1", LvName: "lv2"},
								{VgName: "vg2", LvName: "lv1"},
								{VgName: "vg2", LvName: "lv2"},
								{VgName: "vg2", LvName: "lv3"},
								{VgName: "vg2", LvName: "lv4"},
								{VgName: "vg2", LvName: "lv5"},
								{VgName: "vg2", LvName: "lv6"},
							}},
							nil,
						),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					defaultFs:    "testfs",
				}
			},
			args{
				context.Background(),
				&csi.ListVolumesRequest{
					StartingToken: "invalid/one",
					MaxEntries:    3,
				},
			},
			nil,
			true,
			codes.Aborted,
		},
		{
			"Should paginate results when no starting token is provided",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),

					client.EXPECT().
						Lvs(
							gomock.Any(),
							CmpMatcher(t, &proto.LvsRequest{Select: "lv_name=~^csl-v-", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
							gomock.Any(),
						).
						Return(
							&proto.LvsResponse{Lvs: []*proto.LogicalVolume{
								{VgName: "vg1", LvName: "lv1"},
								{VgName: "vg1", LvName: "lv2"},
								{VgName: "vg2", LvName: "lv1"},
								{VgName: "vg2", LvName: "lv2"},
								{VgName: "vg2", LvName: "lv3"},
								{VgName: "vg2", LvName: "lv4"},
								{VgName: "vg2", LvName: "lv5"},
								{VgName: "vg2", LvName: "lv6"},
							}},
							nil,
						),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					defaultFs:    "testfs",
				}
			},
			args{
				context.Background(),
				&csi.ListVolumesRequest{
					MaxEntries: 3,
				},
			},
			&csi.ListVolumesResponse{
				Entries: []*csi.ListVolumesResponse_Entry{
					{Volume: &csi.Volume{VolumeId: "lv1@vg1"}},
					{Volume: &csi.Volume{VolumeId: "lv2@vg1"}},
					{Volume: &csi.Volume{VolumeId: "lv1@vg2"}},
				},
				NextToken: "lv2@vg2",
			},
			false,
			codes.OK,
		},
		{
			"Should paginate results when starting token",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),

					client.EXPECT().
						Lvs(
							gomock.Any(),
							CmpMatcher(t, &proto.LvsRequest{Select: "lv_name=~^csl-v-", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
							gomock.Any(),
						).
						Return(
							&proto.LvsResponse{Lvs: []*proto.LogicalVolume{
								{VgName: "vg1", LvName: "lv1"},
								{VgName: "vg1", LvName: "lv2"},
								{VgName: "vg2", LvName: "lv1"},
								{VgName: "vg2", LvName: "lv2"},
								{VgName: "vg2", LvName: "lv3"},
								{VgName: "vg2", LvName: "lv4"},
								{VgName: "vg2", LvName: "lv5"},
								{VgName: "vg2", LvName: "lv6"},
							}},
							nil,
						),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					defaultFs:    "testfs",
				}
			},
			args{
				context.Background(),
				&csi.ListVolumesRequest{
					StartingToken: "lv3@vg2",
					MaxEntries:    3,
				},
			},
			&csi.ListVolumesResponse{
				Entries: []*csi.ListVolumesResponse_Entry{
					{Volume: &csi.Volume{VolumeId: "lv3@vg2"}},
					{Volume: &csi.Volume{VolumeId: "lv4@vg2"}},
					{Volume: &csi.Volume{VolumeId: "lv5@vg2"}},
				},
				NextToken: "lv6@vg2",
			},
			false,
			codes.OK,
		},
		{
			"Should paginate results when starting token at last page",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),

					client.EXPECT().
						Lvs(
							gomock.Any(),
							CmpMatcher(t, &proto.LvsRequest{Select: "lv_name=~^csl-v-", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
							gomock.Any(),
						).
						Return(
							&proto.LvsResponse{Lvs: []*proto.LogicalVolume{
								{VgName: "vg1", LvName: "lv1"},
								{VgName: "vg1", LvName: "lv2"},
								{VgName: "vg2", LvName: "lv1"},
								{VgName: "vg2", LvName: "lv2"},
								{VgName: "vg2", LvName: "lv3"},
								{VgName: "vg2", LvName: "lv4"},
								{VgName: "vg2", LvName: "lv5"},
								{VgName: "vg2", LvName: "lv6"},
							}},
							nil,
						),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					defaultFs:    "testfs",
				}
			},
			args{
				context.Background(),
				&csi.ListVolumesRequest{
					StartingToken: "lv5@vg2",
					MaxEntries:    3,
				},
			},
			&csi.ListVolumesResponse{
				Entries: []*csi.ListVolumesResponse_Entry{
					{Volume: &csi.Volume{VolumeId: "lv5@vg2"}},
					{Volume: &csi.Volume{VolumeId: "lv6@vg2"}},
				},
			},
			false,
			codes.OK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			fields := tt.fields(mockCtrl)
			ns, _ := pkg.NewControllerServer(
				fields.lvmctrld,
				fields.volumeLocker,
				fields.diskRpc,
				fields.fsRegistry,
				fields.defaultFs,
			)
			got, err := ns.ListVolumes(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListVolumes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("ListVolumes() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListVolumes() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_controllerServer_ListSnapshots(t *testing.T) {
	type fields struct {
		lvmctrld     proto.LvmCtrldClient
		volumeLocker pkg.VolumeLocker
		diskRpc      diskrpc.DiskRpc
		fsRegistry   driverd.FileSystemRegistry
		defaultFs    string
	}
	type args struct {
		ctx context.Context
		req *csi.ListSnapshotsRequest
	}
	tests := []struct {
		name        string
		fields      func(controller *gomock.Controller) *fields
		args        args
		want        *csi.ListSnapshotsResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail with abort when starting token is invalid",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),

					client.EXPECT().
						Lvs(
							gomock.Any(),
							CmpMatcher(t, &proto.LvsRequest{Select: "lv_name=~^csl-s-", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
							gomock.Any(),
						).
						Return(
							&proto.LvsResponse{Lvs: []*proto.LogicalVolume{
								{VgName: "vg1", LvName: "lv1", Origin: "lv0"},
								{VgName: "vg1", LvName: "lv2", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv1", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv2", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv3", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv4", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv5", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv6", Origin: "lv0"},
							}},
							nil,
						),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "testfs",
				}
			},
			args{
				context.Background(),
				&csi.ListSnapshotsRequest{
					StartingToken: "invalid/one",
					MaxEntries:    3,
				},
			},
			nil,
			true,
			codes.Aborted,
		},
		{
			"Should paginate results when no starting token is provided",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),

					client.EXPECT().
						Lvs(
							gomock.Any(),
							CmpMatcher(t, &proto.LvsRequest{Select: "lv_name=~^csl-s-", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
							gomock.Any(),
						).
						Return(
							&proto.LvsResponse{Lvs: []*proto.LogicalVolume{
								{VgName: "vg1", LvName: "lv1", Origin: "lv0"},
								{VgName: "vg1", LvName: "lv2", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv1", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv2", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv3", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv4", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv5", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv6", Origin: "lv0"},
							}},
							nil,
						),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "testfs",
				}
			},
			args{
				context.Background(),
				&csi.ListSnapshotsRequest{
					MaxEntries: 3,
				},
			},
			&csi.ListSnapshotsResponse{
				Entries: []*csi.ListSnapshotsResponse_Entry{
					{Snapshot: &csi.Snapshot{SnapshotId: "lv1@vg1", SourceVolumeId: "lv0@vg1", ReadyToUse: true}},
					{Snapshot: &csi.Snapshot{SnapshotId: "lv2@vg1", SourceVolumeId: "lv0@vg1", ReadyToUse: true}},
					{Snapshot: &csi.Snapshot{SnapshotId: "lv1@vg2", SourceVolumeId: "lv0@vg2", ReadyToUse: true}},
				},
				NextToken: "lv2@vg2",
			},
			false,
			codes.OK,
		},
		{
			"Should paginate results when starting token",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),

					client.EXPECT().
						Lvs(
							gomock.Any(),
							CmpMatcher(t, &proto.LvsRequest{Select: "lv_name=~^csl-s-", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
							gomock.Any(),
						).
						Return(
							&proto.LvsResponse{Lvs: []*proto.LogicalVolume{
								{VgName: "vg1", LvName: "lv1", Origin: "lv0"},
								{VgName: "vg1", LvName: "lv2", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv1", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv2", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv3", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv4", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv5", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv6", Origin: "lv0"},
							}},
							nil,
						),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "testfs",
				}
			},
			args{
				context.Background(),
				&csi.ListSnapshotsRequest{
					StartingToken: "lv3@vg2",
					MaxEntries:    3,
				},
			},
			&csi.ListSnapshotsResponse{
				Entries: []*csi.ListSnapshotsResponse_Entry{
					{Snapshot: &csi.Snapshot{SnapshotId: "lv3@vg2", SourceVolumeId: "lv0@vg2", ReadyToUse: true}},
					{Snapshot: &csi.Snapshot{SnapshotId: "lv4@vg2", SourceVolumeId: "lv0@vg2", ReadyToUse: true}},
					{Snapshot: &csi.Snapshot{SnapshotId: "lv5@vg2", SourceVolumeId: "lv0@vg2", ReadyToUse: true}},
				},
				NextToken: "lv6@vg2",
			},
			false,
			codes.OK,
		},
		{
			"Should paginate results when starting token at last page",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				diskRpc := mock.NewMockDiskRpc(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectRegisterChannel(t, diskRpc),

					client.EXPECT().
						Lvs(
							gomock.Any(),
							CmpMatcher(t, &proto.LvsRequest{Select: "lv_name=~^csl-s-", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
							gomock.Any(),
						).
						Return(
							&proto.LvsResponse{Lvs: []*proto.LogicalVolume{
								{VgName: "vg1", LvName: "lv1", Origin: "lv0"},
								{VgName: "vg1", LvName: "lv2", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv1", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv2", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv3", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv4", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv5", Origin: "lv0"},
								{VgName: "vg2", LvName: "lv6", Origin: "lv0"},
							}},
							nil,
						),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					diskRpc:      diskRpc,
					fsRegistry:   fsRegistry,
					defaultFs:    "testfs",
				}
			},
			args{
				context.Background(),
				&csi.ListSnapshotsRequest{
					StartingToken: "lv5@vg2",
					MaxEntries:    3,
				},
			},
			&csi.ListSnapshotsResponse{
				Entries: []*csi.ListSnapshotsResponse_Entry{
					{Snapshot: &csi.Snapshot{SnapshotId: "lv5@vg2", SourceVolumeId: "lv0@vg2", ReadyToUse: true}},
					{Snapshot: &csi.Snapshot{SnapshotId: "lv6@vg2", SourceVolumeId: "lv0@vg2", ReadyToUse: true}},
				},
			},
			false,
			codes.OK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			fields := tt.fields(mockCtrl)
			ns, _ := pkg.NewControllerServer(
				fields.lvmctrld,
				fields.volumeLocker,
				fields.diskRpc,
				fields.fsRegistry,
				fields.defaultFs,
			)
			got, err := ns.ListSnapshots(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListSnapshots() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("ListSnapshots() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListSnapshots() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func expectGetStatus(t *testing.T, client *mock.MockLvmCtrldClient) *gomock.Call {
	return client.EXPECT().
		GetStatus(gomock.Any(), CmpMatcher(t, &proto.GetStatusRequest{}, protocmp.Transform())).
		Return(
			&proto.GetStatusResponse{NodeId: 1234},
			nil,
		)
}

func expectRegisterChannel(t *testing.T, diskRpc *mock.MockDiskRpc) *gomock.Call {
	return diskRpc.EXPECT().
		Register(diskrpc.Channel(0), gomock.Any())
}

func expectLvCreate(t *testing.T, client *mock.MockLvmCtrldClient, vgName, lvName string, size uint64, tags []string, origin string, activation pb.LvActivationMode, err error) *gomock.Call {
	return client.EXPECT().
		LvCreate(
			gomock.Any(),
			CmpMatcher(t, &pb.LvCreateRequest{
				VgName:   vgName,
				LvName:   lvName,
				Size:     size,
				LvTags:   tags,
				Origin:   origin,
				Activate: activation,
			}, protocmp.Transform()),
			gomock.Any(),
		).
		Return(
			&pb.LvCreateResponse{},
			err,
		)
}

func expectLvRemove(t *testing.T, client *mock.MockLvmCtrldClient, target []string, err error) *gomock.Call {
	return client.EXPECT().
		LvRemove(
			gomock.Any(),
			CmpMatcher(t, &pb.LvRemoveRequest{
				Select: "",
				Target: target,
			}, protocmp.Transform()),
			gomock.Any(),
		).
		Return(
			&pb.LvRemoveResponse{},
			err,
		)
}

func expectFsMake(t *testing.T, fs *mock.MockFileSystem, device string, err error) *gomock.Call {
	return fs.EXPECT().
		Make(device).
		Return(err)
}
