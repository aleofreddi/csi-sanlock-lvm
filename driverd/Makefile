.PHONY: test build-image push-image all clean

all:	bin/driverd test

clean:
	$(RM) -rf ./bin

# :.!find . -type f -name \*go -a \! -name \*_test.go | sort -u | tr \\n ' '
SRC=\
./cmd/main.go ./pkg/controllerserver.go ./pkg/fs.go ./pkg/identityserver.go ./pkg/init.go ./pkg/listener.go ./pkg/lvmctrldclient.go ./pkg/nodeserver.go ./pkg/tagencoder.go ./pkg/tags.go 

bin/driverd: $(SRC)
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o ./bin/driverd ./cmd

test: $(SRC)
	go test ./cmd ./pkg

build-image:
	docker build -t quay.io/aleofreddi/csi-lvm-sanlock:v0.1 -f Dockerfile ..

push-image: build-image
	docker push quay.io/aleofreddi/csi-lvm-sanlock:v0.1
