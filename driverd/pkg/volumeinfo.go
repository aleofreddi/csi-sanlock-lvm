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
	pb "github.com/aleofreddi/csi-sanlock-lvm/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A type that holds details of a volume by wrapping a LogicalVolume.
type VolumeInfo struct {
	VolumeRef
	pb.LogicalVolume
}

func NewVolumeDetailsFromLv(lv pb.LogicalVolume) *VolumeInfo {
	return &VolumeInfo{
		VolumeRef{lv.LvName, lv.VgName},
		lv,
	}
}

func (v *VolumeInfo) OriginRef() *VolumeRef {
	return &VolumeRef{
		LvName: v.Origin,
		VgName: v.LogicalVolume.VgName,
	}
}

func (v *VolumeInfo) Tags() (map[TagKey]string, error) {
	tags, err := decodeTags(v.LvTags)
	if err != nil {
		return nil, status.Errorf(codes.Internal /* is this the right code? this should be non retryable */, "failed to decode tags for volume %q: %v", v.ID(), err)
	}
	return tags, nil
}

func (v *VolumeInfo) String() string {
	return v.ID()
}
