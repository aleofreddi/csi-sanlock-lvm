module github.com/aleofreddi/csi-sanlock-lvm/driverd

go 1.13

require (
	github.com/aleofreddi/csi-sanlock-lvm/diskrpc v1.0.0
	github.com/aleofreddi/csi-sanlock-lvm/logger v1.0.0
	github.com/aleofreddi/csi-sanlock-lvm/proto v1.0.0
	github.com/container-storage-interface/spec v1.2.0
	github.com/golang/mock v1.5.0
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/google/go-cmp v0.5.0
	github.com/google/uuid v1.2.0
	github.com/kubernetes-csi/csi-lib-utils v0.9.1 // indirect
	github.com/ncw/directio v1.0.5 // indirect
	github.com/pkg/math v0.0.0-20141027224758-f2ed9e40e245
	github.com/slok/noglog v0.2.0 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	google.golang.org/grpc v1.29.0
	google.golang.org/protobuf v1.25.0
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.12.2
	rsc.io/quote/v3 v3.1.0 // indirect
	sigs.k8s.io/structured-merge-diff v0.0.0-20190525122527-15d366b2352e // indirect
)

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.0

replace github.com/google/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a

replace github.com/golang/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a

replace github.com/aleofreddi/csi-sanlock-lvm/proto => ../proto

replace github.com/aleofreddi/csi-sanlock-lvm/logger => ../logger

replace github.com/aleofreddi/csi-sanlock-lvm/diskrpc => ../diskrpc
