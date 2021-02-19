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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	pb "github.com/aleofreddi/csi-sanlock-lvm/proto"
)

const (
	// Prefix to be used for LVM volumes.
	volumeLvPrefix = "csi-v-"

	// Prefix to be used for LVM snapshots.
	snapshotLvPrefix = "csi-s-"

	// Prefix to be used for LVM temporary volumes or snapshots.
	tempLvPrefix = "csi-t-"
)

var volTypeToPrefix = map[LogVolType]string{
	VolumeVolType:    volumeLvPrefix,
	SnapshotVolType:  snapshotLvPrefix,
	TemporaryVolType: tempLvPrefix,
}

// A type that holds a volume reference. Only the VgName and LvName fields are expected to be set.
type VolumeRef struct {
	LvName string
	VgName string
}

func NewVolumeRefFromVgTypeName(vg string, t LogVolType, name string) *VolumeRef {
	h := sha256.New()
	h.Write([]byte(name))
	b64 := base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(h.Sum(nil))
	prefix, ok := volTypeToPrefix[t]
	if !ok {
		panic(fmt.Errorf("unexpected volume type %s", t))
	}
	// LVM doesn't like the '/' character, replace with '_'
	lv := prefix + strings.ReplaceAll(b64, "/", "_")
	return &VolumeRef{
		LvName: lv,
		VgName: vg,
	}
}

func NewVolumeRefFromID(volumeID string) (*VolumeRef, error) {
	tokens := strings.Split(volumeID, "@")
	if len(tokens) != 2 {
		return nil, fmt.Errorf("invalid volume id %q", volumeID)
	}
	return &VolumeRef{
		LvName: tokens[0],
		VgName: tokens[1],
	}, nil
}

func NewVolumeRefFromLv(lv *pb.LogicalVolume) *VolumeRef {
	return &VolumeRef{
		lv.LvName,
		lv.VgName,
	}
}

func (v *VolumeRef) ID() string {
	return fmt.Sprintf("%s@%s", v.LvName, v.VgName)
}

func (v *VolumeRef) Vg() string {
	return v.VgName
}

func (v *VolumeRef) Lv() string {
	return v.LvName
}

func (v *VolumeRef) VgLv() string {
	return fmt.Sprintf("%s/%s", v.VgName, v.LvName)
}

func (v *VolumeRef) DevPath() string {
	return fmt.Sprintf("/dev/%s/%s", v.VgName, v.LvName)
}

func (v *VolumeRef) String() string {
	return v.ID()
}
