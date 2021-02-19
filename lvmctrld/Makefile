.PHONY: all proto build mock test clean build-image push-image

include ../common.mk

# :.!find pkg cmd -type f -name \*go -a \! -name \*_test.go | sort -u | tr \\n ' '
SRC=\
cmd/main.go pkg/runner.go pkg/listener.go pkg/lock.go pkg/lvmctrldserver.go

# :.!find . -type f -name \*_test.go | sort -u | tr \\n ' '
TEST=\
./pkg/fakerunner_test.go ./pkg/lvmctrldserver_test.go

PROTO=../proto/lvmctrld.pb.go

MOCK=

VPATH=./pkg

all: build test

build: bin/lvmctrld

clean:
	$(RM) -r go.dep coverage.txt ./bin ./mock/*.m[ie].go

go.dep: go.mod $(PROTO)
	go get -t ./... && touch go.dep

mock: go.dep $(MOCK)

bin/lvmctrld: go.dep $(SRC) $(PROTO)
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X main.version=$(VERSION) -X main.commit=$(COMMIT)' -o ./bin/lvmctrld ./cmd

proto: $(PROTO)

test: go.dep $(MOCK) $(PROTO) $(SRC) $(TEST)
	go test -race -coverprofile=coverage.txt -covermode=atomic ./cmd ./pkg

build-image:
	docker build --build-arg VERSION='$(VERSION)' -t quay.io/aleofreddi/lvmctrld:'$(VERSION)' -f Dockerfile ..

push-image:	build-image
	docker push quay.io/aleofreddi/lvmctrld:'$(VERSION)'
