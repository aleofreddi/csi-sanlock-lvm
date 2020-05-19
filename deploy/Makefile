.PHONY: all proto build mock test clean build-image push-image

include ../common.mk

# :.!find . \( -type f -o -type l \) \( -name \*var -o -name \*url \) | sort -u | sed 's@$@.yaml@g' | tr \\n ' '
MANIFEST=\
./kubernetes-1.17/crd-snapshot.storage.k8s.io_volumesnapshotclasses.url.yaml ./kubernetes-1.17/crd-snapshot.storage.k8s.io_volumesnapshotcontents.url.yaml ./kubernetes-1.17/crd-snapshot.storage.k8s.io_volumesnapshots.url.yaml ./kubernetes-1.17/csi-sanlock-lvm-attacher.var.yaml ./kubernetes-1.17/csi-sanlock-lvm-driverinfo.var.yaml ./kubernetes-1.17/csi-sanlock-lvm-init.var.yaml ./kubernetes-1.17/csi-sanlock-lvm-networkpolicy.var.yaml ./kubernetes-1.17/csi-sanlock-lvm-plugin.var.yaml ./kubernetes-1.17/csi-sanlock-lvm-provisioner.var.yaml ./kubernetes-1.17/csi-sanlock-lvm-resizer.var.yaml ./kubernetes-1.17/csi-sanlock-lvm-snapshotter.var.yaml ./kubernetes-1.17/csi-sanlock-lvm-socat.var.yaml ./kubernetes-1.17/rbac-attacher.url.yaml ./kubernetes-1.17/rbac-provisioner.url.yaml ./kubernetes-1.17/rbac-resizer.url.yaml ./kubernetes-1.17/rbac-snapshot-controller.url.yaml ./kubernetes-1.17/rbac-snapshotter.url.yaml ./kubernetes-1.17/setup-snapshot-controller.url.yaml ./kubernetes-1.18/crd-snapshot.storage.k8s.io_volumesnapshotclasses.url.yaml ./kubernetes-1.18/crd-snapshot.storage.k8s.io_volumesnapshotcontents.url.yaml ./kubernetes-1.18/crd-snapshot.storage.k8s.io_volumesnapshots.url.yaml ./kubernetes-1.18/csi-sanlock-lvm-attacher.var.yaml ./kubernetes-1.18/csi-sanlock-lvm-driverinfo.var.yaml ./kubernetes-1.18/csi-sanlock-lvm-init.var.yaml ./kubernetes-1.18/csi-sanlock-lvm-networkpolicy.var.yaml ./kubernetes-1.18/csi-sanlock-lvm-plugin.var.yaml ./kubernetes-1.18/csi-sanlock-lvm-provisioner.var.yaml ./kubernetes-1.18/csi-sanlock-lvm-resizer.var.yaml ./kubernetes-1.18/csi-sanlock-lvm-snapshotter.var.yaml ./kubernetes-1.18/csi-sanlock-lvm-socat.var.yaml ./kubernetes-1.18/rbac-attacher.url.yaml ./kubernetes-1.18/rbac-provisioner.url.yaml ./kubernetes-1.18/rbac-resizer.url.yaml ./kubernetes-1.18/rbac-snapshot-controller.url.yaml ./kubernetes-1.18/rbac-snapshotter.url.yaml ./kubernetes-1.18/setup-snapshot-controller.url.yaml 

EXTERNAL_SNAPSHOTTER_VERSION=v2.0.1
EXTERNAL_ATTACHER_VERSION=v2.1.1
EXTERNAL_PROVISIONER_VERSION=v1.5.0
EXTERNAL_RESIZER_VERSION=v0.4.0

all: build

build: $(MANIFEST)

clean:
	$(RM) $(MANIFEST)
