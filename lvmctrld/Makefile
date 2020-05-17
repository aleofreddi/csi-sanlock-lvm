.PHONY: all proto build mock test clean build-image push-image

include ../common.mk

# :.!find pkg cmd -type f -name \*go -a \! -name \*_test.go | sort -u | tr \\n ' '
SRC=\
cmd/main.go pkg/commander.go pkg/grpclogger.go pkg/listener.go pkg/lock.go pkg/lvmctrldserver.go 

# :.!find . -type f -name \*_test.go | sort -u | tr \\n ' '
TEST=\
./pkg/fakecommander_test.go ./pkg/lvmctrldserver_test.go 

PROTO=./proto/lvmctrld.pb.go

MOCKS=

VPATH=./pkg ./proto

all: build test

build: bin/lvmctrld

clean:
	$(RM) -r ./bin ./mock/*.go ./proto/*.go

mock: go.dep $(MOCKS)

proto: go.dep $(PROTO)

bin/lvmctrld: go.dep $(SRC) $(PROTO)
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X main.version=$(VERSION)' -o ./bin/lvmctrld ./cmd

test: go.dep mock proto $(SRC) $(TEST)
	go test -race -coverprofile=coverage.txt -covermode=atomic ./cmd ./pkg

build-image:
	docker build --build-arg VERSION='$(VERSION)' -t quay.io/aleofreddi/lvmctrld:'$(VERSION)' -f Dockerfile ..

push-image:	build-image
	docker push quay.io/aleofreddi/lvmctrld:'$(VERSION)'
