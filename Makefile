.PHONY: args build clean mock proto test %.image %.push

# Recursive wildcard from https://stackoverflow.com/questions/2483182/recursive-wildcards-in-gnu-make.
rwildcard=$(foreach d,$(wildcard $(1:=/*)),$(call rwildcard,$d,$2) $(filter $(subst *,%,$2),$d))

BIN=cmd/lvmctrld/lvmctrld cmd/driverd/driverd
GO=$(call rwildcard,.,*.go)
MOCK=$(addprefix pkg/mock/, diskrpc.mock.go filesystem.mock.go filesystemregistry.mock.go lvmctrldclient.mock.go mount.mock.go volumelocker.mock.go)
PROTO=$(addprefix pkg/proto/, lvmctrld.pb.go lvmctrld_grpc.pb.go diskrpc.pb.go diskrpc_grpc.pb.go)
MANIFEST=$(addsuffix .yaml, $(wildcard deploy/kubernetes/*.url) $(wildcard deploy/kubernetes/*.var))
IMAGE=lvmctrld.image driverd.image
PUSH=lvmctrld.push driverd.push

# https://github.com/kubernetes-csi/external-snapshotter/tags, k8s >= 1.20
export EXTERNAL_SNAPSHOTTER_VERSION=v6.2.2
# https://github.com/kubernetes-csi/external-attacher/tags, k8s >= 1.17
export EXTERNAL_ATTACHER_VERSION=v3.5.0
# https://github.com/kubernetes-csi/external-provisioner/tags, k8s >= 1.20
export EXTERNAL_PROVISIONER_VERSION=v3.4.0
# https://github.com/kubernetes-csi/external-resizer/tags, k8s >= 1.16
export EXTERNAL_RESIZER_VERSION=v1.7.0

VERSION?=$(shell git describe --tags 2>/dev/null || (printf commit-; git rev-parse --short HEAD))
export VERSION
COMMIT?=$(shell git rev-parse --short HEAD)
export COMMIT

# Ensure build parameter changes causes the precedent build to be discarded.
-include .makeargs
ARGS_CURR=commit=$(COMMIT),version=$(VERSION)
ifneq ($(ARGS_CURR),$(ARGS_PREV))
ARGS_DEP=args
endif

ifeq ($(VERSION), test)
IMAGE_PULL_POLICY=Always
else
IMAGE_PULL_POLICY=IfNotPresent
endif
export IMAGE_PULL_POLICY

build: $(BIN) $(MANIFEST)
proto: $(PROTO)
mock: $(MOCK)
image: $(IMAGE)
push: $(PUSH)

args: $(ARGS_CLEAN)
	printf "ARGS_PREV=%s\nARGS_CLEAN=clean\n" $(ARGS_CURR) > .makeargs

clean:
	$(RM) $(BIN) $(PROTO) $(MOCK) $(MANIFEST)

test coverage.txt: mock
	go test -race -coverprofile=coverage.txt -covermode=atomic ./cmd/* ./pkg/*

%: %.bin $(GO) $(ARGS_DEP) | proto
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X main.version=$(VERSION) -X main.commit=$(COMMIT)' -o $@ ./$(@D)

%.pb.go %_grpc.pb.go: %.proto $(ARGS_DEP)
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:. --go-grpc_opt=paths=source_relative $<

%.mock.go: %.mock
	mockgen -package mock -destination $@ `cat $<`

%.url.yaml: %.url $(ARGS_DEP)
	curl -s -o $@ `cat $< | sed 's/@@EXTERNAL_SNAPSHOTTER_VERSION@@/$(EXTERNAL_SNAPSHOTTER_VERSION)/g;s/@@EXTERNAL_ATTACHER_VERSION@@/$(EXTERNAL_ATTACHER_VERSION)/g;s/@@EXTERNAL_PROVISIONER_VERSION@@/$(EXTERNAL_PROVISIONER_VERSION)/g;s/@@EXTERNAL_RESIZER_VERSION@@/$(EXTERNAL_RESIZER_VERSION)/g;'`

%.var.yaml: %.var $(ARGS_DEP)
	envsubst < $< > $@

%.image:
	docker build --platform linux/x86_64 --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -t quay.io/aleofreddi/csi-sanlock-lvm-$*:$(VERSION) -f build/docker/$*/Dockerfile .

%.push: %.image
	docker push quay.io/aleofreddi/csi-sanlock-lvm-$*:$(VERSION)
