.PHONY: godep

VERSION?=$(shell git describe --tags 2>/dev/null || (printf commit-; git rev-parse --short HEAD))

proto/%.pb.go: %.proto
	protoc --go_out=plugins=grpc:. --go_opt=paths=source_relative $<

mock/%.mi.go: %.go
	mockgen -package mock -destination $@ -source $<

mock/%.me.go: %.mock
	mockgen -package mock -destination $@ `cat $<`

godep:
	go get ./...
