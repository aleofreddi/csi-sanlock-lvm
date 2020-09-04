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
	"errors"
	"reflect"
	"testing"

	mock "github.com/aleofreddi/csi-sanlock-lvm/driverd/mock"
	pkg "github.com/aleofreddi/csi-sanlock-lvm/driverd/pkg"
	"github.com/aleofreddi/csi-sanlock-lvm/lvmctrld/proto"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
)

func Test_nodeServer_NodeStageVolume(t *testing.T) {
	type fields struct {
		nodeId                string
		lvmctrldAddr          string
		lvmctrldClientFactory pkg.LvmCtrldClientFactory
		newFileSystem         pkg.FileSystemFactory
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:          "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:         "vg00/volume1",
					VolumeCapability: &csi.VolumeCapability{},
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when volume activation fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1", Activate: proto.LvActivationMode_ACTIVE_EXCLUSIVE}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:          "vg00/volume1",
					StagingTargetPath: "/staging/path",
					VolumeCapability:  &csi.VolumeCapability{},
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when lvs fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1", Activate: proto.LvActivationMode_ACTIVE_EXCLUSIVE}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:          "vg00/volume1",
					StagingTargetPath: "/staging/path",
					VolumeCapability:  &csi.VolumeCapability{},
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when update tag fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1", Activate: proto.LvActivationMode_ACTIVE_EXCLUSIVE}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{
									"ignore_invalid_tags_&",
									"csi&2dsanlock&2dlvm.vleo.net&2fowner=node1&40addr",
									"csi&2dsanlock&2dlvm.vleo.net&2fowner=node&40addr",
									"csi&2dsanlock&2dlvm.vleo.net&2fowner=node2&40addr",
								}}}},
								nil,
							),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1",
									AddTag: []string{"csi&2dsanlock&2dlvm.vleo.net&2fowner=node&40addr"},
									DelTag: []string{"csi&2dsanlock&2dlvm.vleo.net&2fowner=node1&40addr", "csi&2dsanlock&2dlvm.vleo.net&2fowner=node2&40addr"},
								}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:          "vg00/volume1",
					StagingTargetPath: "/staging/path",
					VolumeCapability:  &csi.VolumeCapability{},
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should activate the volume and set owner tag",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1", Activate: proto.LvActivationMode_ACTIVE_EXCLUSIVE}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{
									"ignore_invalid_tags_&",
									"csi&2dsanlock&2dlvm.vleo.net&2fowner=node1&40addr",
									"csi&2dsanlock&2dlvm.vleo.net&2fowner=node&40addr",
									"csi&2dsanlock&2dlvm.vleo.net&2fowner=node2&40addr",
								}}}},
								nil,
							),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1",
									AddTag: []string{"csi&2dsanlock&2dlvm.vleo.net&2fowner=node&40addr"},
									DelTag: []string{"csi&2dsanlock&2dlvm.vleo.net&2fowner=node1&40addr", "csi&2dsanlock&2dlvm.vleo.net&2fowner=node2&40addr"},
								}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeStageVolumeRequest{
					VolumeId:          "vg00/volume1",
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
			ns, _ := pkg.NewNodeServer(fields.nodeId, fields.lvmctrldAddr, fields.lvmctrldClientFactory, fields.newFileSystem)
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
		nodeId                string
		lvmctrldAddr          string
		lvmctrldClientFactory pkg.LvmCtrldClientFactory
		newFileSystem         pkg.FileSystemFactory
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId: "vg00/volume1",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when volume deactivation fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1", Activate: proto.LvActivationMode_DEACTIVATE}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId:          "vg00/volume1",
					StagingTargetPath: "/staging/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when update tag fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1", Activate: proto.LvActivationMode_DEACTIVATE}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1",
									DelTag: []string{"csi&2dsanlock&2dlvm.vleo.net&2fowner=node&40addr"},
								}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId:          "vg00/volume1",
					StagingTargetPath: "/staging/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should deactivate the volume and remove owner tag",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1", Activate: proto.LvActivationMode_DEACTIVATE}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: "vg00/volume1",
									DelTag: []string{"csi&2dsanlock&2dlvm.vleo.net&2fowner=node&40addr"},
								}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnstageVolumeRequest{
					VolumeId:          "vg00/volume1",
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
			ns, _ := pkg.NewNodeServer(fields.nodeId, fields.lvmctrldAddr, fields.lvmctrldClientFactory, fields.newFileSystem)
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
		nodeId                string
		lvmctrldAddr          string
		lvmctrldClientFactory pkg.LvmCtrldClientFactory
		newFileSystem         pkg.FileSystemFactory
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
				return &fields{
					nodeId:       "node",
					lvmctrldAddr: "addr",
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
				return &fields{
					nodeId:       "node",
					lvmctrldAddr: "addr",
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId: "volume1",
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
				return &fields{
					nodeId:       "node",
					lvmctrldAddr: "addr",
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId: "volume1",
				},
			},
			nil,
			true,
			codes.InvalidArgument,
		},
		{
			"Should fail when TargetPath is missing",
			func(controller *gomock.Controller) *fields {
				return &fields{
					nodeId:       "node",
					lvmctrldAddr: "addr",
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId: "volume1",
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
			"Should fail when fails to get lvmctrld client",
			func(controller *gomock.Controller) *fields {
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(nil, errors.New("failed")),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
			"Should fail when Lvs fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.NotFound, "not found"),
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
			"Should fail when filesystem not found (empty results)",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{}},
								nil,
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi&2dsanlock&2dlvm.vleo.net&2ffs=raw", "csi&2dsanlock&2dlvm.vleo.net&2ffs=ext4"}}}},
								nil,
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{}}},
								nil,
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi&2dsanlock&2dlvm.vleo.net&2ffs=raw"}}}},
								nil,
							),
					filesystemFactory.EXPECT().
						New("raw").Return(nil, errors.New("unknown filesystem")),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystem := mock.NewMockFileSystem(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi&2dsanlock&2dlvm.vleo.net&2ffs=raw"}}}},
								nil,
							),
					filesystemFactory.EXPECT().
						New("raw").Return(filesystem, nil),
					filesystem.EXPECT().
						Accepts(pkg.BlockAccessType).
						Return(false),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystem := mock.NewMockFileSystem(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi&2dsanlock&2dlvm.vleo.net&2ffs=raw"}}}},
								nil,
							),
					filesystemFactory.EXPECT().
						New("raw").Return(filesystem, nil),
					filesystem.EXPECT().
						Accepts(pkg.BlockAccessType).
						Return(true),
					filesystem.EXPECT().
						Mount("/dev/vg00/volume1", "/target/path", []string{}).
						Return(nil),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystem := mock.NewMockFileSystem(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi&2dsanlock&2dlvm.vleo.net&2ffs=ext4"}}}},
								nil,
							),
					filesystemFactory.EXPECT().
						New("ext4").Return(filesystem, nil),
					filesystem.EXPECT().
						Accepts(pkg.MountAccessType).
						Return(true),
					filesystem.EXPECT().
						Mount("/dev/vg00/volume1", "/target/path", []string{"ro"}).
						Return(nil),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystem := mock.NewMockFileSystem(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi&2dsanlock&2dlvm.vleo.net&2ffs=ext4"}}}},
								nil,
							),
					filesystemFactory.EXPECT().
						New("ext4").Return(filesystem, nil),
					filesystem.EXPECT().
						Accepts(pkg.MountAccessType).
						Return(true),
					filesystem.EXPECT().
						Mount("/dev/vg00/volume1", "/target/path", []string{"noatime", "rw"}).
						Return(nil),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodePublishVolumeRequest{
					VolumeId:   "vg00/volume1",
					TargetPath: "/target/path",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{MountFlags: []string{"noatime"}}},
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
			ns, _ := pkg.NewNodeServer(fields.nodeId, fields.lvmctrldAddr, fields.lvmctrldClientFactory, fields.newFileSystem)
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
		nodeId                string
		lvmctrldAddr          string
		lvmctrldClientFactory pkg.LvmCtrldClientFactory
		newFileSystem         pkg.FileSystemFactory
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
				return &fields{
					nodeId:       "node",
					lvmctrldAddr: "addr",
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
				return &fields{
					nodeId:       "node",
					lvmctrldAddr: "addr",
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
			"Should fail when fails to get lvmctrld client",
			func(controller *gomock.Controller) *fields {
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(nil, errors.New("failed")),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "vg00/volume1",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.Internal,
		},
		{
			"Should fail when Lvs fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.NotFound, "not found"),
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "vg00/volume1",
					TargetPath: "/target/path",
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Should fail when filesystem not found (empty results)",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{}},
								nil,
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi&2dsanlock&2dlvm.vleo.net&2ffs=raw", "csi&2dsanlock&2dlvm.vleo.net&2ffs=ext4"}}}},
								nil,
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{}}},
								nil,
							),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi&2dsanlock&2dlvm.vleo.net&2ffs=raw"}}}},
								nil,
							),
					filesystemFactory.EXPECT().
						New("raw").Return(nil, errors.New("unknown filesystem")),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				filesystem := mock.NewMockFileSystem(controller)
				filesystemFactory := mock.NewMockFileSystemFactoryInterface(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: "vg00/volume1", Select: "lv_role!=snapshot"}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{LvTags: []string{"csi&2dsanlock&2dlvm.vleo.net&2ffs=raw"}}}},
								nil,
							),
					filesystemFactory.EXPECT().
						New("raw").Return(filesystem, nil),
					filesystem.EXPECT().
						Umount("/target/path").
						Return(nil),
				)
				return &fields{
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
					newFileSystem:         filesystemFactory.New,
				}
			},
			args{
				context.Background(),
				&csi.NodeUnpublishVolumeRequest{
					VolumeId:   "vg00/volume1",
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
			ns, _ := pkg.NewNodeServer(fields.nodeId, fields.lvmctrldAddr, fields.lvmctrldClientFactory, fields.newFileSystem)
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