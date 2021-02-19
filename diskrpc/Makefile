.PHONY: all proto build mock test clean build-image push-image

include ../common.mk

# :.!find pkg cmd -type f -name \*go -a \! -name \*_test.go | sort -u | tr \\n ' '
SRC=\
pkg/allocator.go pkg/diskrpc.go pkg/mailbox.go 

# :.!find . -type f -name \*_test.go | sort -u | tr \\n ' '
TEST=\
./pkg/allocator_test.go ./pkg/diskrpc_test.go ./pkg/mailbox_test.go 

MOCK=

VPATH=./pkg ./mock

all: build test

build: ;

clean:
	$(RM) -r go.dep coverage.txt ./bin ./mock/*.m[ie].go

go.dep: go.mod $(PROTO)
	go get -t ./... && touch go.dep

mock: go.dep $(MOCK)

proto: ;

test: go.dep $(MOCK) $(PROTO) $(SRC) $(TEST)
	go test -race -coverprofile=coverage.txt -covermode=atomic ./pkg

build-image: ;

push-image:	;
