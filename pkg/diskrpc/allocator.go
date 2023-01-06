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
	"fmt"
	"math"
	"math/bits"
	"strings"
)

// Allocator segment node.
type node struct {
	free int32
}

// Allocator is defined as an array of nodes and implements a buddy allocator.
type Allocator []*node

// An allocated address.
type Addr int32

func NewAllocatorBySize(size int32) (Allocator, error) {
	if size < 0 || bits.OnesCount32(uint32(size)) != 1 {
		return nil, fmt.Errorf("size must be a power of 2")
	}
	depth := 0
	for i := size; i > 0; i = i >> 1 {
		depth++
	}
	nodes := make([]*node, (1<<depth)-1)
	initialize(nodes, 0, size)
	return nodes, nil
}

func NewAllocatorByNodeCnt(nodeCnt int) (Allocator, error) {
	if nodeCnt < 0 || nodeCnt == math.MaxInt32-1 ||
		bits.OnesCount32(uint32(nodeCnt+1)) != 1 {
		return nil, fmt.Errorf("size must be a power of 2")
	}
	nodes := make([]*node, nodeCnt)
	initialize(nodes, 0, 1<<((nodeCnt+1)/2))
	return nodes, nil
}

func (nodes *Allocator) Alloc(size int32) (Addr, error) {
	return alloc(*nodes, 0, size)
}

func (nodes *Allocator) Free(addr Addr) error {
	return free(*nodes, 0, int32(len(*nodes)+1)/2, addr)
}

// Dump the tree to a string builder. Useful for debugging.
func Dump(nodes []*node) string {
	var sb strings.Builder
	dump(nodes, 0, 0, &sb)
	return sb.String()
}

func max(a int32, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func left(pos int) int {
	return 2*pos + 1
}

func right(pos int) int {
	return 2*pos + 2
}

func level(pos int) int {
	return int(math.Log2(float64(pos + 1)))
}

func address(nodes []*node, pos int) (Addr, int32) {
	l := level(pos)
	size := (len(nodes) + 1) / 2 / (1 << l)
	off := (pos - (1<<l - 1)) * size
	return Addr(int32(off)), int32(size)
}

func initialize(nodes []*node, pos int, free int32) {
	if pos >= len(nodes) {
		return
	}
	nodes[pos] = &node{free: free}
	initialize(nodes, left(pos), free/2)
	initialize(nodes, right(pos), free/2)
}

func alloc(nodes []*node, pos int, size int32) (Addr, error) {
	l := len(nodes)
	n := nodes[pos]
	if size <= 0 || n.free < size {
		return -1, fmt.Errorf("not enough space")
	}
	rightPos := right(pos)
	leftPos := left(pos)
	if n.free >= size && (leftPos >= l || nodes[leftPos].free < size) && (rightPos >= l || nodes[rightPos].free < size) {
		// Allocate the current node.
		n.free = -n.free
		v, _ := address(nodes, pos)
		return v, nil
	}
	var v Addr
	if nodes[leftPos].free >= size {
		v, _ = alloc(nodes, leftPos, size)
	} else {
		v, _ = alloc(nodes, rightPos, size)
	}
	n.free = max(max(nodes[leftPos].free, nodes[rightPos].free), 0)
	return v, nil
}

func free(nodes []*node, pos int, size int32, addr Addr) error {
	if pos >= len(nodes) {
		return fmt.Errorf("invalid address")
	}
	n := nodes[pos]
	if n.free < 0 {
		if off, _ := address(nodes, pos); off == addr {
			n.free = -n.free
			return nil
		}
		return fmt.Errorf("invalid address")
	}
	rPos := right(pos)
	lPos := left(pos)
	if rPos > len(nodes) {
		return fmt.Errorf("invalid address")
	}
	var res error
	if off, _ := address(nodes, pos); int32(addr-off) < size/2 {
		res = free(nodes, lPos, size>>1, addr)
	} else {
		res = free(nodes, rPos, size>>1, addr)
	}
	if res == nil {
		rFree := nodes[rPos].free
		lFree := nodes[lPos].free
		if lFree+rFree == size {
			n.free = size
		} else {
			n.free = max(max(lFree, rFree), 0)
		}
	}
	return res
}

func dump(nodes []*node, pos int, lvl int, sb *strings.Builder) {
	for i := 0; i < lvl; i++ {
		sb.WriteRune(' ')
	}
	addr, size := address(nodes, pos)
	sb.WriteString(fmt.Sprintf("%d-%d: #%d %+v", addr, size, pos, nodes[pos]))
	sb.WriteRune('\n')
	lp := left(pos)
	if lp >= len(nodes) {
		return
	}
	dump(nodes, lp, lvl+1, sb)
	dump(nodes, right(pos), lvl+1, sb)
}
