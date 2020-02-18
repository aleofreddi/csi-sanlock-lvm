.PHONY: test build-image push-image all clean

all:	bin/lvmctrld test

clean:
	$(RM) -r ./bin ./proto/*.pb.go

%.pb.go: %.proto
	protoc --go_out=plugins=grpc:. $<

# :.!(find . -type f -name \*go -a \! -name \*_test.go; echo ./proto/lvmctrld.pb.go) | sort -u | tr \\n ' '
SRC=\
./cmd/main.go ./pkg/grpclogger.go ./pkg/listener.go ./pkg/lock.go ./pkg/lvmctrldserver.go ./proto/lvmctrld.pb.go

bin/lvmctrld: $(SRC)
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o ./bin/lvmctrld ./cmd

test: $(SRC)
	go test ./cmd ./pkg

build-image:
	docker build -t quay.io/aleofreddi/lvmctrld:v0.1 -f Dockerfile ..

push-image: build-image
	docker push quay.io/aleofreddi/lvmctrld:v0.1
