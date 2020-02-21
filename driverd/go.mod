module github.com/aleofreddi/csi-sanlock-lvm/driverd

go 1.13

require (
	github.com/aleofreddi/csi-sanlock-lvm/lvmctrld v1.0.0
	github.com/container-storage-interface/spec v1.2.0
	github.com/golang/protobuf v1.3.3
	github.com/kubernetes-csi/csi-lib-utils v0.3.0
	github.com/slok/noglog v0.2.0 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	golang.org/x/net v0.0.0-20200114155413-6afb5195e5aa
	golang.org/x/sys v0.0.0-20200124204421-9fbb57f87de9 // indirect
	google.golang.org/genproto v0.0.0-20200128133413-58ce757ed39b // indirect
	google.golang.org/grpc v1.27.0
	k8s.io/apimachinery v0.17.2 // indirect
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.12.2
	k8s.io/utils v0.0.0-20181102055113-1bd4f387aa67 // indirect
)

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.0

replace github.com/google/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a

replace github.com/golang/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a

replace github.com/aleofreddi/csi-sanlock-lvm/lvmctrld => ../lvmctrld
