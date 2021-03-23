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
	"reflect"
	"testing"

	"github.com/aleofreddi/csi-sanlock-lvm/driverd/mock"
	pkg "github.com/aleofreddi/csi-sanlock-lvm/driverd/pkg"
	"github.com/aleofreddi/csi-sanlock-lvm/proto"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestNewNodeServer(t *testing.T) {
	type args struct {
		lvmctrld   proto.LvmCtrldClient
		volumeLock pkg.VolumeLocker
		fsRegistry pkg.FileSystemRegistry
	}
	tests := []struct {
		name        string
		args        func(controller *gomock.Controller) *args
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when client fails to get status",
			func(controller *gomock.Controller) *args {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					client.EXPECT().
						GetStatus(gomock.Any(), CmpMatcher(t, &proto.GetStatusRequest{}, protocmp.Transform())).
						Return(nil, errors.New("failed")),
				)
				return &args{
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
			args := tt.args(mockCtrl)
			got, err := pkg.NewNodeServer(
				args.lvmctrld,
				args.volumeLock,
				args.fsRegistry,
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
	type fields struct {
		lvmctrld     proto.LvmCtrldClient
		volumeLocker pkg.VolumeLocker
		fsRegistry   pkg.FileSystemRegistry
	}
	type args struct {
		ctx context.Context
		req *csi.NodeStageVolumeRequest
	}
	tests := []struct {
		name        string
		fields      func(controller *gomock.Controller) *fields
		args        args
		want        *csi.NodeStageVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when VolumeId is missing",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeCapability:  &csi.VolumeCapability{},
					StagingTargetPath: "/staging/path",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when VolumeCapability is missing",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:          "v:vg00:lv1",
					StagingTargetPath: "/staging/path",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when StagingTargetPath is missing",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:         "v:vg00:lv1",
					VolumeCapability: &csi.VolumeCapability{},
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when volume lock fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					locker.EXPECT().
							LockVolume(
								gomock.Any(),
								MustVolumeRefFromID("v:vg00:lv1"),
								"stage",
							).
						Return(status.Error(codes.Internal, "internal error")),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:          "v:vg00:lv1",
					StagingTargetPath: "/staging/path",
					VolumeCapability:  &csi.VolumeCapability{},
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should stage the volume",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					locker.EXPECT().
							LockVolume(
								gomock.Any(),
								MustVolumeRefFromID("v:vg00:lv1"),
								"stage",
							).
						Return(nil),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:          "v:vg00:lv1",
					StagingTargetPath: "/staging/path",
					VolumeCapability:  &csi.VolumeCapability{},
				},
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
			fields := tt.fields(mockCtrl)
			ns, _ := pkg.NewNodeServer(
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
	type fields struct {
		lvmctrld     proto.LvmCtrldClient
		volumeLocker pkg.VolumeLocker
		fsRegistry   pkg.FileSystemRegistry
	}
	type args struct {
		ctx context.Context
		req *csi.NodeUnstageVolumeRequest
	}
	tests := []struct {
		name        string
		fields      func(controller *gomock.Controller) *fields
		args        args
		want        *csi.NodeUnstageVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when VolumeId is missing",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
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
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
					lvmctrld: client,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId: "v:vg00:lv1",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when volume unlock fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					locker.EXPECT().
							UnlockVolume(
								gomock.Any(),
								MustVolumeRefFromID("v:vg00:lv1"),
								"stage",
							).
							Return(
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId:          "v:vg00:lv1",
					StagingTargetPath: "/staging/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should unstage the volume",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					locker.EXPECT().
							UnlockVolume(
								gomock.Any(),
								MustVolumeRefFromID("v:vg00:lv1"),
								"stage",
							).
						Return(nil),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId:          "v:vg00:lv1",
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
			fields := tt.fields(mockCtrl)
			ns, _ := pkg.NewNodeServer(
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
	type fields struct {
		lvmctrld     proto.LvmCtrldClient
		volumeLocker pkg.VolumeLocker
		fsRegistry   pkg.FileSystemRegistry
	}
	type args struct {
		ctx context.Context
		req *csi.NodePublishVolumeRequest
	}
	tests := []struct {
		name        string
		fields      func(controller *gomock.Controller) *fields
		args        args
		want        *csi.NodePublishVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when VolumeId is missing",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "",
					TargetPath: "targetPath1",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when both access types are specified",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId: "lv1",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when VolumeCapability is missing",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId: "lv1",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when TargetPath is missing",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId: "lv1",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when Lvs fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem not found (not found error)",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.NotFound, "not found"),
							),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Should fail when multiple filesystem tags",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi-sanlock-lvm.vleo.net/fs=raw", "csi-sanlock-lvm.vleo.net/fs=ext4"}}}},
								nil,
							),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem tag is missing",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{}}},
								nil,
							),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem name is invalid",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi-sanlock-lvm.vleo.net/fs=raw"}}}},
								nil,
							),

					fsRegistry.EXPECT().
						GetFileSystem("raw").Return(nil, errors.New("unknown filesystem")),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem doesn't accept access type",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi-sanlock-lvm.vleo.net/fs=raw"}}}},
								nil,
							),

					fsRegistry.EXPECT().
						GetFileSystem("raw").Return(fs, nil),
					fs.EXPECT().
						Accepts(pkg.BlockAccessType).
						Return(false),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should mount raw volume",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi-sanlock-lvm.vleo.net/fs=raw"}}}},
								nil,
							),

					fsRegistry.EXPECT().
						GetFileSystem("raw").Return(fs, nil),
					fs.EXPECT().
						Accepts(pkg.BlockAccessType).
						Return(true),
					fs.EXPECT().
						Mount("/dev/vg00/csl-v-lv1", "/target/path", []string{}).
						Return(nil),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
			},
			&csi.NodePublishVolumeResponse{},
			false,
			codes.OK,
		},
		{
			"Should mount read-only volume",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi-sanlock-lvm.vleo.net/fs=ext4"}}}},
								nil,
							),

					fsRegistry.EXPECT().
						GetFileSystem("ext4").Return(fs, nil),

					fs.EXPECT().
						Accepts(pkg.MountAccessType).
						Return(true),
					fs.EXPECT().
						Mount("/dev/vg00/csl-v-lv1", "/target/path", []string{"ro"}).
						Return(nil),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
					Readonly: true,
				},
			},
			&csi.NodePublishVolumeResponse{},
			false,
			codes.OK,
		},
		{
			"Should mount read-write volume with custom flags",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi-sanlock-lvm.vleo.net/fs=ext4"}}}},
								nil,
							),

					fsRegistry.EXPECT().
						GetFileSystem("ext4").Return(fs, nil),

					fs.EXPECT().
						Accepts(pkg.MountAccessType).
						Return(true),
					fs.EXPECT().
						Mount("/dev/vg00/csl-v-lv1", "/target/path", []string{"noatime", "rw"}).
						Return(nil),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								FsType:     "ext4",
								MountFlags: []string{"noatime"},
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
					},
				},
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
			fields := tt.fields(mockCtrl)
			ns, _ := pkg.NewNodeServer(
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
	type fields struct {
		lvmctrld     proto.LvmCtrldClient
		volumeLocker pkg.VolumeLocker
		fsRegistry   pkg.FileSystemRegistry
	}
	type args struct {
		ctx context.Context
		req *csi.NodeUnpublishVolumeRequest
	}
	tests := []struct {
		name        string
		fields      func(controller *gomock.Controller) *fields
		args        args
		want        *csi.NodeUnpublishVolumeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when VolumeId is missing",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
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
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
				)
				return &fields{
					lvmctrld: client,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId: "lv1",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when Lvs fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem not found (not found error)",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.NotFound, "not found"),
							),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Should fail when target not found (empty results)",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Errorf(codes.NotFound, "error"),
							),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Should fail when multiple filesystem tags",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi-sanlock-lvm.vleo.net/fs=raw", "csi-sanlock-lvm.vleo.net/fs=ext4"}}}},
								nil,
							),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem tag is missing",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{}}},
								nil,
							),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when filesystem name is invalid",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi-sanlock-lvm.vleo.net/fs=raw"}}}},
								nil,
							),

					fsRegistry.EXPECT().
						GetFileSystem("raw").Return(nil, errors.New("unknown filesystem")),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should unmount volume",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				locker := mock.NewMockVolumeLocker(controller)
				fs := mock.NewMockFileSystem(controller)
				fsRegistry := mock.NewMockFileSystemRegistry(controller)
				gomock.InOrder(
					expectGetStatus(t, client),

					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/csl-v-lv1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi-sanlock-lvm.vleo.net/fs=raw"}}}},
								nil,
							),

					fsRegistry.EXPECT().
						GetFileSystem("raw").Return(fs, nil),
					fs.EXPECT().
						Umount("/target/path").
						Return(nil),
				)
				return &fields{
					lvmctrld:     client,
					volumeLocker: locker,
					fsRegistry:   fsRegistry,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "v:vg00:lv1",
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
			fields := tt.fields(mockCtrl)
			ns, _ := pkg.NewNodeServer(
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

func MustVolumeRefFromID(volumeID string) pkg.VolumeRef {
	id, err := pkg.NewVolumeRefFromID(volumeID)
	if err != nil {
		panic(err)
	}
	return id
}
