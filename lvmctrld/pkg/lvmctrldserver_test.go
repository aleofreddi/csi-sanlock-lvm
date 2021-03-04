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

package lvmctrld

import (
	"context"
	"reflect"
	"testing"

	"github.com/aleofreddi/csi-sanlock-lvm/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/kylelemons/godebug/pretty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Test_lvmctrldServer_LvChange(t *testing.T) {
	type fields struct {
		cmd runner
	}
	type args struct {
		ctx context.Context
		req *proto.LvChangeRequest
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        *proto.LvChangeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Activate volume in shared mode",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvchange", []string{"-a", "sy", "vg01/lv_test"}, 0, "", "", nil}},
				},
			},
			args{
				nil,
				&proto.LvChangeRequest{
					Target:   []string{"vg01/lv_test"},
					Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_SHARED,
				},
			},
			&proto.LvChangeResponse{},
			false,
			codes.OK,
		},
		{
			"Activate volume in exclusive mode",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvchange", []string{"-a", "ey", "vg01/lv_test"}, 0, "", "", nil}},
				},
			},
			args{
				nil,
				&proto.LvChangeRequest{
					Target:   []string{"vg01/lv_test"},
					Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE,
				},
			},
			&proto.LvChangeResponse{},
			false,
			codes.OK,
		},
		{
			"Deactivate volume",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvchange", []string{"-a", "n", "vg01/lv_test"}, 0, "", "", nil}},
				},
			},
			args{
				nil,
				&proto.LvChangeRequest{
					Target:   []string{"vg01/lv_test"},
					Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_DEACTIVATE,
				},
			},
			&proto.LvChangeResponse{},
			false,
			codes.OK,
		},
		{
			"Filter select",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvchange", []string{"-a", "ey", "-S", "lv_size>0", "vg01/lv_test"}, 0, "", "", nil}},
				},
			},
			args{
				nil,
				&proto.LvChangeRequest{
					Target:   []string{"vg01/lv_test"},
					Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE,
					Select:   "lv_size>0",
				},
			},
			&proto.LvChangeResponse{},
			false,
			codes.OK,
		},
		{
			"Add tags",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvchange", []string{"--addtag", "tag1", "--addtag", "tag2", "vg01/lv_test"}, 0, "", "", nil}},
				},
			},
			args{
				nil,
				&proto.LvChangeRequest{
					Target: []string{"vg01/lv_test"},
					AddTag: []string{"tag1", "tag2"},
				},
			},
			&proto.LvChangeResponse{},
			false,
			codes.OK,
		},
		{
			"Delete tags",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvchange", []string{"--deltag", "tag1", "--deltag", "tag2", "vg01/lv_test"}, 0, "", "", nil}},
				},
			},
			args{
				nil,
				&proto.LvChangeRequest{
					Target: []string{"vg01/lv_test"},
					DelTag: []string{"tag1", "tag2"},
				},
			},
			&proto.LvChangeResponse{},
			false,
			codes.OK,
		},
		{
			"Fail when non existent volume group",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvchange", []string{"-a", "ey", "vg01/lv_test"}, 5, "", "  Volume group \"vg_test\" not found\n  Cannot process volume group vg_test", nil}},
				},
			},
			args{
				nil,
				&proto.LvChangeRequest{
					Target:   []string{"vg01/lv_test"},
					Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE,
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Fail when non existent logical volume",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvchange", []string{"-a", "ey", "vg01/lv_test"}, 5, "", "  Failed to find logical volume \"vg_test/lv_test\"", nil}},
				},
			},
			args{
				nil,
				&proto.LvChangeRequest{
					Target:   []string{"vg01/lv_test"},
					Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE,
				},
			},
			nil,
			true,
			codes.NotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := lvmctrldServer{
				cmd: tt.fields.cmd,
			}
			got, err := s.LvChange(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("LvChange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("LvChange() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LvChange() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lvmctrldServer_LvCreate(t *testing.T) {
	type fields struct {
		cmd runner
	}
	type args struct {
		in0 context.Context
		req *proto.LvCreateRequest
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        *proto.LvCreateResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Create logical volume",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvcreate", []string{"-y", "-L", "1048576b", "-a", "sy", "--addtag", "tag1", "--addtag", "tag2", "-n", "vg01/lv_test"}, 0, "", "", nil}},
				},
			},
			args{
				nil,
				&proto.LvCreateRequest{
					VgName:   "vg01",
					LvName:   "lv_test",
					Size:     1 << 20,
					Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_SHARED,
					LvTags:   []string{"tag1", "tag2"},
				},
			},
			&proto.LvCreateResponse{},
			false,
			codes.OK,
		},
		{
			"Create logical volume snapshot",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvcreate", []string{"-y", "-L", "1048576b", "-a", "n", "-s", "vg01/lv_test", "--addtag", "tag1", "--addtag", "tag2", "-n", "vg01/lv_snapshot"}, 0, "", "", nil}},
				},
			},
			args{
				nil,
				&proto.LvCreateRequest{
					VgName:   "vg01",
					LvName:   "lv_snapshot",
					Size:     1 << 20,
					Origin:   "vg01/lv_test",
					Activate: proto.LvActivationMode_LV_ACTIVATION_MODE_DEACTIVATE,
					LvTags:   []string{"tag1", "tag2"},
				},
			},
			&proto.LvCreateResponse{},
			false,
			codes.OK,
		},
		{
			"Fail when insufficient free space",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvcreate", []string{"-y", "-L", "1048576b", "--addtag", "tag1", "--addtag", "tag2", "-n", "vg01/lv_test"}, 5, "", "  Volume group \"vg01\" has insufficient free space (3839 extents): 25600 required.", nil}},
				},
			},
			args{
				nil,
				&proto.LvCreateRequest{
					VgName: "vg01",
					LvName: "lv_test",
					Size:   1 << 20,
					LvTags: []string{"tag1", "tag2"},
				},
			},
			nil,
			true,
			codes.OutOfRange,
		},
		{
			"Fail when invalid volume group",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvcreate", []string{"-y", "-L", "1048576b", "--addtag", "tag1", "--addtag", "tag2", "-n", "vg01/lv_test"}, 5, "", "  Volume group \"vg01\" not found\n  Cannot process volume group vg01", nil}},
				},
			},
			args{
				nil,
				&proto.LvCreateRequest{
					VgName: "vg01",
					LvName: "lv_test",
					Size:   1 << 20,
					LvTags: []string{"tag1", "tag2"},
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Fail when logical volume already exists",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvcreate", []string{"-y", "-L", "1048576b", "--addtag", "tag1", "--addtag", "tag2", "-n", "vg01/lv_test"}, 5, "", "  Logical Volume \"lv_test\" already exists in volume group \"vg01\"", nil}},
				},
			},
			args{
				nil,
				&proto.LvCreateRequest{
					VgName: "vg01",
					LvName: "lv_test",
					Size:   1 << 20,
					LvTags: []string{"tag1", "tag2"},
				},
			},
			nil,
			true,
			codes.AlreadyExists,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := lvmctrldServer{
				cmd: tt.fields.cmd,
			}
			got, err := s.LvCreate(tt.args.in0, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("LvCreate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("LvCreate() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LvCreate() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lvmctrldServer_LvRemove(t *testing.T) {
	type fields struct {
		cmd runner
	}
	type args struct {
		in0 context.Context
		req *proto.LvRemoveRequest
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        *proto.LvRemoveResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Remove logical volume",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvremove", []string{"-f", "-S", "lv_size>0", "vg01/lv_test"}, 0, "", "", nil}},
				},
			},
			args{
				nil,
				&proto.LvRemoveRequest{
					Target: []string{"vg01/lv_test"},
					Select: "lv_size>0",
				},
			},
			&proto.LvRemoveResponse{},
			false,
			codes.OK,
		},
		{
			"Fail when invalid logical volume",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvremove", []string{"-f", "vg01/lv_test"}, 5, "", "  Failed to find logical volume \"vg01/lv_test\"", nil}},
				},
			},
			args{
				nil,
				&proto.LvRemoveRequest{
					Target: []string{"vg01/lv_test"},
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Fail when invalid volume group",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvremove", []string{"-f", "vg01/lv_test"}, 5, "", "  Volume group \"vg01\" not found\n  Cannot process volume group vg01", nil}},
				},
			},
			args{
				nil,
				&proto.LvRemoveRequest{
					Target: []string{"vg01/lv_test"},
				},
			},
			nil,
			true,
			codes.NotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := lvmctrldServer{
				cmd: tt.fields.cmd,
			}
			got, err := s.LvRemove(tt.args.in0, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("LvRemove() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("LvRemove() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LvRemove() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lvmctrldServer_LvResize(t *testing.T) {
	type fields struct {
		cmd runner
	}
	type args struct {
		in0 context.Context
		req *proto.LvResizeRequest
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        *proto.LvResizeResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"Resize logical volume",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvresize", []string{"-f", "-L", "1048576b", "vg01/lv_test"}, 0, "", "", nil}},
				},
			},
			args{
				nil,
				&proto.LvResizeRequest{
					VgName: "vg01",
					LvName: "lv_test",
					Size:   1 << 20,
				},
			},
			&proto.LvResizeResponse{},
			false,
			codes.OK,
		},
		{
			"Fail when invalid volume group",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvresize", []string{"-f", "-L", "1048576b", "vg01/lv_test"}, 5, "", "  Volume group \"vg01\" not found\n  Cannot process volume group vg01", nil}},
				},
			},
			args{
				nil,
				&proto.LvResizeRequest{
					VgName: "vg01",
					LvName: "lv_test",
					Size:   1 << 20,
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Fail when invalid logical volume",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvresize", []string{"-f", "-L", "1048576b", "vg01/lv_test"}, 5, "", "  Logical volume lv_test not found in volume group vg01.", nil}},
				},
			},
			args{
				nil,
				&proto.LvResizeRequest{
					VgName: "vg01",
					LvName: "lv_test",
					Size:   1 << 20,
				},
			},
			nil,
			true,
			codes.NotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := lvmctrldServer{
				cmd: tt.fields.cmd,
			}
			got, err := s.LvResize(tt.args.in0, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("LvResize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("LvResize() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LvResize() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lvmctrldServer_Lvs(t *testing.T) {
	type fields struct {
		cmd runner
	}
	type args struct {
		in0 context.Context
		req *proto.LvsRequest
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        *proto.LvsResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"List all logical volumes",
			fields{
				&fakeRunner{
					t: t,
					executions: []fakeCommand{{"lvs", []string{"--options", "lv_name,vg_name,lv_attr,lv_size,pool_lv,origin,data_percent,metadata_percent,move_pv,mirror_log,copy_percent,convert_lv,lv_tags,lv_role,lv_time,lv_device_open", "--units", "b", "--nosuffix", "--reportformat", "json", "-S", "field=value", "-O", "field1,-field2", "vg01"}, 0,
						`
  {
      "report": [
          {
              "lv": [
                  {"lv_name":"lv1", "vg_name":"vg1", "lv_attr":"-wi-a-----", "lv_size":"33554432", "pool_lv":"", "origin":"", "data_percent":"", "metadata_percent":"", "move_pv":"", "mirror_log":"", "copy_percent":"", "convert_lv":"", "lv_tags":"", "lv_role":"public", "lv_time":"2020-02-27 20:57:35 +0000"},
                  {"lv_name":"lv2", "vg_name":"vg1", "lv_attr":"-wi-ao----", "lv_size":"4294967296", "pool_lv":"", "origin":"", "data_percent":"", "metadata_percent":"", "move_pv":"", "mirror_log":"", "copy_percent":"", "convert_lv":"", "lv_tags":"", "lv_role":"public", "lv_time":"2020-02-27 20:15:37 +0000"},
                  {"lv_name":"lv3", "vg_name":"vg2", "lv_attr":"-wi-a-----", "lv_size":"12582912", "pool_lv":"", "origin":"", "data_percent":"", "metadata_percent":"", "move_pv":"", "mirror_log":"", "copy_percent":"", "convert_lv":"", "lv_tags":"", "lv_role":"public", "lv_time":"2020-02-27 20:18:37 +0000"}
              ]
          }
      ]
  }
`, "", nil}}},
			},
			args{
				nil,
				&proto.LvsRequest{
					Target: []string{"vg01"},
					Select: "field=value",
					Sort:   []string{"field1", "-field2"},
				},
			},
			&proto.LvsResponse{
				Lvs: []*proto.LogicalVolume{
					{
						LvName: "lv1",
						VgName: "vg1",
						LvAttr: "-wi-a-----",
						LvSize: 33554432,
						LvTags: []string{},
						LvRole: []string{"public"},
						LvTime: &timestamp.Timestamp{Seconds: 1582837055},
					},
					{
						LvName: "lv2",
						VgName: "vg1",
						LvAttr: "-wi-ao----",
						LvSize: 4294967296,
						LvTags: []string{},
						LvRole: []string{"public"},
						LvTime: &timestamp.Timestamp{Seconds: 1582834537},
					},
					{
						LvName: "lv3",
						VgName: "vg2",
						LvAttr: "-wi-a-----",
						LvSize: 12582912,
						LvTags: []string{},
						LvRole: []string{"public"},
						LvTime: &timestamp.Timestamp{Seconds: 1582834717},
					},
				},
			},
			false,
			codes.OK,
		},
		{
			"Fail when invalid volume group",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvs", []string{"--options", "lv_name,vg_name,lv_attr,lv_size,pool_lv,origin,data_percent,metadata_percent,move_pv,mirror_log,copy_percent,convert_lv,lv_tags,lv_role,lv_time,lv_device_open", "--units", "b", "--nosuffix", "--reportformat", "json", "-S", "field=value", "vg01"}, 5, "", "  Volume group \"vg01\" not found\n  Cannot process volume group vg01", nil}},
				},
			},
			args{
				nil,
				&proto.LvsRequest{
					Target: []string{"vg01"},
					Select: "field=value",
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Fail when invalid logical volume",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvs", []string{"--options", "lv_name,vg_name,lv_attr,lv_size,pool_lv,origin,data_percent,metadata_percent,move_pv,mirror_log,copy_percent,convert_lv,lv_tags,lv_role,lv_time,lv_device_open", "--units", "b", "--nosuffix", "--reportformat", "json", "vg01/lv01"}, 5, "", "  Failed to find logical volume \"vg01/lv01\"", nil}},
				},
			},
			args{
				nil,
				&proto.LvsRequest{
					Target: []string{"vg01/lv01"},
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Fail when report contains invalid JSON",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvs", []string{"--options", "lv_name,vg_name,lv_attr,lv_size,pool_lv,origin,data_percent,metadata_percent,move_pv,mirror_log,copy_percent,convert_lv,lv_tags,lv_role,lv_time,lv_device_open", "--units", "b", "--nosuffix", "--reportformat", "json", "-S", "field=value", "vg01"}, 0, "{\"invalid\": \"json", "", nil}}},
			},
			args{
				nil,
				&proto.LvsRequest{
					Target: []string{"vg01"},
					Select: "field=value",
				},
			},
			nil,
			true,
			codes.Unknown,
		},
		{
			"Fail when report contain multiple report entries",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"lvs", []string{"--options", "lv_name,vg_name,lv_attr,lv_size,pool_lv,origin,data_percent,metadata_percent,move_pv,mirror_log,copy_percent,convert_lv,lv_tags,lv_role,lv_time,lv_device_open", "--units", "b", "--nosuffix", "--reportformat", "json", "-S", "field=value", "vg01"}, 0, `{ "report": [ {},{} ] }`, "", nil}}},
			},
			args{
				nil,
				&proto.LvsRequest{
					Target: []string{"vg01"},
					Select: "field=value",
				},
			},
			nil,
			true,
			codes.Unknown,
		},
		{
			"Fail when report contains invalid timestamp",
			fields{
				&fakeRunner{
					t: t,
					executions: []fakeCommand{{"lvs", []string{"--options", "lv_name,vg_name,lv_attr,lv_size,pool_lv,origin,data_percent,metadata_percent,move_pv,mirror_log,copy_percent,convert_lv,lv_tags,lv_role,lv_time,lv_device_open", "--units", "b", "--nosuffix", "--reportformat", "json", "-S", "field=value", "vg01"}, 0,
						`
  {
      "report": [
          {
              "lv": [
                  {"lv_name":"lv1", "vg_name":"vg1", "lv_attr":"-wi-a-----", "lv_size":"33554432", "pool_lv":"", "origin":"", "data_percent":"", "metadata_percent":"", "move_pv":"", "mirror_log":"", "copy_percent":"", "convert_lv":"", "lv_tags":"", "lv_role":"public", "lv_time":"2020-00-00 20:57:35 +0000"}
              ]
          }
      ]
  }
`, "", nil}}},
			},
			args{
				nil,
				&proto.LvsRequest{
					Target: []string{"vg01"},
					Select: "field=value",
				},
			},
			nil,
			true,
			codes.Unknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := lvmctrldServer{
				cmd: tt.fields.cmd,
			}
			got, err := s.Lvs(tt.args.in0, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Lvs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("Lvs() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Lvs() diff: %v", pretty.Compare(tt.want, got))
			}
		})
	}
}

func Test_lvmctrldServer_Vgs(t *testing.T) {
	type fields struct {
		cmd runner
	}
	type args struct {
		in0 context.Context
		req *proto.VgsRequest
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        *proto.VgsResponse
		wantErr     bool
		wantErrCode codes.Code
	}{
		{
			"List all volume groups",
			fields{
				&fakeRunner{
					t: t,
					executions: []fakeCommand{{
						"vgs", []string{"--options", "vg_name,pv_count,lv_count,snap_count,vg_attr,vg_size,vg_free,vg_tags", "--units", "b", "--nosuffix", "--reportformat", "json"}, 0, `
  {
      "report": [
          {
              "vg": [
                  {"vg_name":"vg01", "pv_count":"1", "lv_count":"2", "snap_count":"0", "vg_attr":"wz--n-", "vg_size":"20396900352", "vg_free":"16068378624", "vg_tags":""},
                  {"vg_name":"vg02", "pv_count":"1", "lv_count":"1", "snap_count":"0", "vg_attr":"wz--n-", "vg_size":"100663296", "vg_free":"88080384", "vg_tags":""}
              ]
          }
      ]
  }
`, "", nil}}},
			},
			args{
				nil,
				&proto.VgsRequest{},
			},
			&proto.VgsResponse{
				Vgs: []*proto.VolumeGroup{
					{
						VgName:    "vg01",
						PvCount:   1,
						LvCount:   2,
						SnapCount: 0,
						VgAttr:    "wz--n-",
						VgSize:    20396900352,
						VgFree:    16068378624,
						VgTags:    []string{},
					},
					{
						VgName:    "vg02",
						PvCount:   1,
						LvCount:   1,
						SnapCount: 0,
						VgAttr:    "wz--n-",
						VgSize:    100663296,
						VgFree:    88080384,
						VgTags:    []string{},
					},
				},
			},
			false,
			codes.OK,
		},
		{
			"Fail when invalid volume group",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"vgs", []string{"--options", "vg_name,pv_count,lv_count,snap_count,vg_attr,vg_size,vg_free,vg_tags", "--units", "b", "--nosuffix", "--reportformat", "json", "-S", "field=value", "vg01"}, 5, "", "  Volume group \"vg01\" not found\n  Cannot process volume group vg01", nil}},
				},
			},
			args{
				nil,
				&proto.VgsRequest{
					Target: []string{"vg01"},
					Select: "field=value",
				},
			},
			nil,
			true,
			codes.NotFound,
		},
		{
			"Fail when report contains invalid JSON",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"vgs", []string{"--options", "vg_name,pv_count,lv_count,snap_count,vg_attr,vg_size,vg_free,vg_tags", "--units", "b", "--nosuffix", "--reportformat", "json", "-S", "field=value", "vg01"}, 0, "{\"invalid\": \"json", "", nil}}},
			},
			args{
				nil,
				&proto.VgsRequest{
					Target: []string{"vg01"},
					Select: "field=value",
				},
			},
			nil,
			true,
			codes.Unknown,
		},
		{
			"Fail when report contain multiple report entries",
			fields{
				&fakeRunner{
					t:          t,
					executions: []fakeCommand{{"vgs", []string{"--options", "vg_name,pv_count,lv_count,snap_count,vg_attr,vg_size,vg_free,vg_tags", "--units", "b", "--nosuffix", "--reportformat", "json", "-S", "field=value", "vg01"}, 0, `{ "report": [ {},{} ] }`, "", nil}}},
			},
			args{
				nil,
				&proto.VgsRequest{
					Target: []string{"vg01"},
					Select: "field=value",
				},
			},
			nil,
			true,
			codes.Unknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := lvmctrldServer{
				cmd: tt.fields.cmd,
			}
			got, err := s.Vgs(tt.args.in0, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Vgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && status.Code(err) != tt.wantErrCode {
				t.Errorf("Vgs() error code = %v, wantErrCode %v", status.Code(err), tt.wantErrCode)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Vgs() diff: %v", pretty.Compare(tt.want, got))
			}
		})
	}
}
