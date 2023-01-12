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
	"errors"
	"fmt"
	"strings"
)

func encodeTag(value string) string {
	var sb strings.Builder
	for i, b := range []byte(value) {
		if isPlain(b, i) {
			sb.WriteByte(b)
		} else {
			sb.WriteString(fmt.Sprintf("&%2x", b))
		}
	}
	return sb.String()
}

func decodeTag(encodedTag string) (string, error) {
	type State int
	const (
		Normal State = iota
		Quote1
		Quote2
	)
	var sb strings.Builder
	qState := Normal
	var qValue byte
	for i, b := range []byte(encodedTag) {
		if qState != Normal {
			v, err := hexDecode(b)
			if err != nil {
				return "", errors.New("encoded tag contains invalid quote sequence")
			}
			if qState == Quote1 {
				qValue = v << 4
				qState = Quote2
			} else {
				sb.WriteByte(qValue | v)
				qValue = 0
				qState = Normal
			}
		} else if b == '&' {
			qState = Quote1
		} else if isPlain(b, i) {
			sb.WriteByte(b)
		} else {
			return "", errors.New("encoded tag contains an invalid character")
		}
	}
	if qState != 0 {
		return "", errors.New("encoded tag contains truncated quote sequence")
	}
	return sb.String(), nil
}

func isPlain(b byte, pos int) bool {
	return b >= 'a' && b <= 'z' ||
		b >= 'A' && b <= 'Z' ||
		b >= '0' && b <= '9' ||
		b == '_' ||
		b == '+' ||
		b == '.' ||
		(b == '-' && pos > 0) || // LVM doesn't allow hyphen starting tags
		b == '/' ||
		b == '=' ||
		b == '!' ||
		b == ':' ||
		b == '#'
}

func hexDecode(hex byte) (byte, error) {
	if hex >= '0' && hex <= '9' {
		return hex - '0', nil
	}
	if hex >= 'A' && hex <= 'F' {
		return hex - 'A' + 10, nil
	}
	if hex >= 'a' && hex <= 'f' {
		return hex - 'a' + 10, nil
	}
	return 0, errors.New("invalid hexadecimal sequence")
}
