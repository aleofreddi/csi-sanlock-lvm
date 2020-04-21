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
	"k8s.io/kubernetes/pkg/util/mount"
	"os"
	"os/exec"
)

type FileSystem interface {
	Make(device string) error
	Grow(device string) error
	Mount(source, mountPoint string, flags []string) error
	FsType() string
	Accepts(accessType volumeAccessType) bool
}

type rawFileSystem struct {
}

type ext4FileSystem struct {
}

func NewFileSystem(fs string) (FileSystem, error) {
	switch fs {
	case BLOCK_ACCESS_FS_NAME:
		return &rawFileSystem{}, nil
	case "ext4":
		return &ext4FileSystem{}, nil
	}
	return nil, fmt.Errorf("invalid filesystem %q", fs)
}

func (fs *rawFileSystem) Make(device string) error {
	return nil
}

func (fs *rawFileSystem) Grow(device string) error {
	return nil
}

func (fs *rawFileSystem) FsType() string {
	return ""
}

func (fs *rawFileSystem) Accepts(accessType volumeAccessType) bool {
	return accessType == BLOCK_ACCESS_TYPE
}

func (fs *rawFileSystem) Mount(source, mountPoint string, flags []string) error {
	mounter := mount.New("")
	mounted, err := mounter.IsLikelyNotMountPoint(mountPoint)

	if err != nil {
		if os.IsExist(err) {
			return status.Errorf(codes.Internal, "failed to determine if %s is mounted: %s", mountPoint, err.Error())
		}
		if err = mounter.MakeFile(mountPoint); err != nil {
			return status.Errorf(codes.Internal, "failed to create file %s: %s", mountPoint, err.Error())
		}
		mounted = true
	}

	if mounted {
		// Get Options
		options := []string{"bind"}
		options = append(options, flags...)

		// Mount
		mounter := mount.New("")
		err = mounter.Mount(source, mountPoint, fs.FsType(), options)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
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

func (fs *ext4FileSystem) FsType() string {
	return "ext4"
}

func (fs *ext4FileSystem) Accepts(accessType volumeAccessType) bool {
	return accessType == MOUNT_ACCESS_TYPE
}

func (fs *ext4FileSystem) Mount(source, mountPoint string, flags []string) error {
	mounter := mount.New("")
	mounted, err := mounter.IsLikelyNotMountPoint(mountPoint)
	if err != nil {
		if os.IsExist(err) {
			return status.Errorf(codes.Internal, "failed to determine if %s is mounted: %s", mountPoint, err.Error())
		}
		if err := os.MkdirAll(mountPoint, 0750); err != nil {
			return status.Errorf(codes.Internal, "failed to mkdir %s: %s", mountPoint, err.Error())
		}
		mounted = true
	}

	if mounted {
		err = mounter.Mount(source, mountPoint, fs.FsType(), flags)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}
