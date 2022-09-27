module github.com/aleofreddi/csi-sanlock-lvm/lvmctrld

go 1.16

require (
	github.com/aleofreddi/csi-sanlock-lvm/logger v1.0.0
	github.com/aleofreddi/csi-sanlock-lvm/proto v1.0.0
	github.com/golang/protobuf v1.5.2
	github.com/kylelemons/godebug v1.1.0
	golang.org/x/net v0.0.0-20220927171203-f486391704dc // indirect
	golang.org/x/sys v0.0.0-20220927170352-d9d178bc13c6 // indirect
	google.golang.org/genproto v0.0.0-20220927151529-dcaddaf36704 // indirect
	google.golang.org/grpc v1.49.0
	k8s.io/klog v1.0.0
)

replace gopkg.in/russross/blackfriday.v2 => github.com/russross/blackfriday/v2 v2.0.1

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.0

replace github.com/google/glog => github.com/slok/noglog v0.2.1-0.20181001030204-470afb9f333a

replace github.com/aleofreddi/csi-sanlock-lvm/proto => ../proto

replace github.com/aleofreddi/csi-sanlock-lvm/logger => ../logger
