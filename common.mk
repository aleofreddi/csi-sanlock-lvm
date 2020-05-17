VERSION?=$(shell git describe --tags 2>/dev/null || (printf commit-; git rev-parse --short HEAD))

%.pb.go: %.proto
	protoc --go_out=plugins=grpc:. $<

mock/%.int.go: %.go
	mockgen -package mock -destination $@ -source $<

mock/%.ext.go: %.mock
	mockgen -package mock -destination $@ `cat $<`

%.pb.go: %.proto
	protoc --go_out=plugins=grpc:. --go_out=proto --go_opt=paths=source_relative $<
