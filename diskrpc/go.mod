module github.com/aleofreddi/csi-sanlock-lvm/diskrpc

go 1.13

require (
	github.com/aleofreddi/csi-sanlock-lvm/proto v1.0.0
	github.com/golang/protobuf v1.4.3
	github.com/google/uuid v1.2.0
	github.com/kubernetes-csi/csi-lib-utils v0.9.1
	github.com/ncw/directio v1.0.5
	github.com/slok/noglog v0.2.0 // indirect
	google.golang.org/grpc v1.29.0
	k8s.io/klog v1.0.0
)

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.0

replace github.com/golang/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a

replace github.com/aleofreddi/csi-sanlock-lvm/proto => ../proto

