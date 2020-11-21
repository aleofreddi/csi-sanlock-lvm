.PHONY: godep
.DEFAULT_GOAL := all

VERSION?=$(shell git describe --tags 2>/dev/null || (printf commit-; git rev-parse --short HEAD))
export VERSION

COMMIT?=$(shell git rev-parse --short HEAD)
export COMMIT

%.pb.go: %.proto
	protoc --go_out=plugins=grpc:. --go_opt=paths=source_relative $<

mock/%.mi.go: %.go go.dep
	mockgen -package mock -destination $@ -source $<

mock/%.me.go: %.mock go.dep
	mockgen -package mock -destination $@ `cat $<`

%.url.yaml: %.url
	curl -s -o $@ `cat $< | sed 's/@@EXTERNAL_SNAPSHOTTER_VERSION@@/$(EXTERNAL_SNAPSHOTTER_VERSION)/g;s/@@EXTERNAL_ATTACHER_VERSION@@/$(EXTERNAL_ATTACHER_VERSION)/g;s/@@EXTERNAL_PROVISIONER_VERSION@@/$(EXTERNAL_PROVISIONER_VERSION)/g;s/@@EXTERNAL_RESIZER_VERSION@@/$(EXTERNAL_RESIZER_VERSION)/g;'`

%.var.yaml: %.var
	envsubst < $< > $@
