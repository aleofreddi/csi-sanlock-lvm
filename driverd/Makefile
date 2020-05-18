.PHONY: all proto build mock test clean build-image push-image

include ../common.mk

# :.!find pkg cmd -type f -name \*go -a \! -name \*_test.go | sort -u | tr \\n ' '
SRC=\
cmd/main.go pkg/controllerserver.go pkg/fs.go pkg/identityserver.go pkg/init.go pkg/listener.go pkg/lvmctrldclient.go pkg/nodeserver.go pkg/tagencoder.go pkg/tags.go 

# :.!find . -type f -name \*_test.go | sort -u | tr \\n ' '
TEST=\
./pkg/tagencoder_test.go 

PROTO=

MOCK=\
mock/fs.mi.go mock/lvmctrld.pb.mi.go mock/lvmctrldclient.mi.go mock/mount.me.go

VPATH=./pkg ./mock ../lvmctrld/proto

all: build test

build: bin/driverd

clean:
	$(RM) -r go.dep ./bin ./mock/*.go

go.dep: go.mod $(PROTO)
	go get -t ./... && touch go.dep

mock: go.dep $(MOCK)

proto: $(PROTO)

bin/driverd: go.dep $(SRC) $(PROTO)
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X main.version=$(VERSION)' -o ./bin/driverd ./cmd

test: go.dep $(MOCK) $(PROTO) $(SRC) $(TEST)
	go test -race -coverprofile=coverage.txt -covermode=atomic ./cmd ./pkg

build-image:
	docker build --build-arg VERSION='$(VERSION)' -t quay.io/aleofreddi/csi-lvm-sanlock:'$(VERSION)' -f Dockerfile ..

push-image:	build-image
	docker push quay.io/aleofreddi/csi-lvm-sanlock:'$(VERSION)'
