#!/usr/bin/env bash
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
    echo "ERROR: $@" >&2
    exit 10
}

me="$(basename $0)"
rootdir="$(dirname $0)"
rollback=:
verbosity=3

while getopts "hv:" arg; do
  case $arg in
  v)
    verbosity="$OPTARG"
    ;;
  *)
    echo Usage: "$(basename "$0")" [-v verbosity] [-- csi-sanity-params] >&2
    exit 1
    ;;
  esac
done
shift $((OPTIND-1))

[[ "$(uname | tr '[A-Z]' '[a-z]')" = linux ]] || die "$me requires a Linux machine"

waitForSocket() {
	while [[ ! -r "$1" ]]; do
		printf .
		sleep 1
	done
	echo
}

for i in \
    'csi-sanity:install using `go get github.com/kubernetes-csi/csi-test/cmd/csi-sanity`' \
    'fallocate:install the proper package' \
    'losetup:install the proper package' \
    'pvcreate:install lvm tools' \
    'vgcreate:install lvm tools' \
    './cmd/lvmctrld/lvmctrld:run "make" to build the driver' \
    './cmd/driverd/driverd:run "make" to build the driver' \
; do
    IFS=: read f d <<<"$i"
    which "$f" >/dev/null 2>&1 || die "$me requires $f, $d"
done

tmpdir="$(mktemp -d /tmp/csi-sanity-$$.XXXXX)" || die Failed to allocate a temporary directory
rollback="echo Removing temporary directory \"$tmpdir\"; rm -rf \"$tmpdir\"; $rollback"
trap "(trap '' INT; $rollback)" EXIT

imgfile="$tmpdir/img"
fallocate -l 1GiB "$imgfile" || die Failed to allocate test image file

device="$(losetup --show -f "$imgfile")" || die Failed to setup loopback device
rollback="echo Detaching $device; losetup -d $device; $rollback"
trap "(trap '' INT; $rollback)" EXIT

pvcreate -f "$device" || die Failed to create physical device

vgcreate -s $((1024*1024))b vg_csi_sanity_$$ "$device" || die Failed to create volume group
rollback="echo Bringing down vg_csi_sanity_$$; vgchange -a n vg_csi_sanity_$$; $rollback"

lvcreate -L 512b -n rpc-lock --addtag csi-sanlock-lvm.vleo.net/rpcRole=lock vg_csi_sanity_$$ || die Failed to create rpc lock logical volume
lvcreate -L 8m -n rpc-data --addtag csi-sanlock-lvm.vleo.net/rpcRole=data vg_csi_sanity_$$ || die Failed to create rpc data logical volume

lvmctrld_sock="unix://$tmpdir/lvmctrld.sock"
"$rootdir"/cmd/lvmctrld/lvmctrld --listen "$lvmctrld_sock" --no-lock -v "$verbosity" &
rollback="echo Killing lvmctrld pid $!; kill $! 2>/dev/null; sleep 1; kill -9 $! 2>/dev/null; $rollback"
trap "(trap '' INT; $rollback)" EXIT
echo Waiting for lvmctrld to spin up...
waitForSocket "$tmpdir/lvmctrld.sock"

driverd_sock="unix://$tmpdir/driverd.sock"
"$rootdir"/cmd/driverd/driverd --lvmctrld "$lvmctrld_sock" --listen "$driverd_sock" -v "$verbosity" &
rollback="echo Killing driverd pid $!; kill $! 2>/dev/null; sleep 1; kill -9 $! 2>/dev/null; $rollback"
trap "(trap '' INT; $rollback)" EXIT
echo Waiting for driverd to spin up...
waitForSocket "$tmpdir/driverd.sock"

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
    --csi.testvolumeexpandsize $((2*1024*1024)) \
    "$@"

exit $?
