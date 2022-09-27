module github.com/aleofreddi/csi-sanlock-lvm/driverd

go 1.16

require (
	github.com/aleofreddi/csi-sanlock-lvm/diskrpc v1.0.0
	github.com/aleofreddi/csi-sanlock-lvm/logger v1.0.0
	github.com/aleofreddi/csi-sanlock-lvm/proto v1.0.0
	github.com/container-storage-interface/spec v1.6.0
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.5.9
	github.com/google/uuid v1.3.0
	github.com/ncw/directio v1.0.5 // indirect
	github.com/pkg/math v0.0.0-20141027224758-f2ed9e40e245
	github.com/slok/noglog v0.2.0 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	golang.org/x/net v0.0.0-20220927171203-f486391704dc
	golang.org/x/sys v0.0.0-20220927170352-d9d178bc13c6 // indirect
	google.golang.org/genproto v0.0.0-20220927151529-dcaddaf36704 // indirect
	google.golang.org/grpc v1.49.0
	google.golang.org/protobuf v1.28.1
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.12.2
	k8s.io/utils v0.0.0-20220922133306-665eaaec4324
	rsc.io/quote/v3 v3.1.0 // indirect
	sigs.k8s.io/structured-merge-diff v0.0.0-20190525122527-15d366b2352e // indirect
)

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.0

replace github.com/google/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a

replace github.com/golang/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a

replace github.com/aleofreddi/csi-sanlock-lvm/proto => ../proto

replace github.com/aleofreddi/csi-sanlock-lvm/logger => ../logger

replace github.com/aleofreddi/csi-sanlock-lvm/diskrpc => ../diskrpc
