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
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"sync"

	pb "github.com/aleofreddi/csi-sanlock-lvm/pkg/proto"
	proto "github.com/golang/protobuf/proto"
	"github.com/ncw/directio"
	"k8s.io/klog"
)

const (
	// Current data version.
	dataVersion byte = 1
)

// Expected static header value.
var magicHeader = []byte("csi-sanlock-lvm\n")

// An identifier to identify mailboxes. Valid ranges are 0-2000.
// 0 is treated as a special value to identify the local mailbox.
type MailBoxID uint16

const (
	// Local mailbox.
	localMailBoxID MailBoxID = 0
)

// State segment.
type stSeg []byte

// A mailbox message.
type Message struct {
	Sender    MailBoxID
	Recipient MailBoxID
	Payload   []byte
}

// A MailBox is an interface that allows unicast message communication.
type MailBox interface {
	// Retrieve the local mailbox id.
	LocalID() MailBoxID

	// Receive messages from the mail box.
	//
	// This call will block until there is something to read.
	Recv() ([]*Message, error)

	// Send a message.
	Send(msg *Message) error
}

/**
 * The file is chopped into (at least) 4k blocks, so to be aligned with
 * underlying hardware blocks.
 *
 * The file is organized as a list of the following segments:
 *
 * HEADER STATE[0] STATE[1] STATE[2] DATA
 *
 * Each section is block aligned (it is legit to have a file block size, as
 * reported in the HEADER section, be a multiple of the platform block size).
 *
 * The HEADER segment is setup on formatting, and it is immutable. It specifies
 * various information required to properly handle the file.
 *
 * 	 Byte 0-16: magic id ("csi-sanlock-lvm\n")
 *   Byte 16-17: version, uint8
 *   Byte 17-18: reserved
 *   Byte 18-22: block size, uint32
 *   Byte 22-26: file size, uint32
 *   Byte 26-30: allocator node count, uint32
 *   Byte 26-4096: reserved
 *
 * The STATE segments are held in triple copy, so to implement a simple
 * protection against torn writes. TODO(add reference to algorithm).
 *
 * Each STATE segment is organized as follows:
 *
 *   Byte 0-8004: node list head, 2001*uint32
 *   Byte 192-8192: node list head, 2000*uint32
 *   Byte 8192-N: allocation tree (depends on the file size)
 *
 * The node head structure holds a pointer to the message list on the data
 * section if any, while the allocation tree holds the status of allocated data
 * space.
 *
 * The DATA sections contain user data, and referenced from the node head and
 * allocated from the allocation tree.
 *
 * To achieve transactional semantics, we rely on the fact that each transaction
 * can only use free space from the DATA segment (so it can be thought as a
 * journaling), so that in case of ungraceful STATE flush, the previous active
 * STATE will still have a consistent view of data.
 */
type mailBox struct {
	localID  MailBoxID
	locker   sync.Locker
	dataFile string
}

func NewMailBox(localID MailBoxID, locker sync.Locker, dataFile string) (*mailBox, error) {
	if localID <= 0 || localID > 2000 {
		return nil, fmt.Errorf("invalid mailbox ID %d", localID)
	}
	mb := &mailBox{
		localID:  localID,
		locker:   locker,
		dataFile: dataFile,
	}
	// Open the backend file/device and initialize if needed.
	file, err := os.OpenFile(dataFile, os.O_RDWR|os.O_SYNC, 0750)
	if err != nil {
		return nil, fmt.Errorf("failed to open %q: %v", dataFile, err)
	}
	defer file.Close()
	bSize, sSize, aSize, err := mb.readHeader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %v", err)
	}
	klog.Infof("MailBox file block size is %d, state segment size %d, allocator size %d", bSize, sSize, aSize)
	return mb, nil
}

func (mb *mailBox) LocalID() MailBoxID {
	return mb.localID
}

func (mb *mailBox) Send(msg *Message) error {
	recipient := msg.Recipient
	if recipient == localMailBoxID {
		recipient = mb.localID
	}
	pbMsg := &pb.MailBoxMessage{
		Sender:  uint32(mb.localID),
		Length:  uint32(len(msg.Payload)),
		Payload: msg.Payload,
	}
	// Acquire lock.
	mb.locker.Lock()
	defer mb.locker.Unlock()
	// Open data file.
	file, _, err := mb.open()
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()
	// Read header and retrieve file block size.
	bSize, sSize, aSize, err := mb.readHeader(file)
	if err != nil {
		return fmt.Errorf("failed to read header: %v", err)
	}
	// Read STATE segments.
	state, err := mb.readState(file, bSize, sSize)
	if err != nil {
		return fmt.Errorf("failed to read state: %v", err)
	}
	// Deserialize allocator tree.
	alloc, err := NewAllocatorByNodeCnt(aSize / 4)
	if err != nil {
		return fmt.Errorf("failed to instance allocator: %v", err)
	}
	mb.allocatorFromState(alloc, state)
	// Point the current head from the message.
	head := state[192+4*(recipient-1) : 192+4*(recipient-1)+4]
	pbMsg.Next = binary.LittleEndian.Uint32(head)
	buf, err := proto.Marshal(pbMsg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %v", err)
	}
	pBlocks := mb.toBlocks(bSize, 4+len(buf))
	addr, err := alloc.Alloc(int32(pBlocks))
	if err != nil {
		return fmt.Errorf("failed to allocate %d bytes: %v", 4+len(buf), err)
	}
	// Seek to data position.
	if _, err := file.Seek(int64(addr)*int64(bSize), 0); err != nil {
		return fmt.Errorf("failed to seek file: %v", err)
	}
	// Prepare aligned buffer.
	pBuf := directio.AlignedBlock(pBlocks * bSize)
	binary.LittleEndian.PutUint32(pBuf, uint32(len(buf)))
	copy(pBuf[4:], buf)
	// Write the data.
	if _, err := file.Write(pBuf); err != nil {
		return fmt.Errorf("failed to write data: %v", err)
	}
	// Serialize allocator tree.
	mb.allocatorToState(alloc, state)
	// Update head pointer.
	binary.LittleEndian.PutUint32(head, uint32(addr))
	if err := mb.writeState(file, bSize, sSize, state); err != nil {
		return fmt.Errorf("failed to write state: %v", err)
	}
	return nil
}

func (mb *mailBox) Recv() ([]*Message, error) {
	// Open data file.
	file, _, err := mb.open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()
	// Read header and state segments.
	bSize, sSize, aSize, err := mb.readHeader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %v", err)
	}
	state, err := mb.readState(file, bSize, sSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %v", err)
	}
	headBuf := state[192+4*(mb.localID-1) : 192+4*(mb.localID-1)+4]
	if int32(binary.LittleEndian.Uint32(headBuf)) < 0 {
		return nil, nil
	}

	// We have something to read: acquire lock.
	mb.locker.Lock()
	defer mb.locker.Unlock()
	// Read header and state segments.
	bSize, sSize, aSize, err = mb.readHeader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %v", err)
	}
	state, err = mb.readState(file, bSize, sSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %v", err)
	}
	// Deserialize allocator tree.
	alloc, err := NewAllocatorByNodeCnt(aSize / 4)
	if err != nil {
		return nil, fmt.Errorf("failed to instance allocator: %v", err)
	}
	mb.allocatorFromState(alloc, state)
	// Point the current head from the message.
	var msgs []*Message
	headBuf = state[192+4*(mb.localID-1) : 192+4*(mb.localID-1)+4]
	for addr := int32(binary.LittleEndian.Uint32(headBuf)); addr >= 0; {
		msgBuf, err := mb.readChunk(file, uint64(int(addr)*bSize), bSize)
		if err != nil {
			return nil, fmt.Errorf("failed to read message length: %v", err)
		}
		length := int(binary.LittleEndian.Uint32(msgBuf))
		if length <= len(msgBuf)-4 {
			msgBuf = msgBuf[4 : 4+length]
		} else {
			msgBuf, err = mb.readChunk(file, uint64(int(addr)*bSize), ((length-1)/bSize+1)*bSize)
			if err != nil {
				return nil, fmt.Errorf("failed to read message length: %v", err)
			}
		}
		// Unmarshal message.
		var pbMsg pb.MailBoxMessage
		if err := proto.Unmarshal(msgBuf, &pbMsg); err != nil {
			return nil, fmt.Errorf("failed to deserialize message: %v", err)
		}
		msgs = append(msgs, &Message{
			Sender:    MailBoxID(pbMsg.Sender),
			Recipient: MailBoxID(mb.localID),
			Payload:   pbMsg.Payload,
		})
		if err := alloc.Free(Addr(addr)); err != nil {
			return nil, fmt.Errorf("failed to deallocate %d: %v", addr, err)
		}
		addr = int32(pbMsg.Next)
	}
	// Update state.
	mb.allocatorToState(alloc, state)
	nullPtr := -1
	binary.LittleEndian.PutUint32(headBuf, uint32(nullPtr))
	if err := mb.writeState(file, bSize, sSize, state); err != nil {
		return nil, fmt.Errorf("failed to write state: %v", err)
	}
	return msgs, nil
}

// Open the data device, and return its handle and size. If the file exceeds 2^32
func (mb *mailBox) open() (*os.File, uint32, error) {
	file, err := directio.OpenFile(mb.dataFile, os.O_RDWR, 0750)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open data device: %v", err)
	}
	fi, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, fmt.Errorf("failed to stat data device: %v", err)
	}
	var size uint32
	if fi.Size() > math.MaxInt32 {
		klog.Warningf("data device %v size is over 2 GiB", mb.dataFile)
		size = math.MaxInt32
	} else {
		size = uint32(fi.Size())
	}
	return file, size, err
}

// Read header data record, and return block size. Format the file if needed.
func (mb *mailBox) readHeader(file *os.File) (bSize int, sSize int, aSize int, err error) {
	bSize = directio.BlockSize
	block := directio.AlignedBlock(bSize)
	// We try to read the header twice: on empty header, the first iteration will
	// format the file.
	for i := 0; i < 2; i++ {
		_, err = file.Seek(0, 0)
		if err != nil {
			return -1, -1, -1, fmt.Errorf("failed to seek data volume: %v", err)
		}
		if _, err := io.ReadFull(file, block[:]); err != nil {
			return -1, -1, -1, fmt.Errorf("failed to read header from data volume: %v", err)
		}
		// Check magic header.
		header := block[:len(magicHeader)]
		if bytes.Compare(header, magicHeader) == 0 {
			break
		}
		// On first iteration, if the header is empty, format the file.
		if i == 0 && bytes.Compare(header, make([]byte, len(magicHeader))) == 0 {
			klog.Infof("Detected empty data file, formatting...")
			if err = mb.format(file, bSize); err != nil {
				return -1, -1, -1, fmt.Errorf("failed to format data file: %v", err)
			}
		} else {
			return -1, -1, -1, fmt.Errorf("unexpected header while reading data file")
		}
	}
	// Check file version.
	version := block[len(magicHeader)]
	if version != dataVersion {
		return -1, -1, -1, fmt.Errorf("failed to access data file: data version %d, supported version %d", version, dataVersion)
	}
	// Read reserved byte.
	if reserved := block[len(magicHeader)+1]; reserved != 0 {
		return -1, -1, -1, fmt.Errorf("failed to access data file: invalid reserved value %d", reserved)
	}
	// Retrieve and check block size.
	bSize = int(binary.LittleEndian.Uint32(block[len(magicHeader)+2 : len(magicHeader)+6]))
	if bSize == 0 || (bSize%directio.BlockSize) != 0 {
		return -1, -1, -1, fmt.Errorf("data file was formatted with block size %d, which is not a multiple of platform block size %d", bSize, directio.BlockSize)
	}
	// Retrieve allocator node count.
	aSize = int(binary.LittleEndian.Uint32(block[len(magicHeader)+10 : len(magicHeader)+14]))
	if bits.OnesCount(uint(aSize)+4) != 1 {
		return -1, -1, -1, fmt.Errorf("invalid allocator size %d", aSize)
	}
	sSize = mb.toBlocks(bSize, 192 /* reserved */ +2000*4 /* Nodes head. */ +aSize /* Alloc tree. */) * bSize
	return
}

// Read state segment and recover for partial flushes.
func (mb *mailBox) readState(file *os.File, bSize int, sSize int) (state stSeg, err error) {
	_, err = file.Seek(int64(bSize), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to seek data file: %v", err)
	}
	state3 := make([]byte, sSize*3)
	if _, err := io.ReadFull(file, state3); err != nil {
		return nil, fmt.Errorf("failed to read state segment from data file: %v", err)
	}
	if bytes.Compare(state3[0:sSize], state3[sSize:2*sSize]) == 0 {
		return state3[0:sSize], nil
	}
	return state3[2*sSize : 3*sSize], nil
}

// Write state to data file.
func (mb *mailBox) writeState(file *os.File, bSize int, sSize int, state stSeg) error {
	if len(state) != sSize {
		return fmt.Errorf("state does not match expected size")
	}
	_, err := file.Seek(int64(bSize), 0)
	if err != nil {
		return fmt.Errorf("failed to seek data file: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := file.Write(state); err != nil {
			return fmt.Errorf("failed to write state[%d] segment to data file: %v", i, err)
		}
	}
	return nil
}

// format formats the data volume with a blank rpc entry table.
func (mb *mailBox) format(file *os.File, bSize int) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %v", err)
	}
	fSize64 := info.Size()
	if fSize64 == 0 {
		// Block devices report 0, so we seek until the end to get the real size.
		fSize64, err = file.Seek(0, io.SeekEnd)
		if err != nil {
			return fmt.Errorf("failed to fetch file size: %v", err)
		}
	}
	if fSize64 > math.MaxInt32 {
		return fmt.Errorf("file size %d exceeds maximum size %d", info.Size(), math.MaxInt32)
	}
	fSize := int(fSize64)
	alloc, err := NewAllocatorBySize(int32(fSize / bSize))
	if err != nil {
		return fmt.Errorf("failed to initialize allocator: %v", err)
	}
	// Assemble and write HEADER segment.
	hBuf := directio.AlignedBlock(bSize)
	copy(hBuf, magicHeader)
	i := len(magicHeader)
	hBuf[i] = dataVersion
	i++
	hBuf[i] = 0
	i++
	binary.LittleEndian.PutUint32(hBuf[i:], uint32(bSize))
	i += 4
	binary.LittleEndian.PutUint32(hBuf[i:], uint32(fSize))
	i += 4
	binary.LittleEndian.PutUint32(hBuf[i:], uint32(len(alloc)*4))
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek file: %v", err)
	}
	if _, err := file.Write(hBuf); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}
	// Assemble and write STATE segments.
	sBlocks := mb.toBlocks(bSize,
		192+ /* reserved */
				2000*4+ /* nodes head pointer */
				len(alloc)*4 /* alloc tree */)
	klog.Infof("Each state segment uses %d block(s)", sBlocks)
	// Allocate the space used by HEADER and STATE segments.
	addr, err := alloc.Alloc(1 /* header */ + 3*int32(sBlocks))
	if err != nil {
		return fmt.Errorf("failed to allocate header: %v", err)
	}
	// We expect the first allocation to be placed at the beginning of the file,
	// to cover HEADER and STATE segments.
	if addr != 0 {
		panic(fmt.Errorf("unexpected header allocation address %d", addr))
	}
	// Prepare state segments.
	state := directio.AlignedBlock(sBlocks * bSize)
	// Initialize node heads.
	nullPtr := -1
	for i := 0; i < 2000; i++ {
		binary.LittleEndian.PutUint32(state[192+4*i:192+4*i+4], uint32(nullPtr))
	}
	mb.allocatorToState(alloc, state)
	if err := mb.writeState(file, bSize, sBlocks*bSize, state); err != nil {
		return fmt.Errorf("failed to write state: %v", err)
	}
	return nil
}

func (mb *mailBox) readChunk(file *os.File, offset uint64, length int) ([]byte, error) {
	_, err := file.Seek(int64(offset), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to seek data file: %v", err)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(file, buf); err != nil {
		return nil, fmt.Errorf("failed to read from data file: %v", err)
	}
	return buf, nil
}

func (mb *mailBox) allocatorToState(alloc Allocator, state stSeg) {
	for i, node := range alloc {
		binary.LittleEndian.PutUint32(state[8192+4*i:8192+4*i+4], uint32(node.free))
	}
}

func (mb *mailBox) allocatorFromState(alloc Allocator, state stSeg) {
	for i, node := range alloc {
		node.free = int32(binary.LittleEndian.Uint32(state[8192+4*i : 8192+4*i+4]))
	}
}

func (mb *mailBox) toBlocks(bSize, size int) int {
	return (size-1)/bSize + 1
}
