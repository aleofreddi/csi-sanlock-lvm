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
	"os"
	"os/exec"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
)

type FileSystemRegistry interface {
	GetFileSystem(filesystem string) (FileSystem, error)
}

type FileSystem interface {
	Accepts(accessType VolumeAccessType) bool
	Make(device string) error
	Grow(device string) error
	Mount(source, mountPoint string, flags []string) error
	Umount(mountPoint string) error
}

type fileSystemRegistry struct {
}

type rawFileSystem struct {
	mounter mount.Interface
}

type fileSystem struct {
	mounter    mount.Interface
	fileSystem string
}

func NewFileSystemRegistry() (*fileSystemRegistry, error) {
	return &fileSystemRegistry{}, nil
}

func (fr *fileSystemRegistry) GetFileSystem(filesystem string) (FileSystem, error) {
	return NewFileSystem(filesystem)
}

func NewFileSystem(fs string) (FileSystem, error) {
	if fs == BlockAccessFsName {
		return &rawFileSystem{mount.New("")}, nil
	}
	return &fileSystem{mount.New(""), fs}, nil
}

func (fs *rawFileSystem) Make(_ string) error {
	return nil
}

func (fs *rawFileSystem) Grow(_ string) error {
	return nil
}

func (fs *rawFileSystem) Accepts(accessType VolumeAccessType) bool {
	return accessType == BlockAccessType
}

func (fs *rawFileSystem) Mount(source, mountPoint string, flags []string) error {
	mounted, err := fs.mounter.IsLikelyNotMountPoint(mountPoint)
	if err != nil {
		if os.IsExist(err) {
			return status.Errorf(codes.Internal, "failed to determine if %s is mounted: %v", mountPoint, err)
		}
		if err = fs.mounter.MakeDir(mountPoint); err != nil {
			return status.Errorf(codes.Internal, "failed to create mount point %s: %v", mountPoint, err)
		}
		mounted = true
	}

	if mounted {
		// Mount
		mounter := mount.New("")
		err = mounter.Mount(source, mountPoint, "", flags)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}

func (fs *rawFileSystem) Umount(mountPoint string) error {
	return umount(mountPoint)
}

func (fs *fileSystem) Make(device string) error {
	mkfs := exec.Command("mkfs", "-t", fs.fileSystem, device)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	mkfs.Stdout = stdout
	mkfs.Stderr = stderr
	if mkfs.Run() != nil {
		return status.Errorf(codes.Internal, "failed to format volume %s: %s %s", device, stdout.String(), stderr.String())
	}
	return nil
}

func (fs *fileSystem) Grow(device string) error {
	checkfs := exec.Command("fsadm", "check", device)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	checkfs.Stdout = stdout
	checkfs.Stderr = stderr
	// 'fsadm check' can return code 3 when the requested check operation could
	// not be performed because the filesystem is mounted and does not support an
	// online fsck.
	if err := checkfs.Run(); err != nil && checkfs.ProcessState.ExitCode() != 3 {
		return status.Errorf(codes.Internal, "failed to check volume %s: %v (%s %s)", device, err, stdout.String(), stderr.String())
	}
	resize2fs := exec.Command("fsadm", "resize", device)
	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	resize2fs.Stdout = stdout
	resize2fs.Stderr = stderr
	if err := resize2fs.Run(); err != nil {
		return status.Errorf(codes.Internal, "failed to resize volume %s: %v (%s %s)", device, err, stdout.String(), stderr.String())
	}
	return nil
}

func (fs *fileSystem) Accepts(accessType VolumeAccessType) bool {
	return accessType == MountAccessType
}

func (fs *fileSystem) Mount(source, mountPoint string, flags []string) error {
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
		err = fs.mounter.Mount(source, mountPoint, fs.fileSystem, flags)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}

func (fs *fileSystem) Umount(mountPoint string) error {
	return umount(mountPoint)
}

func umount(targetPath string) error {
	mounter := mount.New("")
	notMounted, err := mounter.IsNotMountPoint(targetPath)
	if err != nil && err == os.ErrNotExist || notMounted {
		return nil
	}

	err = mounter.Unmount(targetPath)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to unmount %q: %s", targetPath, err.Error())
	}

	if err = os.RemoveAll(targetPath); err != nil {
		return status.Errorf(codes.Internal, "failed to remove %q: %s", targetPath, err.Error())
	}

	return nil
}
