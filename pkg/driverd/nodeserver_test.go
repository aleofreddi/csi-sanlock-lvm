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
	"context"
	"errors"
	"github.com/aleofreddi/csi-sanlock-lvm/pkg/proto/prototest"
	"google.golang.org/protobuf/testing/protocmp"
	"reflect"
	"testing"

	"github.com/Storytel/gomock-matchers"
	"github.com/aleofreddi/csi-sanlock-lvm/pkg/driverd"
	"github.com/aleofreddi/csi-sanlock-lvm/pkg/mock"
	pb "github.com/aleofreddi/csi-sanlock-lvm/pkg/proto"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewNodeServer(t *testing.T) {
	type deps struct {
		lvmctrld   pb.LvmCtrldClient
		volumeLock driverd.VolumeLocker
		fsRegistry driverd.FileSystemRegistry
	}
	tests := []struct {
		name        string
		setup       func(controller *gomock.Controller) *deps
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when client fails to get status",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					client.EXPECT().
						GetStatus(gomock.Any(), CmpMatcher(t, &pb.GetStatusRequest{}, protocmp.Transform())).
						Return(nil, errors.New("failed")),
				)
				return &deps{
					lvmctrld:   client,
					volumeLock: locker,
					fsRegistry: fsRegistry,
				}
			},
			true,
			codes.Unknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			deps := tt.setup(mockCtrl)
			got, err := driverd.NewNodeServer(
				deps.lvmctrld,
				deps.volumeLock,
				deps.fsRegistry,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewNodeServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("NewNodeServer() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if err == nil && got == nil {
				t.Errorf("MewNodeServer() got = %v, want non nil", got)
				return
			}
		})
	}
}

func Test_nodeServer_NodeStageVolume(t *testing.T) {
	req := &csi.NodeStageVolumeRequest{
		VolumeId:          "volume1@vg00",
		StagingTargetPath: "/staging/path",
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{
					VolumeMountGroup: "1000",
					MountFlags:       []string{"noatime"},
				},
			},
		},
	}
	type deps struct {
		lvmctrld     pb.LvmCtrldClient
		volumeLocker driverd.VolumeLocker
		fsRegistry   driverd.FileSystemRegistry
	}
	type args struct {
		ctx context.Context
		req *csi.NodeStageVolumeRequest
	}
	tests := []struct {
		name        string
		setup       func(controller *gomock.Controller) *deps
		args        args
		want        *csi.NodeStageVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when VolumeId is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				prototest.Without(req, "VolumeId"),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when VolumeCapability is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				prototest.Without(req, "VolumeCapability"),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when StagingTargetPath is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				prototest.Without(req, "StagingTargetPath"),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when volume lock fails",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLockVolume(t, locker, *MustVolumeRefFromID("volume1@vg00"), "stage", status.Error(codes.Internal, "internal error")),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when lvs fails (not found)",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLockVolume(t, locker, *MustVolumeRefFromID("volume1@vg00"), "stage", nil),
					expectLvs(t, client, "vg00/volume1", nil, status.Error(codes.NotFound, "not found")),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Should fail when filesystem doesn't accept access type",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLockVolume(t, locker, *MustVolumeRefFromID("volume1@vg00"), "stage", nil),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=raw"}, nil),
					expectGetFileSystem(t, fsRegistry, "raw", fs, nil),
					expectFsAccepts(t, fs, driverd.MountAccessType, false),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should stage the volume",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				fs := mock.NewMockFileSystem(controller)
				testGid := 1000
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLockVolume(t, locker, *MustVolumeRefFromID("volume1@vg00"), "stage", nil),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=ext4"}, nil),
					expectGetFileSystem(t, fsRegistry, "ext4", fs, nil),
					expectFsAccepts(t, fs, driverd.MountAccessType, true),
					expectStage(t, fs, "/dev/vg00/volume1", "/staging/path", []string{"noatime"}, &testGid, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			&csi.NodeStageVolumeResponse{},
			false,
			codes.OK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			fields := tt.setup(mockCtrl)
			ns, _ := driverd.NewNodeServer(
				fields.lvmctrld,
				fields.volumeLocker,
				fields.fsRegistry,
			)
			got, err := ns.NodeStageVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeStageVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("NodeStageVolume() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeStageVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeServer_NodeUnstageVolume(t *testing.T) {
	type deps struct {
		lvmctrld     pb.LvmCtrldClient
		volumeLocker driverd.VolumeLocker
		fsRegistry   driverd.FileSystemRegistry
	}
	type args struct {
		ctx context.Context
		req *csi.NodeUnstageVolumeRequest
	}
	tests := []struct {
		name        string
		setup       func(controller *gomock.Controller) *deps
		args        args
		want        *csi.NodeUnstageVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when VolumeId is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &deps{
					lvmctrld: client,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					StagingTargetPath: "/staging/path",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when StagingTargetPath is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &deps{
					lvmctrld: client,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId: "volume1@vg00",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when volume unlock fails",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=ext4"}, nil),
					expectGetFileSystem(t, fsRegistry, "ext4", fs, nil),
					expectUnstage(t, fs, "/staging/path", nil),
					expectUnlockVolume(t, locker, *MustVolumeRefFromID("volume1@vg00"), "stage", status.Error(codes.Internal, "internal error")),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId:          "volume1@vg00",
					StagingTargetPath: "/staging/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should unstage the volume",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=ext4"}, nil),
					expectGetFileSystem(t, fsRegistry, "ext4", fs, nil),
					expectUnstage(t, fs, "/staging/path", nil),
					expectUnlockVolume(t, locker, *MustVolumeRefFromID("volume1@vg00"), "stage", nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId:          "volume1@vg00",
					StagingTargetPath: "/staging/path",
				},
			},
			&csi.NodeUnstageVolumeResponse{},
			false,
			codes.OK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			fields := tt.setup(mockCtrl)
			ns, _ := driverd.NewNodeServer(
				fields.lvmctrld,
				fields.volumeLocker,
				fields.fsRegistry,
			)
			got, err := ns.NodeUnstageVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeUnstageVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("NodeUnstageVolume() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeUnstageVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeServer_NodePublishVolume(t *testing.T) {
	req := &csi.NodePublishVolumeRequest{
		VolumeId:          "volume1@vg00",
		StagingTargetPath: "/staging/path",
		TargetPath:        "/target/path",
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
			AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
		},
	}
	type deps struct {
		lvmctrld     pb.LvmCtrldClient
		volumeLocker driverd.VolumeLocker
		fsRegistry   driverd.FileSystemRegistry
	}
	type args struct {
		ctx context.Context
		req *csi.NodePublishVolumeRequest
	}
	tests := []struct {
		name        string
		setup       func(controller *gomock.Controller) *deps
		args        args
		want        *csi.NodePublishVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when VolumeId is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				prototest.Without(req, "VolumeId"),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when VolumeCapability is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				prototest.Without(req, "VolumeCapability"),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when TargetPath is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				prototest.Without(req, "TargetPath"),
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when Lvs fails",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", nil, status.Error(codes.Internal, "internal error")),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem not found (not found error)",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", nil, status.Error(codes.NotFound, "not found")),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Should fail when multiple filesystem tags",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=raw", "csi-sanlock-lvm.vleo.net/fs=ext4"}, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem tag is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/no=fs"}, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem name is invalid",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=unknown"}, nil),
					expectGetFileSystem(t, fsRegistry, "unknown", nil, errors.New("unknown filesystem")),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem doesn't accept access type",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=&24raw"}, nil),
					expectGetFileSystem(t, fsRegistry, "$raw", fs, nil),
					expectFsAccepts(t, fs, driverd.MountAccessType, false),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should mount read-only raw volume",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=&24raw"}, nil),
					expectGetFileSystem(t, fsRegistry, "$raw", fs, nil),
					expectFsAccepts(t, fs, driverd.BlockAccessType, true),
					expectPublish(t, fs, "/dev/vg00/volume1", "/staging/path", "/target/path", true, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				prototest.Merge(req,
					&csi.NodePublishVolumeRequest{
						VolumeCapability: &csi.VolumeCapability{
							AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
						},
						Readonly: true,
					},
				),
			},
			&csi.NodePublishVolumeResponse{},
			false,
			codes.OK,
		},
		{
			"Should mount read-write raw volume",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=&24raw"}, nil),
					expectGetFileSystem(t, fsRegistry, "$raw", fs, nil),
					expectFsAccepts(t, fs, driverd.BlockAccessType, true),
					expectPublish(t, fs, "/dev/vg00/volume1", "/staging/path", "/target/path", false, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				prototest.Merge(req,
					&csi.NodePublishVolumeRequest{
						VolumeCapability: &csi.VolumeCapability{
							AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
						},
					},
				),
			},
			&csi.NodePublishVolumeResponse{},
			false,
			codes.OK,
		},
		{
			"Should mount read-only filesystem volume",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=ext4"}, nil),
					expectGetFileSystem(t, fsRegistry, "ext4", fs, nil),
					expectFsAccepts(t, fs, driverd.MountAccessType, true),
					expectPublish(t, fs, "/dev/vg00/volume1", "/staging/path", "/target/path", true, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				prototest.Merge(req,
					&csi.NodePublishVolumeRequest{
						Readonly: true,
					},
				),
			},
			&csi.NodePublishVolumeResponse{},
			false,
			codes.OK,
		},
		{
			"Should mount read-write filesystem volume",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=ext4"}, nil),
					expectGetFileSystem(t, fsRegistry, "ext4", fs, nil),
					expectFsAccepts(t, fs, driverd.MountAccessType, true),
					expectPublish(t, fs, "/dev/vg00/volume1", "/staging/path", "/target/path", false, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				req,
			},
			&csi.NodePublishVolumeResponse{},
			false,
			codes.OK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			fields := tt.setup(mockCtrl)
			ns, _ := driverd.NewNodeServer(
				fields.lvmctrld,
				fields.volumeLocker,
				fields.fsRegistry,
			)
			got, err := ns.NodePublishVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodePublishVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("NodePublishVolume() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodePublishVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeServer_NodeUnpublishVolume(t *testing.T) {
	type deps struct {
		lvmctrld     pb.LvmCtrldClient
		volumeLocker driverd.VolumeLocker
		fsRegistry   driverd.FileSystemRegistry
	}
	type args struct {
		ctx context.Context
		req *csi.NodeUnpublishVolumeRequest
	}
	tests := []struct {
		name        string
		setup       func(controller *gomock.Controller) *deps
		args        args
		want        *csi.NodeUnpublishVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when VolumeId is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &deps{
					lvmctrld: client,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "",
					TargetPath: "targetPath1",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when TargetPath is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &deps{
					lvmctrld: client,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId: "volume1",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when Lvs fails",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{}, status.Error(codes.Internal, "internal error")),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "volume1@vg00",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem not found (not found error)",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=raw"}, status.Errorf(codes.NotFound, "error")),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "volume1@vg00",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Should fail when multiple filesystem tags",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=raw", "csi-sanlock-lvm.vleo.net/fs=ext4"}, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "volume1@vg00",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem tag is missing",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{}, nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "volume1@vg00",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem name is invalid",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=unknown"}, nil),
					expectGetFileSystem(t, fsRegistry, "unknown", nil, errors.New("unknown filesystem")),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "volume1@vg00",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should unmount volume",
			func(controller *gomock.Controller) *deps {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLvs(t, client, "vg00/volume1", []string{"csi-sanlock-lvm.vleo.net/fs=raw"}, nil),
					expectGetFileSystem(t, fsRegistry, "raw", fs, nil),
					expectUnpublish(t, fs, "/target/path", nil),
				)
				return &deps{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "volume1@vg00",
					TargetPath: "/target/path",
				},
			},
			&csi.NodeUnpublishVolumeResponse{},
			false,
			codes.OK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			fields := tt.setup(mockCtrl)
			ns, _ := driverd.NewNodeServer(
				fields.lvmctrld,
				fields.volumeLocker,
				fields.fsRegistry,
			)
			got, err := ns.NodeUnpublishVolume(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeUnpublishVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("NodeUnpublishVolume() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NodeUnpublishVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func MustVolumeRefFromID(volumeID string) *driverd.VolumeRef {
	id, err := driverd.NewVolumeRefFromID(volumeID)
	if err != nil {
		panic(err)
	}
	return id
}

func expectLockVolume(_ *testing.T, locker *mock.MockVolumeLocker, ref driverd.VolumeRef, opRe string, err error) *gomock.Call {
	return locker.EXPECT().
		LockVolume(gomock.Any(), ref, matchers.Regexp(opRe)).
		Return(err)
}

func expectUnlockVolume(_ *testing.T, locker *mock.MockVolumeLocker, ref driverd.VolumeRef, opRe string, err error) *gomock.Call {
	return locker.EXPECT().
		UnlockVolume(gomock.Any(), ref, matchers.Regexp(opRe)).
		Return(err)
}

func expectLvs(t *testing.T, client *mock.MockLvmCtrldClient, target string, tags []string, err error) *gomock.Call {
	return client.EXPECT().
		Lvs(
			gomock.Any(),
			CmpMatcher(t, &pb.LvsRequest{Target: []string{target}}, protocmp.Transform()),
			gomock.Any(),
		).
		Return(
			&pb.LvsResponse{Lvs: []*pb.LogicalVolume{{LvTags: tags}}},
			err,
		)
}

func expectGetFileSystem(_ *testing.T, fsRegistry *mock.MockFileSystemRegistry, fsName string, fs *mock.MockFileSystem, err error) *gomock.Call {
	return fsRegistry.EXPECT().
		GetFileSystem(fsName).Return(fs, err)
}

func expectFsAccepts(_ *testing.T, fs *mock.MockFileSystem, accessType driverd.VolumeAccessType, ret bool) *gomock.Call {
	return fs.EXPECT().
		Accepts(accessType).
		Return(ret)
}

func expectStage(_ *testing.T, fs *mock.MockFileSystem, device string, stagePoint string, flags []string, grpID *int, err error) *gomock.Call {
	return fs.EXPECT().
		Stage(device, stagePoint, flags, grpID).Return(err)
}

func expectUnstage(_ *testing.T, fs *mock.MockFileSystem, stagePoint string, err error) *gomock.Call {
	return fs.EXPECT().
		Unstage(stagePoint).Return(err)
}

func expectPublish(_ *testing.T, fs *mock.MockFileSystem, device string, stagePoint string, mountPoint string, readOnly bool, err error) *gomock.Call {
	return fs.EXPECT().
		Publish(device, stagePoint, mountPoint, readOnly).Return(err)
}

func expectUnpublish(_ *testing.T, fs *mock.MockFileSystem, mountPoint string, err error) *gomock.Call {
	return fs.EXPECT().
		Unpublish(mountPoint).Return(err)
}
