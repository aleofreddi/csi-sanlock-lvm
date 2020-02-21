module github.com/aleofreddi/csi-sanlock-lvm/lvmctrld

go 1.13

require (
	github.com/golang/protobuf v1.3.3
	github.com/kubernetes-csi/csi-lib-utils v0.3.0
	github.com/stretchr/testify v1.4.0 // indirect
	google.golang.org/grpc v1.23.0
	k8s.io/klog v1.0.0
)

replace gopkg.in/russross/blackfriday.v2 => github.com/russross/blackfriday/v2 v2.0.1

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.0

replace github.com/google/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a
