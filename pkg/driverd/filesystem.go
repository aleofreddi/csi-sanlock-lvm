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
	"k8s.io/utils/mount"
)

type FileSystemRegistry interface {
	GetFileSystem(filesystem string) (FileSystem, error)
}

type FileSystem interface {
	Accepts(accessType VolumeAccessType) bool
	Make(device string) error
	Grow(device string) error
	Mount(source, mountPoint string, flags []string, create bool) error
	Umount(mountPoint string, delete bool) error
}

type fileSystemRegistry struct {
}

type bindFileSystem struct {
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
	if fs == BlockAccessFsName || fs == BindFsName {
		return &bindFileSystem{mount.New("")}, nil
	}
	return &fileSystem{mount.New(""), fs}, nil
}

func (fs *bindFileSystem) Make(_ string) error {
	return nil
}

func (fs *bindFileSystem) Grow(_ string) error {
	return nil
}

func (fs *bindFileSystem) Accepts(accessType VolumeAccessType) bool {
	return accessType == BlockAccessType
}

func (fs *bindFileSystem) Mount(source, mountPoint string, flags []string, create bool) error {
	notMounted, err := fs.mounter.IsLikelyNotMountPoint(mountPoint)
	if err != nil {
		if !os.IsNotExist(err) {
			return status.Errorf(codes.Internal, "failed to determine if %s is mounted: %v", mountPoint, err)
		}
		if !create {
			return status.Errorf(codes.Internal, "%s does not exist", mountPoint)
		}
		if err = os.Mkdir(mountPoint, 0750); err != nil {
			return status.Errorf(codes.Internal, "failed to create mount point %s: %v", mountPoint, err)
		}
		notMounted = true
	}

	if notMounted {
		// Mount the filesystem.
		mounter := mount.New("")
		err = mounter.Mount(source, mountPoint, "", flags)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to mount: %v", err)
		}
	}
	return nil
}

func (fs *bindFileSystem) Umount(mountPoint string, delete bool) error {
	return umountIfMounted(mountPoint, delete)
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
	resize := exec.Command("fsadm", "resize", device)
	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	resize.Stdout = stdout
	resize.Stderr = stderr
	if err := resize.Run(); err != nil {
		return status.Errorf(codes.Internal, "failed to resize volume %s: %v (%s %s)", device, err, stdout.String(), stderr.String())
	}
	return nil
}

func (fs *fileSystem) Accepts(accessType VolumeAccessType) bool {
	return accessType == MountAccessType
}

func (fs *fileSystem) Mount(source, mountPoint string, flags []string, create bool) error {
	notMounted, err := fs.mounter.IsLikelyNotMountPoint(mountPoint)
	if err != nil {
		if !os.IsNotExist(err) {
			return status.Errorf(codes.Internal, "failed to determine if %s is mounted: %v", mountPoint, err)
		}
		if !create {
			return status.Errorf(codes.Internal, "%s does not exist", mountPoint)
		}
		if err := os.MkdirAll(mountPoint, 0750); err != nil {
			return status.Errorf(codes.Internal, "failed to mkdir %s: %v", mountPoint, err)
		}
		notMounted = true
	}

	if notMounted {
		// Mount the filesystem.
		err = fs.mounter.Mount(source, mountPoint, fs.fileSystem, flags)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to mount: %v", err)
		}
	}
	return nil
}

func (fs *fileSystem) Umount(mountPoint string, delete bool) error {
	return umountIfMounted(mountPoint, delete)
}

func umountIfMounted(targetPath string, delete bool) error {
	mounter := mount.New("")
	notMounted, err := mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && err == os.ErrNotExist {
		return nil
	}
	if !notMounted {
		err = mounter.Unmount(targetPath)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to unmount %q: %s", targetPath, err.Error())
		}
	}
	if delete {
		if err = os.RemoveAll(targetPath); err != nil {
			return status.Errorf(codes.Internal, "failed to remove %q: %s", targetPath, err.Error())
		}
	}
	return nil
}
