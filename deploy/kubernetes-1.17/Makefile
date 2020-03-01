# :.!find . -type f -name \*.url | sed 's@\.url$@.yaml@g' | sort -u | tr \\n ' '
OBJ=\
./crd-snapshot.storage.k8s.io_volumesnapshotclasses.yaml ./crd-snapshot.storage.k8s.io_volumesnapshotcontents.yaml ./crd-snapshot.storage.k8s.io_volumesnapshots.yaml ./rbac-attacher.yaml ./rbac-provisioner.yaml ./rbac-resizer.yaml ./rbac-snapshot-controller.yaml ./rbac-snapshotter.yaml ./setup-snapshot-controller.yaml 

all: build

build: $(OBJ)

clean:
	$(RM) -r $(SRC)

%.yaml: %.url
	curl -o $@ `cat $<`
