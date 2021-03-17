# CSI Sanlock-LVM Driver

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Build Status](https://travis-ci.com/aleofreddi/csi-sanlock-lvm.svg?branch=master)](https://travis-ci.com/aleofreddi/csi-sanlock-lvm)
[![Test Coverage](https://codecov.io/gh/aleofreddi/csi-sanlock-lvm/branch/master/graph/badge.svg)](https://codecov.io/gh/aleofreddi/csi-sanlock-lvm)

`csi-sanlock-lvm` is a CSI driver for LVM and Sanlock.

It comes in handy when you want your nodes to access data on a shared block
device - a typical example being Kubernetes on bare metal on a SAN (storage area
network).

## Maturity

This project is in alpha state, YMMV.

## Features

- Dynamic volume provisioning
    - Support both filesystem and block devices
    - Support different filesystems
    - ~~Support different RAID levels~~ (TODO)
    - Support single node read/write access
- Online volume extension
- Online snapshot support
- ~~Ephemeral volumes~~ (TODO)

## Prerequisite

- Kubernetes 1.17+
- `kubectl`

## Limitations

Sanlock might require up to 3 minutes to start, so bringing up a node can take
some time.

Also, Sanlock requires every cluster node to get an unique integer in the range
1-2000. This is implemented using least significant bits of the node ip address,
which works as long as all the nodes reside in a subnet that contains at most
2000 addresses (a `/22` subnet or smaller).

## Installation

This chapter describes a step-by-step procedure to get csi-sanlock-lvm running
on your cluster.

### Initialize a shared volume group

Before deploying the driver, you need to have at least a shared volume group set
up, as well as some logical volumes dedicated for csi-sanlock-lvm rpc mechanism.
You can use the provided `csi-sanlock-lvm-init` pod to initialize lvm as
follows:

```shell
# Extract the kubernetes major.minor version.
kver="$(kubectl version -o json | jq -r '.serverVersion.major + "." + .serverVersion.minor')"

kubectl apply -f "https://raw.githubusercontent.com/aleofreddi/csi-sanlock-lvm/v0.4.1/deploy/kubernetes-$kver/csi-sanlock-lvm-init.var.yaml"
```

Then attach the init pod as follows and initialize the VG.

```shell
kubectl attach -it csi-sanlock-lvm-init
```

Within the init shell, initialize the shared volume group and the rpc logical
volumes:

```shell
# Adjust your devices before running this!
vgcreate --shared vg01 [/dev/device1 ... /dev/deviceN]

# Create the csi-rpc-data logical volume.
lvcreate -L 8m -n csi-rpc-data \
  --add-tag csi-sanlock-lvm.vleo.net/rpcRole=data vg01 &&
  lvchange -a n vg01/csi-rpc-data

# Create the csi-rpc-lock logical volume.
lvcreate -L 512b -n csi-rpc-lock \
  --add-tag csi-sanlock-lvm.vleo.net/rpcRole=lock vg01 &&
  lvchange -a n vg01/csi-rpc-lock

# Initialization complete, terminate the pod successfully.
exit 0
````

Now cleanup the init pod:

```shell
kubectl delete po csi-sanlock-lvm-init
```

### Deploy the driver

When the volume group setup is complete, go ahead and create a namespace to
accommodate the driver:

```shell
kubectl create namespace csi-sanlock-lvm-system
```

And then deploy using `kustomization` (adjust the kubernetes version in the link
as needed):

```shell
# Extract the kubernetes major.minor version.
kver="$(kubectl version -o json | jq -r '.serverVersion.major + "." + .serverVersion.minor')"

# Install the csi-sanlock-lvm driver.
kubectl apply -k "https://github.com/aleofreddi/csi-sanlock-lvm/deploy/kubernetes-$kver?ref=v0.4.1"
```

It might take up to 3 minutes for the csi plugin to become `Running` on each
node, and all the containers should be ready (for the plugin ones that would
be `4/4`). To check the current status you can use the following command:

```shell
kubectl -n csi-sanlock-lvm-system get pod
```

You should get an output similar to:

```
NAME                            READY   STATUS    RESTARTS   AGE
csi-sanlock-lvm-attacher-0      1/1     Running   0          2m15s
csi-sanlock-lvm-plugin-cm7h6    4/4     Running   0          2m13s
csi-sanlock-lvm-plugin-zkw84    4/4     Running   0          2m13s
csi-sanlock-lvm-provisioner-0   1/1     Running   0          2m14s
csi-sanlock-lvm-resizer-0       1/1     Running   0          2m14s
csi-sanlock-lvm-snapshotter-0   1/1     Running   0          2m14s
snapshot-controller-0           1/1     Running   0          2m14s
```

### Setup storage and snapshot classes

To enable the csi-sanlock-lvm driver you need to refer it from a storage class.

In particular, you need to set up a `StorageClass` object to manage volumes, and
optionally a `VolumeSnapshotClass` object to manage snapshots.

Configuration examples are provided at `conf/csi-sanlock-lvm-storageclass.yaml`
and `conf/csi-sanlock-lvm-volumesnapshotclass.yaml`.

The following storage class parameters are supported:

- `volumeGroup` _(required)_: the volume group to use when provisioning logical
  volumes.

The following volume snapshot class parameters are supported:

- `maxSizePct` _(optional)_: maximum snapshot size as percentage of its origin
  size;
- `maxSize` _(optional)_: maximum snapshot size.

If multiple maximum size settings are provided, the snapshot will use the least
one.

## Example application

The `examples` directory contains an example configuration that will spin up a
pvc and pod using it:

```shell
kubectl apply -f examples/pvc.yaml examples/pod.yaml
```

You can also create a snapshot of the volume using the `snap.yaml`:

```shell
kubectl apply -f examples/snap.yaml
```

## Building the binaries

### Requirements

To build the project, you need:

* A recent version of the golang compiler;
* GNU make;
* protoc compiler.

To generate go implementations from proto files, you need to install
`protoc-gen-go` as follows:

```shell
go get github.com/golang/protobuf/protoc-gen-go
```

### Build binaries

If you want to build the driver yourself, you can do so invoking make:

```shell
make
```

### Build docker images

Similarly, you want to build docker images for the driver using
the `build-image` target:

```shell
make build-image
```

## Security

Each node exposes a CSI server via a socket to Kubernetes: access to such a
socket grants direct access to any cluster volume. The same holds true for the
RPC data volume which is used for inter-node communication.

## Disclaimer

This is not an officially supported Google product.
