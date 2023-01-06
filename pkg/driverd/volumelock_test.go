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

package driverd_test

import (
	"context"
	"testing"

	pkg "github.com/aleofreddi/csi-sanlock-lvm/pkg/driverd"
	"github.com/aleofreddi/csi-sanlock-lvm/pkg/mock"
	"github.com/aleofreddi/csi-sanlock-lvm/pkg/proto"
	"github.com/golang/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
)

func Test_volumeLocker_LockVolume(t *testing.T) {
	type fields struct {
		lvmctrld proto.LvmCtrldClient
		nodeName string
	}
	type args struct {
		ctx context.Context
		vol pkg.VolumeRef
		op  string
	}
	tests := []struct {
		name        string
		fields      func(controller *gomock.Controller) *fields
		args        args
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Should fail when volume activation fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLockerSyncLvs(t, client),

					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{
									Target:   []string{"vg00/volume1"},
									Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE,
								}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
				)
				return &fields{
					lvmctrld: client,
					nodeName: "node1",
				}
			},
			args{
				context.Background(),
				*MustVolumeRefFromID("volume1@vg00"),
				"",
			},
			true,
			codes.Internal,
		},
		{
			"Should fail when update tag fails",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLockerSyncLvs(t, client),

					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: []string{"vg00/volume1"}, Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/volume1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{
									VgName: "vg00",
									LvName: "volume1",
									LvTags: []string{
										"ignore_invalid_tags_&",
										"csi-sanlock-lvm.vleo.net/ownerId=4321",
										"csi-sanlock-lvm.vleo.net/ownerNode=node2",
									},
								}}},
								nil,
							),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{
									Target: []string{"vg00/volume1"},
									AddTag: []string{"csi-sanlock-lvm.vleo.net/ownerId=1234", "csi-sanlock-lvm.vleo.net/ownerNode=node1"},
									DelTag: []string{"csi-sanlock-lvm.vleo.net/ownerId=4321", "csi-sanlock-lvm.vleo.net/ownerNode=node2"},
								}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								nil,
								status.Error(codes.Internal, "internal error"),
							),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: []string{"vg00/volume1"}, Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_DEACTIVATE}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
				)
				return &fields{
					lvmctrld: client,
					nodeName: "node1",
				}
			},
			args{
				context.Background(),
				*MustVolumeRefFromID("volume1@vg00"),
				"",
			},
			true,
			codes.Internal,
		},
		{
			"Should activate the volume and set owner tag",
			func(controller *gomock.Controller) *fields {
				client := mock.NewMockLvmCtrldClient(controller)
				gomock.InOrder(
					expectGetStatus(t, client),
					expectLockerSyncLvs(t, client),

					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{Target: []string{"vg00/volume1"}, Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
					client.EXPECT().
							Lvs(
								gomock.Any(),
								CmpMatcher(t, &proto.LvsRequest{Target: []string{"vg00/volume1"}}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvsResponse{Lvs: []*proto.LogicalVolume{{
									VgName: "vg00",
									LvName: "volume1",
									LvTags: []string{
										"ignore_invalid_tags_&",
										"csi-sanlock-lvm.vleo.net/ownerId=4321",
										"csi-sanlock-lvm.vleo.net/ownerNode=node2",
									}}},
								},
								nil,
							),
					client.EXPECT().
							LvChange(gomock.Any(),
								CmpMatcher(t, &proto.LvChangeRequest{
									Target: []string{"vg00/volume1"},
									AddTag: []string{"csi-sanlock-lvm.vleo.net/ownerId=1234", "csi-sanlock-lvm.vleo.net/ownerNode=node1"},
									DelTag: []string{"csi-sanlock-lvm.vleo.net/ownerId=4321", "csi-sanlock-lvm.vleo.net/ownerNode=node2"},
								}, protocmp.Transform()),
								gomock.Any(),
							).
							Return(
								&proto.LvChangeResponse{},
								nil,
							),
				)
				return &fields{
					lvmctrld: client,
					nodeName: "node1",
				}
			},
			args{
				context.Background(),
				*MustVolumeRefFromID("volume1@vg00"),
				"",
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
			vl, _ := pkg.NewVolumeLocker(
				fields.lvmctrld,
				fields.nodeName,
			)
			if err := vl.Start(tt.args.ctx); err != nil {
				t.Fatalf("Start() error = %v, want none", err)
			}
			err := vl.LockVolume(
				tt.args.ctx,
				tt.args.vol,
				tt.args.op,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("LockVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("LockVolume() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
		})
	}
}

func expectLockerSyncLvs(t *testing.T, client *mock.MockLvmCtrldClient) *gomock.Call {
	return client.EXPECT().
			Lvs(gomock.Any(), CmpMatcher(t, &proto.LvsRequest{
				Select: "lv_name=~^csl-v- && lv_active=active",
				Sort:   nil,
				Target: nil,
			}, protocmp.Transform())).
		Return(&proto.LvsResponse{}, nil)
}
