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

type FileSystemFactoryInterface interface {
	New(fs string) (FileSystem, error)
}

type FileSystemFactory func(fs string) (FileSystem, error)

type FileSystem interface {
	Accepts(accessType VolumeAccessType) bool
	Make(device string) error
	Grow(device string) error
	Mount(source, mountPoint string, flags []string) error
}

type rawFileSystem struct {
	mounter mount.Interface
}

type ext4FileSystem struct {
	mounter mount.Interface
}

func NewFileSystem(fs string) (FileSystem, error) {
	switch fs {
	case BlockAccessFsName:
		return &rawFileSystem{mount.New("")}, nil
	case "ext4":
		return &ext4FileSystem{mount.New("")}, nil
	}
	return nil, fmt.Errorf("invalid filesystem %q", fs)
}

func (fs *rawFileSystem) Make(device string) error {
	return nil
}

func (fs *rawFileSystem) Grow(device string) error {
	return nil
}

func (fs *rawFileSystem) Accepts(accessType VolumeAccessType) bool {
	return accessType == BlockAccessType
}

func (fs *rawFileSystem) Mount(source, mountPoint string, flags []string) error {
	mounted, err := fs.mounter.IsLikelyNotMountPoint(mountPoint)
	if err != nil {
		if os.IsExist(err) {
			return status.Errorf(codes.Internal, "failed to determine if %s is mounted: %s", mountPoint, err.Error())
		}
		if err = fs.mounter.MakeFile(mountPoint); err != nil {
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
		err = mounter.Mount(source, mountPoint, "", options)
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

func (fs *ext4FileSystem) Accepts(accessType VolumeAccessType) bool {
	return accessType == MountAccessType
}

func (fs *ext4FileSystem) Mount(source, mountPoint string, flags []string) error {
	mounted, err := fs.mounter.IsLikelyNotMountPoint(mountPoint)
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
		err = fs.mounter.Mount(source, mountPoint, "ext4", flags)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}
