.PHONY: all build test clean build-image push-image

# :.!find . -type f -name \*go -a \! -name \*_test.go | sort -u | tr \\n ' '
SRC=\
./cmd/main.go ./pkg/controllerserver.go ./pkg/fs.go ./pkg/identityserver.go ./pkg/init.go ./pkg/listener.go ./pkg/lvmctrldclient.go ./pkg/nodeserver.go ./pkg/tagencoder.go ./pkg/tags.go 

# :.!find . -type f -name \*_test.go | sort -u | tr \\n ' '
TEST=\
./pkg/tagencoder_test.go 

VERSION=$(shell git describe --tags 2>/dev/null || (printf commit-; git rev-parse --short HEAD))

all:	build test

build:  bin/driverd

clean:
	$(RM) -r ./bin

bin/driverd: $(SRC)
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X main.version=$(VERSION)' -o ./bin/driverd ./cmd

test: $(SRC) $(TEST)
	go test -race -coverprofile=coverage.txt -covermode=atomic ./cmd ./pkg

build-image:
	docker build -t quay.io/aleofreddi/csi-lvm-sanlock:$(VERSION) -f Dockerfile ..

push-image: build-image
	docker push quay.io/aleofreddi/csi-lvm-sanlock:$(VERSION)
