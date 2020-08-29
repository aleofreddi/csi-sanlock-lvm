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

func Test_controllerServer_ListVolumes(t *testing.T) {
	type fields struct {
		nodeId                string
		lvmctrldAddr          string
		lvmctrldClientFactory pkg.LvmCtrldClientFactory
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Select: "lv_role!=snapshot", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
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
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Select: "lv_role!=snapshot", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
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
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
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
					{Volume: &csi.Volume{VolumeId: "vg1/lv1"}},
					{Volume: &csi.Volume{VolumeId: "vg1/lv2"}},
					{Volume: &csi.Volume{VolumeId: "vg2/lv1"}},
				},
				NextToken: "vg2/lv2",
			},
			false,
			codes.OK,
		},
		{
			"Should paginate results when starting token",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Select: "lv_role!=snapshot", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
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
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.ListVolumesRequest{
					StartingToken: "vg2/lv3",
					MaxEntries:    3,
				},
			},
			&csi.ListVolumesResponse{
				Entries: []*csi.ListVolumesResponse_Entry{
					{Volume: &csi.Volume{VolumeId: "vg2/lv3"}},
					{Volume: &csi.Volume{VolumeId: "vg2/lv4"}},
					{Volume: &csi.Volume{VolumeId: "vg2/lv5"}},
				},
				NextToken: "vg2/lv6",
			},
			false,
			codes.OK,
		},
		{
			"Should paginate results when starting token at last page",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Select: "lv_role!=snapshot", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
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
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.ListVolumesRequest{
					StartingToken: "vg2/lv5",
					MaxEntries:    3,
				},
			},
			&csi.ListVolumesResponse{
				Entries: []*csi.ListVolumesResponse_Entry{
					{Volume: &csi.Volume{VolumeId: "vg2/lv5"}},
					{Volume: &csi.Volume{VolumeId: "vg2/lv6"}},
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
			ns, _ := pkg.NewControllerServer(fields.nodeId, fields.lvmctrldAddr, fields.lvmctrldClientFactory)
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
		nodeId                string
		lvmctrldAddr          string
		lvmctrldClientFactory pkg.LvmCtrldClientFactory
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Select: "lv_role=snapshot", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
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
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
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
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Select: "lv_role=snapshot", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
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
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
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
					{Snapshot: &csi.Snapshot{SnapshotId: "vg1/lv1", SourceVolumeId: "vg1/lv0", ReadyToUse: true}},
					{Snapshot: &csi.Snapshot{SnapshotId: "vg1/lv2", SourceVolumeId: "vg1/lv0", ReadyToUse: true}},
					{Snapshot: &csi.Snapshot{SnapshotId: "vg2/lv1", SourceVolumeId: "vg2/lv0", ReadyToUse: true}},
				},
				NextToken: "vg2/lv2",
			},
			false,
			codes.OK,
		},
		{
			"Should paginate results when starting token",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Select: "lv_role=snapshot", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
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
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.ListSnapshotsRequest{
					StartingToken: "vg2/lv3",
					MaxEntries:    3,
				},
			},
			&csi.ListSnapshotsResponse{
				Entries: []*csi.ListSnapshotsResponse_Entry{
					{Snapshot: &csi.Snapshot{SnapshotId: "vg2/lv3", SourceVolumeId: "vg2/lv0", ReadyToUse: true}},
					{Snapshot: &csi.Snapshot{SnapshotId: "vg2/lv4", SourceVolumeId: "vg2/lv0", ReadyToUse: true}},
					{Snapshot: &csi.Snapshot{SnapshotId: "vg2/lv5", SourceVolumeId: "vg2/lv0", ReadyToUse: true}},
				},
				NextToken: "vg2/lv6",
			},
			false,
			codes.OK,
		},
		{
			"Should paginate results when starting token at last page",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				factory := mock.NewMockLvmCtrldClientFactory(controller)
				gomock.InOrder(
					factory.EXPECT().
						NewLocal().Return(&pkg.LvmCtrldClientConnection{LvmCtrldClient: client}, nil),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Select: "lv_role=snapshot", Sort: []string{"vg_name", "lv_name"}}, protocmp.Transform()),
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
					nodeId:                "node",
					lvmctrldAddr:          "addr",
					lvmctrldClientFactory: factory,
				}
			},
			args{
				context.Background(),
				&csi.ListSnapshotsRequest{
					StartingToken: "vg2/lv5",
					MaxEntries:    3,
				},
			},
			&csi.ListSnapshotsResponse{
				Entries: []*csi.ListSnapshotsResponse_Entry{
					{Snapshot: &csi.Snapshot{SnapshotId: "vg2/lv5", SourceVolumeId: "vg2/lv0", ReadyToUse: true}},
					{Snapshot: &csi.Snapshot{SnapshotId: "vg2/lv6", SourceVolumeId: "vg2/lv0", ReadyToUse: true}},
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
			ns, _ := pkg.NewControllerServer(fields.nodeId, fields.lvmctrldAddr, fields.lvmctrldClientFactory)
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
