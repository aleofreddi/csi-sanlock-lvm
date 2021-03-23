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
	"regexp"
	"strings"

	pb "github.com/aleofreddi/csi-sanlock-lvm/proto"
)

const (
	lvmPrefix = "csl"
)

var volTypeToCode = map[LogVolType]uint8{
	VolumeVolType:    'v',
	SnapshotVolType:  's',
	TemporaryVolType: 't',
	RpcVolType:       'r',
}

var codeToVolType = map[uint8]LogVolType{
	'v': VolumeVolType,
	's': SnapshotVolType,
	't': TemporaryVolType,
	'r': RpcVolType,
}

var lvNameRe = regexp.MustCompile("^" + regexp.QuoteMeta(lvmPrefix) + "-(.)-(.+)")

type VolumeRef interface {
	// Returns the volume id.
	ID() string

	// Returns the volume group.
	Vg() string

	// Returns the logical volume.
	Lv() string

	// Returns volume group '/' logical volume.
	VgLv() string

	// Returns the device path, that is '/dev/' volume group '/' logical volume.
	DevPath() string

	String() string
}

// A type that holds a volume reference. Only the vgName and LvName fields are expected to be set.
type volumeRef struct {
	volType LogVolType
	sha     string
	vgName  string
}

func NewVolumeRefFromVgTypeName(vg string, volType LogVolType, name string) *volumeRef {
	h := sha256.New()
	h.Write([]byte(name))
	// LVM doesn't like the '/' character, we replace it with '_'.
	sha := strings.ReplaceAll(
		base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(h.Sum(nil)),
		"/", "_")
	return &volumeRef{
		volType: volType,
		sha:     sha,
		vgName:  vg,
	}
}

func NewVolumeRefFromID(volumeID string) (*volumeRef, error) {
	tokens := strings.Split(volumeID, ":")
	if len(tokens) != 3 || len(tokens[0]) != 1 {
		return nil, fmt.Errorf("invalid volume id %q", volumeID)
	}
	volType, ok := codeToVolType[tokens[0][0]]
	if !ok {
		return nil, fmt.Errorf("unexpected volume type %s", tokens[0])
	}
	return &volumeRef{
		volType: volType,
		vgName:  tokens[1],
		sha:     tokens[2],
	}, nil
}

func NewVolumeRefFromVgLv(vgName, lvName string) (*volumeRef, error) {
	tokens := lvNameRe.FindStringSubmatch(lvName)
	if tokens == nil {
		return nil, fmt.Errorf("unexpected format for logvol %s", lvName)
	}
	volType, ok := codeToVolType[tokens[1][0]]
	if !ok {
		return nil, fmt.Errorf("unexpected volume type for logvol %s", lvName)
	}
	return &volumeRef{
		volType,
		tokens[2],
		vgName,
	}, nil
}

func NewVolumeRefFromLv(lv *pb.LogicalVolume) (*volumeRef, error) {
	return NewVolumeRefFromVgLv(lv.VgName, lv.LvName)
}

func (v *volumeRef) ID() string {
	return fmt.Sprintf("%c:%s:%s", volTypeToCode[v.volType], v.vgName, v.sha)
}

func (v *volumeRef) Vg() string {
	return v.vgName
}

func (v *volumeRef) Lv() string {
	return fmt.Sprintf("%s-%c-%s", lvmPrefix, volTypeToCode[v.volType], v.sha)
}

func (v *volumeRef) VgLv() string {
	return fmt.Sprintf("%s/%s", v.vgName, v.Lv())
}

func (v *volumeRef) DevPath() string {
	return fmt.Sprintf("/dev/%s/%s", v.vgName, v.Lv())
}

func (v *volumeRef) String() string {
	return v.ID()
}
