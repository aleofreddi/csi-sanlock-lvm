// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkg

import (
	"context"
	"sync/atomic"

	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc"
	"k8s.io/klog"
)

var logUid uint32 = 0

func GrpcLogger(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	id := atomic.AddUint32(&logUid, 1)
	klog.V(6).Infof("gRPC[%d]: calling %s(%+v)...", id, info.FullMethod, protosanitizer.StripSecrets(req))
	resp, err := handler(ctx, req)
	if err != nil {
		klog.Errorf("gRPC[%d]: call %s(%+v) returned error %v", id, info.FullMethod, protosanitizer.StripSecrets(req), err)
	} else {
		klog.V(5).Infof("gRPC[%d]: call %s(%+v) returned %+v", id, info.FullMethod, protosanitizer.StripSecrets(req), protosanitizer.StripSecrets(resp))
	}
	return resp, err
}