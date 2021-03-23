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
	"fmt"

	pb "github.com/aleofreddi/csi-sanlock-lvm/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A type that holds details of a volume by wrapping a LogicalVolume.
type VolumeInfo interface {
	VolumeRef

	// Get volume size.
	Size() uint64

	// Retrieve the volume reference.
	//Ref() VolumeRef

	// Retrieve the origin reference.
	OriginRef() VolumeRef

	// Get volume tags.
	Tags() (map[TagKey]string, error)

	// Return the underlying LVM volume.
	LvmLv() *pb.LogicalVolume
}

// A type that holds details of a volume by wrapping a LogicalVolume.
type volumeInfo struct {
	VolumeRef
	*pb.LogicalVolume
}

func NewVolumeInfoFromLv(lv *pb.LogicalVolume) (VolumeInfo, error) {
	vr, err := NewVolumeRefFromLv(lv)
	if err != nil {
		return nil, err
	}
	return &volumeInfo{
		vr,
		lv,
	}, nil
}

func (v *volumeInfo) Ref() VolumeRef {
	return v.VolumeRef
}

func (v *volumeInfo) OriginRef() VolumeRef {
	vr, err := NewVolumeRefFromVgLv(v.LogicalVolume.VgName, v.Origin)
	if err != nil {
		panic(fmt.Errorf("unexpected failure when extracting origin volume: %v", err))
	}
	return vr
}

func (v *volumeInfo) Tags() (map[TagKey]string, error) {
	tags, err := decodeTags(v.LvTags)
	if err != nil {
		return nil, status.Errorf(codes.Internal /* is this the right code? this should be non retryable */, "failed to decode tags for volume %q: %v", v.ID(), err)
	}
	return tags, nil
}

func (v *volumeInfo) Size() uint64 {
	return v.LvSize
}

func (v *volumeInfo) String() string {
	return v.ID()
}

func (v *volumeInfo) LvmLv() *pb.LogicalVolume {
	return v.LogicalVolume
}