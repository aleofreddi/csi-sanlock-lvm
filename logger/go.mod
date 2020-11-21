module github.com/aleofreddi/csi-sanlock-lvm/logger

go 1.13

require (
	github.com/aleofreddi/csi-sanlock-lvm/proto v0.0.0-00010101000000-000000000000
	github.com/golang/protobuf v1.4.0
	github.com/kubernetes-csi/csi-lib-utils v0.3.0
	github.com/kylelemons/godebug v1.1.0
	github.com/stretchr/testify v1.5.1 // indirect
	google.golang.org/grpc v1.23.0
	google.golang.org/protobuf v1.22.0
	k8s.io/klog v1.0.0
)

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.0

replace github.com/google/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a

replace github.com/aleofreddi/csi-sanlock-lvm/proto => ../proto
