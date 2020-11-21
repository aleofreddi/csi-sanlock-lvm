.PHONY: all proto build mock test clean build-image push-image

include ../common.mk

PROTO=lvmctrld.pb.go diskrpc.pb.go

all: build

build: $(PROTO)

clean:
	$(RM) -r *.go

go.dep: go.mod $(PROTO)
	go get -t ./... && touch go.dep

proto: $(PROTO)
