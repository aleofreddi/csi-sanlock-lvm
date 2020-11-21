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
	"testing"
)

func Test_encodeTag(t *testing.T) {
	type args struct {
		value string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"Unquoted characters", args{"unquoted.chars=01234456789_+.-/=!:#"}, "unquoted.chars=01234456789_+.-/=!:#"},
		{"Quoted characters", args{"quoted_chars= {}$`"}, "quoted_chars=&20&7b&7d&24&60"},
		{"Quote hyphen only when is the first character", args{"-_should_be_quoted._while_-_not"}, "&2d_should_be_quoted._while_-_not"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeTag(tt.args.value); got != tt.want {
				t.Errorf("encodeTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_decodeTag(t *testing.T) {
	type args struct {
		encodedTag string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"Unquoted characters", args{"unquoted.chars=01234456789"}, "unquoted.chars=01234456789", false},
		{"Quoted characters", args{"quoted.chars=&20&2b&2d&7b&7d&24&60"}, "quoted.chars= +-{}$`", false},
		{"Invalid characters", args{"invalid +"}, "", true},
		{"Decode quoted and unquoted hyphen", args{"&2d-&2d"}, "---", false},
		{"Reject unquoted initial hyphen", args{"--"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeTag(tt.args.encodedTag)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("decodeTag() got = %v, want %v", got, tt.want)
			}
		})
	}
}
