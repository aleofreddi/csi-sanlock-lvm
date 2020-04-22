# :.!find . -type f \( -name \*.url -o -name \*.var \)| sed 's@[^.]*$@yaml@g' | sort -u | tr \\n ' '
OBJ=\
./crd-snapshot.storage.k8s.io_volumesnapshotclasses.yaml ./crd-snapshot.storage.k8s.io_volumesnapshotcontents.yaml ./crd-snapshot.storage.k8s.io_volumesnapshots.yaml ./csi-sanlock-lvm-plugin.yaml ./rbac-attacher.yaml ./rbac-provisioner.yaml ./rbac-resizer.yaml ./rbac-snapshot-controller.yaml ./rbac-snapshotter.yaml ./setup-snapshot-controller.yaml 

VERSION?=$(shell git describe --tags 2>/dev/null || (printf commit-; git rev-parse --short HEAD))
EXTERNAL_SNAPSHOTTER_VERSION=v2.0.1
EXTERNAL_ATTACHER_VERSION=v2.1.1
EXTERNAL_PROVISIONER_VERSION=v1.5.0
EXTERNAL_RESIZER_VERSION=v0.4.0

all: build

build: $(OBJ)

clean:
	$(RM) -r $(OBJ)

%.yaml: %.url
	curl -s -o $@ `cat $< | sed 's/@@EXTERNAL_SNAPSHOTTER_VERSION@@/$(EXTERNAL_SNAPSHOTTER_VERSION)/g;s/@@EXTERNAL_ATTACHER_VERSION@@/$(EXTERNAL_ATTACHER_VERSION)/g;s/@@EXTERNAL_PROVISIONER_VERSION@@/$(EXTERNAL_PROVISIONER_VERSION)/g;s/@@EXTERNAL_RESIZER_VERSION@@/$(EXTERNAL_RESIZER_VERSION)/g;'`

%.yaml: %.var
	VERSION='$(VERSION)' envsubst < $< > $@
