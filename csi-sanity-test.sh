#!/bin/bash
#
# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

die() {
	echo "$@" >&2
	exit 10
}

if ! which csi-sanity 2>/dev/null; then
	echo "$0 requires csi-sanity: go get github.com/kubernetes-csi/csi-test/cmd/csi-sanity" >&2
	exit 1
fi

if [[ `uname` != Linux ]]; then
    echo "$0 requires a linux machine" >&2
    exit 2
fi

rollback=:

tmpdir=`mktemp -d /tmp/csi-sanity-$$.XXXXX` || die Failed to allocate a temporary directory
rollback="echo Removing temporary directory \"$tmpdir\"; rm -rf \"$tmpdir\"; $rollback"
trap "$rollback" EXIT

imgfile="$tmpdir/img"
fallocate -l 1GiB "$imgfile" || die Failed to allocate test image file

device=`losetup --show -f "$imgfile"` || die Failed to setup loopback device
rollback="echo Detaching $device; losetup -d $device; $rollback"
trap "$rollback" EXIT

pvcreate -f $device || die Failed to create physical device

vgcreate vg_csi_sanity_$$ $device || die Failed to create volume group

lvmctrld_sock="unix://$tmpdir/lvmctrld.sock"
./lvmctrld/bin/lvmctrld --listen "$lvmctrld_sock" &
rollback="echo Killing lvmctrld pid $!; kill $! 2>/dev/null; sleep 1; kill -9 $! 2>/dev/null; $rollback"
trap "$rollback" EXIT

driverd_sock="unix://$tmpdir/driverd.sock"
./driverd/bin/driverd --lvmctrld "$lvmctrld_sock" --node-id node --listen "$driverd_sock" &
rollback="echo Killing driverd pid $!; kill $! 2>/dev/null; sleep 1; kill -9 $! 2>/dev/null; $rollback"
trap "$rollback" EXIT

param_file="$tmpdir/params"
cat > "$param_file" <<EOF
filesystem: ext4
volumeGroup: vg_csi_sanity_$$
EOF

csi-sanity \
	--csi.endpoint "$driverd_sock" \
	--csi.mountdir "$tmpdir/mount" \
	--csi.stagingdir "$tmpdir/staging" \
	--csi.testvolumeparameters "$param_file" \
	--csi.testvolumesize $((1024*1024)) \

exit $?
