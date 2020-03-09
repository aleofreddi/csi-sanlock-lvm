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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aleofreddi/csi-sanlock-lvm/lvmctrld/proto"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"regexp"
	"strings"
	"time"
)

// Credits to dave at https://stackoverflow.com/questions/40939261/golang-parse-strange-date-format
type lvmTime struct {
	time.Time
}

func (t *lvmTime) UnmarshalJSON(buf []byte) error {
	date, err := time.Parse("2006-01-02 15:04:05 -0700", strings.Trim(string(buf), `"`))
	if err != nil {
		return err
	}
	t.Time = date
	return nil
}

type lvmReportLvs struct {
	LvName          string  `json:"lv_name"`
	VgName          string  `json:"vg_name"`
	LvAttr          string  `json:"lv_attr"`
	LvSize          uint64  `json:"lv_size,string"`
	PoolLv          string  `json:"pool_lv"`
	Origin          string  `json:"origin"`
	DataPercent     string  `json:"data_percent"`
	MetadataPercent string  `json:"metadata_percent"`
	MovePv          string  `json:"move_pv"`
	MirrorLog       string  `json:"mirror_log"`
	CopyPercent     string  `json:"copy_percent"`
	ConvertLv       string  `json:"convert_lv"`
	LvTags          string  `json:"lv_tags"`
	LvRole          string  `json:"lv_role"`
	LvTime          lvmTime `json:"lv_time,string"`
}

type lvmReportVgs struct {
	VgName    string `json:"vg_name"`
	PvCount   uint32 `json:"pv_count,string"`
	LvCount   uint32 `json:"lv_count,string"`
	SnapCount uint32 `json:"snap_count,string"`
	VgAttr    string `json:"vg_attr"`
	VgSize    uint64 `json:"vg_size,string"`
	VgFree    uint64 `json:"vg_free,string"`
	VgTags    string `json:"vg_tags"`
}

type lvmReport struct {
	Report []struct {
		Lvs []lvmReportLvs `json:"lv"`
		Vgs []lvmReportVgs `json:"vg"`
	} `json:"report"`
}

var (
	// According to https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/7/html/logical_volume_manager_administration/lvm_tags,
	// LVM tags should match the following regex.
	tagRe = regexp.MustCompile("^[A-Za-z0-9_+.\\-/=!:#&]+$")

	lvLockedRe = regexp.MustCompile(`(?mi)^\s*LV locked by other host`)
	lvExists   = regexp.MustCompile(`(?mi)^\s*Logical Volume "[^"]+" already exists in volume group`)
	lvNotFound = []*regexp.Regexp{
		regexp.MustCompile(`(?mi)^\s*Failed to find logical volume "[^"]+"`),
		regexp.MustCompile(`(?mi)^\s*Logical volume \S+ not found in volume group \S+.`),
	}
	vgNotFound   = regexp.MustCompile(`(?mi)^\s*Volume group "[^"]+" not found`)
	lvOutOfRange = []*regexp.Regexp{
		regexp.MustCompile(`(?mi)^\s*Volume group "[^"]+" has insufficient free space`),
		regexp.MustCompile(`(?mi)^\s*Insufficient free space: \d+ extents needed, but only \d+ available`),
	}
	lvSizeMatches = regexp.MustCompile(`(?mi)^\s*New size \(\d+ extents\) matches existing size \(\d+ extents\)`)
)

type lvmctrldServer struct {
	cmd commander
}

func NewLvmctrldServer() *lvmctrldServer {
	return &lvmctrldServer{
		NewCommander(),
	}
}

func (s lvmctrldServer) Vgs(_ context.Context, req *proto.VgsRequest) (*proto.VgsResponse, error) {
	args := []string{
		"--options", "vg_name,pv_count,lv_count,snap_count,vg_attr,vg_size,vg_free,vg_tags",
		"--units", "b",
		"--nosuffix",
		"--reportformat", "json",
	}
	if req.GetSelect() != "" {
		args = append(args, "-S", req.GetSelect())
	}
	if req.Target != "" {
		args = append(args, req.Target)
	}
	out, err := runReport(s.cmd, "vgs", args...)
	if err != nil {
		return nil, err
	}
	if len(out.Report) != 1 {
		return nil, errors.New("unexpected multiple reports")
	}
	vgs := make([]*proto.VolumeGroup, len(out.Report[0].Vgs))
	for i, v := range out.Report[0].Vgs {
		vgs[i] = lvmToVolumeGroup(&v)
	}
	return &proto.VgsResponse{
		Vgs: vgs,
	}, nil
}

func (s lvmctrldServer) LvCreate(_ context.Context, req *proto.LvCreateRequest) (*proto.LvCreateResponse, error) {
	args := []string{
		"-y",
		"-L", fmt.Sprintf("%db", req.Size),
	}
	args = lvmToActivationMode(args, req.GetActivate())
	if req.Origin != "" {
		args = append(args, "-s", req.Origin)
	}
	for _, tag := range req.LvTags {
		if !tagRe.MatchString(tag) {
			return nil, fmt.Errorf("invalid tag %s", tag)
		}
		args = append(args, "--addtag", tag)
	}
	args = append(args, "-n", fmt.Sprintf("%s/%s", req.VgName, req.LvName))
	code, stdout, stderr, err := s.cmd.Exec("lvcreate", args...)
	if code != 0 || err != nil {
		return nil, parseLvmError(code, stdout, stderr)
	}
	return &proto.LvCreateResponse{}, nil
}

func (s lvmctrldServer) LvRemove(_ context.Context, req *proto.LvRemoveRequest) (*proto.LvRemoveResponse, error) {
	args := []string{
		"-f",
	}
	if req.GetSelect() != "" {
		args = append(args, "-S", req.GetSelect())
	}
	args = append(args, fmt.Sprintf("%s/%s", req.VgName, req.LvName))

	code, stdout, stderr, err := s.cmd.Exec("lvremove", args...)
	if code != 0 || err != nil {
		return nil, parseLvmError(code, stdout, stderr)
	}
	return &proto.LvRemoveResponse{}, nil
}

func (s lvmctrldServer) Lvs(_ context.Context, req *proto.LvsRequest) (*proto.LvsResponse, error) {
	args := []string{
		"--options", "lv_name,vg_name,lv_attr,lv_size,pool_lv,origin,data_percent,metadata_percent,move_pv,mirror_log,copy_percent,convert_lv,lv_tags,lv_role,lv_time",
		"--units", "b",
		"--nosuffix",
		"--reportformat", "json",
	}
	if req.Select != "" {
		args = append(args, "-S", req.Select)
	}
	if req.Target != "" {
		args = append(args, req.Target)
	}
	out, err := runReport(s.cmd, "lvs", args...)
	if err != nil {
		return nil, err
	}
	if len(out.Report) != 1 {
		return nil, errors.New("unexpected multiple reports")
	}
	lvs := make([]*proto.LogicalVolume, len(out.Report[0].Lvs))
	for i, v := range out.Report[0].Lvs {
		lvs[i] = lvmToLogicalVolume(&v)
	}
	return &proto.LvsResponse{
		Lvs: lvs,
	}, nil
}

func (s lvmctrldServer) LvResize(ctx context.Context, req *proto.LvResizeRequest) (*proto.LvResizeResponse, error) {
	args := []string{
		"-f",
		"-L", fmt.Sprintf("%db", req.Size),
		fmt.Sprintf("%s/%s", req.VgName, req.LvName),
	}
	code, stdout, stderr, err := s.cmd.Exec("lvresize", args...)
	if code != 0 || err != nil {
		return nil, parseLvmError(code, stdout, stderr)
	}
	return &proto.LvResizeResponse{}, nil
}

func (s lvmctrldServer) LvChange(ctx context.Context, req *proto.LvChangeRequest) (*proto.LvChangeResponse, error) {
	args := make([]string, 0)
	args = lvmToActivationMode(args, req.GetActivate())
	for _, tag := range req.AddTag {
		args = append(args, "--addtag", tag)
	}
	for _, tag := range req.DelTag {
		args = append(args, "--deltag", tag)
	}
	if req.GetSelect() != "" {
		args = append(args, "-S", req.GetSelect())
	}
	args = append(args, req.GetTarget())
	code, stdout, stderr, err := s.cmd.Exec("lvchange", args...)
	if code != 0 || err != nil {
		return nil, parseLvmError(code, stdout, stderr)
	}
	return &proto.LvChangeResponse{}, nil
}

func runReport(cmd commander, exe string, args ...string) (*lvmReport, error) {
	code, stdout, stderr, err := cmd.Exec(exe, args...)
	if code != 0 || err != nil {
		return nil, parseLvmError(code, stdout, stderr)
	}
	var result lvmReport
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, fmt.Errorf("failed to deserialize lvm report with error %v: %q", err, stdout)
	}
	return &result, nil
}

func lvmToActivationMode(args []string, activationMode proto.LvActivationMode) []string {
	if activationMode == proto.LvActivationMode_NONE {
		return args
	}
	switch activationMode {
	case proto.LvActivationMode_ACTIVE_EXCLUSIVE:
		return append(args, "-a", "ey")
	case proto.LvActivationMode_ACTIVE_SHARED:
		return append(args, "-a", "sy")
	case proto.LvActivationMode_DEACTIVATE:
		return append(args, "-a", "n")
	default:
		panic(fmt.Sprintf("unknown activation mode %d", activationMode))
	}
}

func lvmToVolumeGroup(vg *lvmReportVgs) *proto.VolumeGroup {
	return &proto.VolumeGroup{
		VgName:    vg.VgName,
		PvCount:   vg.PvCount,
		LvCount:   vg.LvCount,
		SnapCount: vg.SnapCount,
		VgAttr:    vg.VgAttr,
		VgSize:    vg.VgSize,
		VgFree:    vg.VgFree,
		VgTags:    splitLvmField(vg.VgTags),
	}
}

func lvmToLogicalVolume(lv *lvmReportLvs) *proto.LogicalVolume {
	lvTime, _ := ptypes.TimestampProto(lv.LvTime.Time)
	return &proto.LogicalVolume{
		LvName:          lv.LvName,
		VgName:          lv.VgName,
		LvAttr:          lv.LvAttr,
		LvSize:          lv.LvSize,
		PoolLv:          lv.PoolLv,
		Origin:          lv.Origin,
		DataPercent:     lv.DataPercent,
		MetadataPercent: lv.MetadataPercent,
		MovePv:          lv.MovePv,
		MirrorLog:       lv.MirrorLog,
		CopyPercent:     lv.CopyPercent,
		ConvertLv:       lv.ConvertLv,
		LvTags:          splitLvmField(lv.LvTags),
		LvRole:          splitLvmField(lv.LvRole),
		LvTime:          lvTime,
	}
}

func splitLvmField(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

func parseLvmError(code int, stdout, stderr []byte) error {
	if code == 0 {
		return nil
	}
	// On "ordinary" errors, lvm commands will return 5: anything other than 5 is then an unknown error
	if code != 5 {
		return fmt.Errorf("unexpected error: rc=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	// Check stderr to identify the failure reason
	if lvExists.Match(stderr) {
		return status.Errorf(codes.AlreadyExists, "target already exists")
	}
	if lvLockedRe.Match(stderr) {
		return status.Errorf(codes.PermissionDenied, "target is locked by another host")
	}
	if vgNotFound.Match(stderr) {
		return status.Errorf(codes.NotFound, "volume group does not exist")
	}
	for _, re := range lvNotFound {
		if re.Match(stderr) {
			return status.Errorf(codes.NotFound, "logical volume does not exist")
		}
	}
	for _, re := range lvOutOfRange {
		if re.Match(stderr) {
			return status.Errorf(codes.OutOfRange, "insufficient free space")
		}
	}
	if lvSizeMatches.Match(stderr) {
		// We do not consider a size match an error
		return nil
	}
	return fmt.Errorf("unexpected error: rc=%d stdout=%q stderr=%q", code, stdout, stderr)
}
