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
	"bytes"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os/exec"
)

type FileSystem interface {
	Make(device string) error
	Grow(device string) error
}

type ext4FileSystem struct {
}

func NewFileSystem(fs string) (FileSystem, error) {
	switch fs {
	case "ext4":
		return &ext4FileSystem{}, nil
	}
	return nil, fmt.Errorf("invalid filesystem %s", fs)
}

func (fs *ext4FileSystem) Make(device string) error {
	mkfs := exec.Command("mkfs", "-t", "ext4", device)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	mkfs.Stdout = stdout
	mkfs.Stderr = stderr
	if mkfs.Run() != nil {
		return status.Errorf(codes.Internal, "failed to format volume %s: %s %s]", device, stdout.String(), stderr.String())
	}
	return nil
}

func (fs *ext4FileSystem) Grow(device string) error {
	resize2fs := exec.Command("resize2fs", device)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	resize2fs.Stdout = stdout
	resize2fs.Stderr = stderr
	if resize2fs.Run() != nil {
		return status.Errorf(codes.Internal, "failed to resize volume")
	}
	return nil
}
