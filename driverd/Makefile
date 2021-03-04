.PHONY: all proto build mock test clean build-image push-image

include ../common.mk

# :.!find pkg cmd -type f -name \*go -a \! -name \*_test.go | sort -u | tr \\n ' '
SRC=\
cmd/main.go pkg/baseserver.go pkg/controllerserver.go pkg/diskrpcservice.go pkg/filesystem.go pkg/identityserver.go pkg/listener.go pkg/lvmctrldclient.go pkg/nodeserver.go pkg/tagencoder.go pkg/tags.go pkg/volumeinfo.go pkg/volumelock.go pkg/volumeref.go 

# :.!find . -type f -name \*_test.go | sort -u | tr \\n ' '
TEST=\
./pkg/cmpmatcher_test.go ./pkg/controllerserver_test.go ./pkg/nodeserver_test.go ./pkg/tagencoder_test.go ./pkg/volumelock_test.go 

MOCK=\
mock/diskrpc.me.go mock/filesystem.mi.go mock/lvmctrldclient.me.go mock/mount.me.go mock/volumelocker.me.go

VPATH=./pkg ./mock ../lvmctrld/proto

all: build test

build: bin/driverd

clean:
	$(RM) -r go.dep coverage.txt ./bin ./mock/*.m[ie].go

go.dep: go.mod $(PROTO)
	go get -t ./... && touch go.dep

mock: go.dep $(MOCK)

proto: ;

bin/driverd: go.dep $(SRC) $(PROTO)
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X main.version=$(VERSION) -X main.commit=$(COMMIT)' -o ./bin/driverd ./cmd

test: go.dep $(MOCK) $(PROTO) $(SRC) $(TEST)
	go test -race -coverprofile=coverage.txt -covermode=atomic ./cmd ./pkg

build-image:
	docker build --build-arg VERSION='$(VERSION)' -t quay.io/aleofreddi/csi-lvm-sanlock:'$(VERSION)' -f Dockerfile ..

push-image:	build-image
	docker push quay.io/aleofreddi/csi-lvm-sanlock:'$(VERSION)'
