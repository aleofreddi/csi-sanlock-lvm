// Copyright 2021 Google LLC
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

package diskrpc

import (
	"testing"

	"github.com/golang/protobuf/proto"
)

func TestAllocator(t *testing.T) {
	type allocOp struct {
		alloc   *int32
		free    *Addr
		want    int32
		wantErr bool
	}
	type args struct {
		size int32
	}
	tests := []struct {
		name string
		args args
		ops  []allocOp
	}{
		{
			"Alloc all the space",
			args{8},
			[]allocOp{
				{alloc: proto.Int32(8), free: nil, want: 0, wantErr: false},
				{alloc: proto.Int32(1), free: nil, want: -1, wantErr: true},
			},
		},
		{
			"Fill the tree and free everything",
			args{8},
			[]allocOp{
				{alloc: proto.Int32(3), free: nil, want: 0, wantErr: false},
				{alloc: proto.Int32(2), free: nil, want: 4, wantErr: false},
				{alloc: proto.Int32(2), free: nil, want: 6, wantErr: false},
				{alloc: proto.Int32(1), free: nil, want: -1, wantErr: true},
				{alloc: nil, free: addrPtr(4), want: 0, wantErr: false},
				{alloc: nil, free: addrPtr(0), want: 0, wantErr: false},
				{alloc: nil, free: addrPtr(6), want: 0, wantErr: false},
			},
		},
		{
			"Double Free",
			args{8},
			[]allocOp{
				{alloc: proto.Int32(3), free: nil, want: 0, wantErr: false},
				{alloc: proto.Int32(2), free: nil, want: 4, wantErr: false},
				{alloc: nil, free: addrPtr(4), want: 0, wantErr: false},
				{alloc: nil, free: addrPtr(4), want: 0, wantErr: true},
			},
		},
		{
			"Invalid Free",
			args{8},
			[]allocOp{
				{alloc: proto.Int32(3), free: nil, want: 0, wantErr: false},
				{alloc: proto.Int32(2), free: nil, want: 4, wantErr: false},
				{alloc: nil, free: addrPtr(1), want: 0, wantErr: true},
				{alloc: nil, free: addrPtr(3), want: 0, wantErr: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloc, err := NewAllocatorBySize(tt.args.size)
			if err != nil {
				t.Fatalf("Setup() error = %v", err)
			}
			for _, op := range tt.ops {
				var res int32
				var resAddr Addr
				var err error
				if op.alloc != nil {
					resAddr, err = alloc.Alloc(*op.alloc)
				} else {
					err = alloc.Free(*op.free)
				}
				res = int32(resAddr)
				if (err != nil) != op.wantErr {
					t.Fatalf("Allocation(%+v) error = %v, wantErr %v", op, err, op.wantErr)
				}
				if res != op.want {
					t.Fatalf("Allocation(%+v) got = %v, want %d", op, res, op.want)
				}
			}
		})
	}
}

func addrPtr(addr int) *Addr {
	t := Addr(addr)
	return &t
}
