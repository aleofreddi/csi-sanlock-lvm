.PHONY: all proto build mock test clean build-image push-image

include ../../common.mk

# :.!find . \( -type f -o -type l \) \( -name \*var -o -name \*url \) | sort -u | sed 's@$@.yaml@g' | tr \\n ' '
MANIFEST=\
./crd-snapshot.storage.k8s.io_volumesnapshotclasses.url.yaml ./crd-snapshot.storage.k8s.io_volumesnapshotcontents.url.yaml ./crd-snapshot.storage.k8s.io_volumesnapshots.url.yaml ./csi-sanlock-lvm-attacher.var.yaml ./csi-sanlock-lvm-driverinfo.var.yaml ./csi-sanlock-lvm-init.var.yaml ./csi-sanlock-lvm-plugin.var.yaml ./csi-sanlock-lvm-provisioner.var.yaml ./csi-sanlock-lvm-resizer.var.yaml ./csi-sanlock-lvm-snapshotter.var.yaml ./csi-sanlock-lvm-socat.var.yaml ./rbac-attacher.url.yaml ./rbac-provisioner.url.yaml ./rbac-resizer.url.yaml ./rbac-snapshot-controller.url.yaml ./rbac-snapshotter.url.yaml ./setup-snapshot-controller.url.yaml 

EXTERNAL_SNAPSHOTTER_VERSION=v5.0.1# k8s>=1.20, v4.0.1 k8s>=1.17
EXTERNAL_ATTACHER_VERSION=v3.4.0# k8s>=1.20
EXTERNAL_PROVISIONER_VERSION=v3.1.0# k8s>=1.20, v2.2.2 k8s>=1.17
EXTERNAL_RESIZER_VERSION=v1.4.0

all: build

build: $(MANIFEST)

clean:
	$(RM) $(MANIFEST)
