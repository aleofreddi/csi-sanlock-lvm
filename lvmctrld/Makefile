.PHONY: all build mock test clean build-image push-image

include ../common.mk

# :.!(find . -type f -name \*go -a \! -name \*_test.go; echo ./proto/lvmctrld.pb.go) | sort -u | tr \\n ' '
SRC=\
./cmd/main.go ./pkg/grpclogger.go ./pkg/listener.go ./pkg/lock.go ./pkg/lvmctrldserver.go ./proto/lvmctrld.pb.go

# :.!find . -type f -name \*_test.go | sort -u | tr \\n ' '
TEST=\
./pkg/fakecommander_test.go ./pkg/lvmctrldserver_test.go 

VPATH=./pkg

MOCKS=

all:	build test

build:  bin/lvmctrld

clean:
	$(RM) -r ./bin ./mock ./proto/*.pb.go

mock: $(MOCKS)

bin/lvmctrld: $(SRC)
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X main.version=$(VERSION)' -o ./bin/lvmctrld ./cmd

test: mock $(SRC) $(TEST)
	go test -race -coverprofile=coverage.txt -covermode=atomic ./cmd ./pkg

build-image:
	docker build --build-arg VERSION='$(VERSION)' -t quay.io/aleofreddi/lvmctrld:'$(VERSION)' -f Dockerfile ..

push-image: build-image
	docker push quay.io/aleofreddi/lvmctrld:'$(VERSION)'
