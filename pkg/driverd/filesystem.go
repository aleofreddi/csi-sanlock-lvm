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
	"path/filepath"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/mount"
)

// Action to be taken when mountpoint does not exist.
type mountPointAction int

const (
	requireExisting mountPointAction = 0
	createFile                       = 1
	createDirectory                  = 2
)

type FileSystemRegistry interface {
	GetFileSystem(filesystem string) (FileSystem, error)
}

type FileSystem interface {
	Accepts(accessType VolumeAccessType) bool
	Make(device string) error
	Grow(device string) error
	Stage(device, stagePoint string, flags []string, grpID *int) error
	Unstage(stagePoint string) error
	Publish(device, stagePoint, mountPoint string, readOnly bool) error
	Unpublish(mountPoint string) error
	Stat(mountPoint string) (*unix.Statfs_t, error)
}

type fileSystemRegistry struct {
}

type rawFileSystem struct {
}

type fileSystem struct {
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
		return &rawFileSystem{}, nil
	}
	return &fileSystem{fs}, nil
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

func (fs *fileSystem) Stage(device, stagePoint string, flags []string, grpID *int) error {
	err := mountFs(device, stagePoint, fs.fileSystem, flags, requireExisting)
	if err != nil {
		return err
	}
	if grpID != nil {
		if err := grantGroupAccess(stagePoint, *grpID); err != nil {
			return status.Errorf(codes.Internal, "failed to grant group access for volume %s: %v", device, err)
		}
	}
	return nil
}

func (fs *fileSystem) Unstage(mountPoint string) error {
	return umountFs(mountPoint, false)
}

func (fs *fileSystem) Publish(device, stagePoint, mountPoint string, readOnly bool) error {
	flags := []string{"bind"}
	if readOnly {
		flags = append(flags, "ro")
	}
	return mountFs(stagePoint, mountPoint, "", flags, createDirectory)
}

func (fs *fileSystem) Unpublish(mountPoint string) error {
	return umountFs(mountPoint, true)
}

func (fs *fileSystem) Stat(mountPoint string) (*unix.Statfs_t, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(mountPoint, &stat)
	return &stat, err
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

func (fs *rawFileSystem) Stage(device, stagePoint string, flags []string, grpID *int) error {
	if grpID != nil {
		if err := grantGroupAccess(device, *grpID); err != nil {
			return status.Errorf(codes.Internal, "failed to grant group access for volume %s: %v", device, err)
		}
	}
	return nil
}

func (fs *rawFileSystem) Unstage(mountPoint string) error {
	return nil
}

func (fs *rawFileSystem) Stat(mountPoint string) (*unix.Statfs_t, error) {
	return nil, nil
}

func (fs *rawFileSystem) Publish(device, stagePoint, mountPoint string, readOnly bool) error {
	flags := []string{"bind"}
	if readOnly {
		flags = append(flags, "ro")
	}
	return mountFs(device, mountPoint, "", flags, createFile)
}

func (fs *rawFileSystem) Unpublish(mountPoint string) error {
	return umountFs(mountPoint, true)
}

func grantGroupAccess(path string, groupID int) error {
	// Search for files
	err := filepath.WalkDir(path, func(file string, d os.DirEntry, err error) error {
		return os.Chown(file, -1, groupID)
	})
	if err != nil {
		return err
	}
	// Ensure root has full group access.
	if err := os.Chmod(path, os.FileMode(0770)); err != nil {
		return err
	}
	return nil
}

func mountFs(source, mountPoint, fsName string, flags []string, mpAction mountPointAction) error {
	mounter := mount.New("")
	notMounted, err := mounter.IsLikelyNotMountPoint(mountPoint)
	if err != nil {
		if !os.IsNotExist(err) {
			return status.Errorf(codes.Internal, "failed to determine if %s is mounted: %v", mountPoint, err)
		}
		switch mpAction {
		case requireExisting:
			return status.Errorf(codes.Internal, "%s does not exist", mountPoint)
		case createFile:
			file, err := os.OpenFile(mountPoint, os.O_CREATE, os.FileMode(0640))
			if err = file.Close(); err != nil {
				return err
			}
			if err != nil {
				if !os.IsExist(err) {
					return status.Errorf(codes.Internal, "failed to create file %s: %v", mountPoint, err)
				}
			}
		case createDirectory:
			if err := os.MkdirAll(mountPoint, 0750); err != nil {
				return status.Errorf(codes.Internal, "failed to mkdir %s: %v", mountPoint, err)
			}
		}
		notMounted = true
	}

	if notMounted {
		// Mount the filesystem.
		err = mounter.Mount(source, mountPoint, fsName, flags)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to mount: %v", err)
		}
	}
	return nil
}

func umountFs(targetPath string, deleteMountPoint bool) error {
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
	if deleteMountPoint {
		if err = os.RemoveAll(targetPath); err != nil {
			return status.Errorf(codes.Internal, "failed to remove %q: %s", targetPath, err.Error())
		}
	}
	return nil
}
