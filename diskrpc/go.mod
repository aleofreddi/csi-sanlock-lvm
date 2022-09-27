module github.com/aleofreddi/csi-sanlock-lvm/diskrpc

go 1.13

require (
	github.com/aleofreddi/csi-sanlock-lvm/proto v1.0.0
	github.com/docker/spdystream v0.0.0-20160310174837-449fdfce4d96 // indirect
	github.com/go-openapi/spec v0.0.0-20160808142527-6aced65f8501 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.3.0
	github.com/kubernetes-csi/csi-lib-utils v0.11.0
	github.com/ncw/directio v1.0.5
	github.com/slok/noglog v0.2.0 // indirect
	golang.org/x/net v0.0.0-20220927171203-f486391704dc // indirect
	golang.org/x/sys v0.0.0-20220927170352-d9d178bc13c6 // indirect
	google.golang.org/genproto v0.0.0-20220927151529-dcaddaf36704 // indirect
	google.golang.org/grpc v1.49.0
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/klog v1.0.0
)

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.0

replace github.com/golang/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a

replace github.com/aleofreddi/csi-sanlock-lvm/proto => ../proto
