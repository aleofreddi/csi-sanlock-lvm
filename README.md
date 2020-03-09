# CSI Sanlock-LVM Driver
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Build Status](https://travis-ci.com/aleofreddi/csi-sanlock-lvm.svg?branch=master)](https://travis-ci.com/aleofreddi/csi-sanlock-lvm)
[![Test Coverage](https://codecov.io/gh/aleofreddi/csi-sanlock-lvm/branch/master/graph/badge.svg)](https://codecov.io/gh/aleofreddi/csi-sanlock-lvm) 

`csi-sanlock-lvm` is a CSI driver for LVM and Sanlock.

It comes in handy when you want your nodes to access data on a shared block
device - a typical example being Kubernetes on bare metal with a SAN.

## Maturity

This project is in alpha state, YMMV.

## Features

-   Dynamic volume provisioning
    -   ~~Support both filesystem and block devices~~ (TODO)
    -   ~~Support different filesystems~~ (TODO)
    -   ~~Support different RAID levels~~ (TODO)
    -   Support single node read/write access
-   Online volume extension
-   Online snapshot support
-   ~~Ephemeral volumes~~ (TODO)

## Prerequisite

-   Kubernetes 1.17+
-   `kubectl`

## Limitations

Sanlock might require up to 3 minutes to start, so bringing up a node can take
some time.

Also, Sanlock requires every cluster node to get an unique integer in the range
1-2000. This is implemented using least significant bits of the node ip address,
which is ensured to work as long as all the nodes reside in a proper subnet
(a `/22` block or smaller).

## Deployment

Before deploying the driver you should initialize a shared volume group.

### Initialize a shared volume group

You can either initialize it externally (like using lvm from a node) or use
the provided `deploy/kubernetes-1.17/csi-sanlock-lvm-init.yaml` job template
to get Kubernetes initialize your disks.

To use the `csi-sanlock-lvm-init` job, do the following:

- Adjust the `command` value according to your need, so to create the proper
physical volume(s) and volume group(s);
- When you are ready to run, patch the job `parallelism` to `1` and let the job
initialize the volume(s).

### Deploy the driver

When the volume group setup is complete, go ahead and create a namespace to
accommodate the driver:

```shell
$ kubectl create namespace csi-sanlock-lvm-system
```

And then deploy using `kustomization`:

```shell
$ kubectl apply -k 'https://github.com/aleofreddi/csi-sanlock-lvm/deploy/kubernetes-1.17?ref=v0.1'
```

On a successful installation, all csi pods should be running (with the exception
of the initialization job one, if any):

```shell
$ kubectl -n csi-sanlock-lvm-system get pod
NAME                            READY   STATUS      RESTARTS   AGE
csi-sanlock-lvm-attacher-0      1/1     Running     0          4h41m
csi-sanlock-lvm-init-bwsh7      0/1     Completed   0          4h42m
csi-sanlock-lvm-plugin-b44sh    4/4     Running     0          4h41m
csi-sanlock-lvm-plugin-nnbfz    4/4     Running     1          4h41m
csi-sanlock-lvm-provisioner-0   1/1     Running     0          4h41m
csi-sanlock-lvm-resizer-0       1/1     Running     0          4h41m
csi-sanlock-lvm-snapshotter-0   1/1     Running     0          4h41m
snapshot-controller-0           1/1     Running     0          4h41m
```

## Example application

The examples directory contains an example configuration that will spin up a pvc
and pod using it:

```shell
$ kubectl apply -f examples/pvc.yaml examples/pod.yaml
```

You can also create a snapshot of the volume using the `snap.yaml`:

```shell
$ kubectl apply -f examples/snap.yaml
```

## Building the binaries

If you want to build the driver yourself, you can do so with the following
command from the root directory:

```shell
$ make
```

## Disclaimer

This is not an officially supported Google product.
