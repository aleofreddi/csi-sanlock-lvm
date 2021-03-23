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
	"strings"
)

type TagKey string

const (
	tagPrefix = "csi-sanlock-lvm.vleo.net/"

	nameTagKey      TagKey = "name"
	fsTagKey        TagKey = "fs"
	ownerIdTagKey   TagKey = "ownerId"
	ownerNodeTagKey TagKey = "ownerNode"
	sourceTagKey    TagKey = "src"

	rpcRoleTagKey TagKey = "rpcRole"
)

type LogVolType string

const (
	VolumeVolType    LogVolType = "volume"
	SnapshotVolType  LogVolType = "snapshot"
	TemporaryVolType LogVolType = "temp"
	RpcVolType       LogVolType = "rpc"
)

var (
	encodedTagPrefix = encodeTag(tagPrefix)
)

func decodeTagKV(encoded string) (TagKey, string, bool, error) {
	if !strings.HasPrefix(encoded, encodedTagPrefix) {
		return "", "", false, nil
	}
	decoded, err := decodeTag(encoded[len(encodedTagPrefix):])
	if err != nil {
		return "", "", false, err
	}
	i := strings.Index(decoded, "=")
	if i == -1 {
		return "", "", false, fmt.Errorf("invalid tag value %q", encoded)
	}
	return TagKey(decoded[0:i]), decoded[i+1:], true, nil
}

func encodeTagKV(key TagKey, value string) string {
	return encodeTag(fmt.Sprintf("%s%s=%s", tagPrefix, key, value))
}

func encodeTagKeyPrefix(key TagKey) string {
	return encodeTag(fmt.Sprintf("%s%s=", tagPrefix, key))
}

func encodeTags(tags map[TagKey]string) []string {
	r := make([]string, len(tags))
	i := 0
	for key, value := range tags {
		r[i] = encodeTagKV(key, value)
		i++
	}
	return r
}

func decodeTags(encodedTags []string) (map[TagKey]string, error) {
	r := make(map[TagKey]string)
	for _, v := range encodedTags {
		key, value, ok, err := decodeTagKV(v)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if _, ok := r[key]; ok {
			return nil, fmt.Errorf("duplicate tag entry for key %q", key)
		}
		r[key] = value
	}
	return r, nil
}
