// Copyright 2023 Google LLC
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

package prototest

import (
	"github.com/golang/protobuf/proto"
	"reflect"
)

// Without returns a clone of the given a protobuf message with all the given fields cleared.
func Without[T proto.Message](msg T, fields ...string) T {
	msg = proto.Clone(msg).(T)
	vs := reflect.Indirect(reflect.ValueOf(msg))
	for _, field := range fields {
		f := vs.FieldByName(field)
		f.Set(reflect.Zero(f.Type()))
	}
	return msg
}

// Merge returns the result of merging two messages.
func Merge[T proto.Message](dst, src T) T {
	dst = proto.Clone(dst).(T)
	proto.Merge(dst, src)
	return dst
}
