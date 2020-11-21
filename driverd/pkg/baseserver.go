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

package driverd

import (
	"context"
	"fmt"

	pb "github.com/aleofreddi/csi-sanlock-lvm/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Common functionalities for all servers.
type baseServer struct {
	lvmctrld pb.LvmCtrldClient
	nodeID   uint16
}

func newBaseServer(lvmctrld pb.LvmCtrldClient) (*baseServer, error) {
	st, err := lvmctrld.GetStatus(context.Background(), &pb.GetStatusRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve status from lvmctrld: %v", err)
	}
	return &baseServer{
		nodeID:   uint16(st.NodeId),
		lvmctrld: lvmctrld,
	}, nil
}

// Fetch volume details from a volume reference.
func (bs *baseServer) fetch(ctx context.Context, ref *VolumeRef) (*VolumeInfo, error) {
	lvs, err := bs.lvmctrld.Lvs(ctx, &pb.LvsRequest{
		Target: []string{ref.VgLv()},
	})
	if status.Code(err) == codes.NotFound {
		return nil, status.Errorf(codes.NotFound, "volume %s not found", ref)
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch volume %s: %v", ref, err)
	}
	return NewVolumeDetailsFromLv(*lvs.Lvs[0]), nil
}