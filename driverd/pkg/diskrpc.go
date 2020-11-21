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
	"io"
	"os"
	"reflect"
	"time"

	pb "github.com/aleofreddi/csi-sanlock-lvm/proto"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"github.com/ncw/directio"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

const nodePayloadSize = 4 << 10

const (
	diskRpcLockTag = tagPrefix + "type=rpc-lock"
	diskRpcDataTag = tagPrefix + "type=rpc-data"
)

type diskRpc struct {
	baseServer
	target   map[uint8]interface{}
	done     chan struct{}
	dataPath string
	dataVgLv string
	lockVgLv string
}

// FIXME REMOVE
//func NewDiskRpcTEST(lvmctrld pb.LvmCtrldClient) (*diskRpc, error) {
//	n, err := NewDiskRpc(lvmctrld)
//	if err != nil {
//		return nil, err
//	}
//	n.nodeID = 2000
//	return n, err
//}

func NewDiskRpc(lvmctrld pb.LvmCtrldClient) (*diskRpc, error) {
	bs, err := newBaseServer(lvmctrld)
	if err != nil {
		return nil, err
	}

	// Find rpc lock and data logical volumes
	ctx := context.Background()
	lvs, err := lvmctrld.Lvs(ctx, &pb.LvsRequest{
		Select: fmt.Sprintf("lv_tags=%s || lv_tags=%s", diskRpcDataTag, diskRpcLockTag),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list logical volumes: %v", err)
	}
	var dataVgLv, lockVgLv string
	for _, lv := range lvs.Lvs {
		for _, tag := range lv.LvTags {
			if tag == diskRpcLockTag {
				if lockVgLv != "" {
					return nil, fmt.Errorf("found multiple logical volumes tagged with the %s tag", diskRpcLockTag)
				}
				lockVgLv = fmt.Sprintf("%s/%s", lv.VgName, lv.LvName)
			} else if tag == diskRpcDataTag {
				if dataVgLv != "" {
					return nil, fmt.Errorf("found multiple logical volumes tagged with the %s tag", diskRpcDataTag)
				}
				if lv.LvSize < 2000*nodePayloadSize {
					return nil, fmt.Errorf("data logical volume %s/%s is too small: actual size %d bytes, expected at least %d bytes", lv.VgName, lv.LvName, lv.LvSize, 2000*nodePayloadSize)
				}
				dataVgLv = fmt.Sprintf("%s/%s", lv.VgName, lv.LvName)
			}
		}
	}
	if lockVgLv == "" || dataVgLv == "" {
		return nil, fmt.Errorf("missing lock or data logical volumes (expected tags %q/%q)", diskRpcLockTag, diskRpcDataTag)
	}

	// Activate dataVgLv as shared
	_, err = lvmctrld.LvChange(ctx, &pb.LvChangeRequest{
		Target:   []string{dataVgLv},
		Activate: pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_SHARED,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to activate rpc data logical volume: %v", err)
	}

	// Ensure we can open data logical volume
	file, err := os.OpenFile("/dev/"+dataVgLv, os.O_RDWR|os.O_SYNC, 0750)
	if err != nil {
		return nil, fmt.Errorf("failed to open data device: %v", err)
	}
	defer file.Close()

	return &diskRpc{
		baseServer: *bs,
		dataPath: "/dev/" + dataVgLv,
		target:   make(map[uint8]interface{}),
		lockVgLv: lockVgLv,
		dataVgLv: dataVgLv,
	}, nil
}

func (s *diskRpc) open() (*os.File, error) {
	file, err := directio.OpenFile(s.dataPath, os.O_RDWR, 0750)
	if err != nil {
		return nil, fmt.Errorf("failed to open data device: %v", err)
	}
	return file, err
}

func (s *diskRpc) lock(ctx context.Context) error {
	// Acquire write lock
	var err error
	for t := 0; t < 30 /* FIXME: hardcoded timeout! */ ; t++ {
		_, err = s.lvmctrld.LvChange(ctx, &pb.LvChangeRequest{
			Target:   []string{s.lockVgLv},
			Activate: pb.LvActivationMode_LV_ACTIVATION_MODE_ACTIVE_EXCLUSIVE,
		})
		if status.Code(err) == codes.PermissionDenied {
			// Locked by another host: retry
			select { case <-time.After(1 * time.Second): }
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to lock rpc volume: %v", err)
		}
		return nil
	}
	return fmt.Errorf("timeout while trying to acquire lock on rpc volume: %v", err)
}

func (s *diskRpc) unlock(ctx context.Context) {
	_, err := s.lvmctrld.LvChange(ctx, &pb.LvChangeRequest{
		Target:   []string{s.lockVgLv},
		Activate: pb.LvActivationMode_LV_ACTIVATION_MODE_DEACTIVATE,
	})
	if err != nil {
		panic(fmt.Errorf("failed to deactivate lock rpc volume: %v", err))
	}
}

func readNodeHeader(file *os.File, nodeID uint16) (pb.DiskRpcType, error) {
	block := directio.AlignedBlock(directio.BlockSize)
	//var hdr [4]byte
	_, err := file.Seek(int64(nodeID)*nodePayloadSize, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to seek data volume: %v", err)
	}
	if _, err := io.ReadFull(file, block[:]); err != nil {
		return 0, fmt.Errorf("failed to read from data volume: %v", err)
	}
	return pb.DiskRpcType(block[0]), nil
}

func writeNodeHeader(file *os.File, nodeID uint16, st pb.DiskRpcType) error {
	block := directio.AlignedBlock(directio.BlockSize)
	// Read header block
	_, err := file.Seek(int64(nodeID)*nodePayloadSize, 0)
	if _, err := io.ReadFull(file, block[:]); err != nil {
		return fmt.Errorf("failed to read from data volume: %v", err)
	}
	// Update header
	block[0] = byte(st)
	_, err = file.Seek(int64(nodeID)*nodePayloadSize, 0)
	if err != nil {
		return fmt.Errorf("failed to seek data volume: %v", err)
	}
	if err := writeFull(file, block[:]); err != nil {
		return fmt.Errorf("failed to write to data volume: %v", err)
	}
	return nil
}

func readNodeMessage(file *os.File, nodeID uint16) (*pb.DiskRpcMessage, error) {
	record := directio.AlignedBlock(nodePayloadSize)
	if _, err := file.Seek(int64(nodeID)*nodePayloadSize, 0); err != nil {
		return nil, fmt.Errorf("failed to seek data volume: %v", err)
	}
	if _, err := io.ReadFull(file, record[:]); err != nil {
		return nil, fmt.Errorf("failed to read record from disk: %v", err)
	}
	l := uint16(record[4])<<8 | uint16(record[5])
	if l > nodePayloadSize-6 {
		return nil, fmt.Errorf("read invalid node message size: %d", l)
	}
	msg := &pb.DiskRpcMessage{}
	if err := proto.Unmarshal(record[6:6+l], msg); err != nil {
		return nil, fmt.Errorf("failed to deserialize message: %v", err)
	}
	return msg, nil
}

func writeNodeMessage(file *os.File, nodeID uint16, msg *pb.DiskRpcMessage) error {
	// Encode message
	msgBuf, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to encode response: %v", err)
	}
	if len(msgBuf) > nodePayloadSize-6 {
		return fmt.Errorf("response exceeds payload limit: %d of %d bytes", len(msgBuf), nodePayloadSize-6)
	}

	// Read previous record
	record := directio.AlignedBlock(nodePayloadSize)
	if _, err := file.Seek(int64(nodeID)*nodePayloadSize, 0); err != nil {
		return fmt.Errorf("failed to seek data volume: %v", err)
	}
	if _, err := io.ReadFull(file, record[:]); err != nil {
		return fmt.Errorf("failed to read record from disk: %v", err)
	}
	record[4] = byte(len(msgBuf) >> 8)
	record[5] = byte(len(msgBuf))
	copy(record[6:], msgBuf)
	//// Write response
	//if _, err := file.Seek(int64(nodeID)*nodePayloadSize+4, 0); err != nil {
	//	return fmt.Errorf("failed to seek data volume: %v", err)
	//}
	//if err = writeFull(file, []byte{byte(len(msgBuf) >> 8), byte(len(msgBuf))}); err != nil {
	//	return fmt.Errorf("failed to write length to data volume: %v", err)
	//}
	if _, err := file.Seek(int64(nodeID)*nodePayloadSize, 0); err != nil {
		return fmt.Errorf("failed to seek data volume: %v", err)
	}
	if err = writeFull(file, record); err != nil {
		return fmt.Errorf("failed to write message to data volume: %v", err)
	}
	return nil
}

func (s *diskRpc) Start() error {
	s.done = make(chan struct{})
	go s.watch()
	return nil
}

func (s *diskRpc) Stop() {
	close(s.done)
}

func (s *diskRpc) SetTarget(targetID uint8, target interface{}) {
	// FIXME: we should lock s.target!
	if target == nil {
		delete(s.target, targetID)
	} else {
		s.target[targetID] = target
	}
}

func (s *diskRpc) Invoke(ctx context.Context, nodeID uint16, targetID uint8, method string, req proto.Message, res proto.Message) error {
	locked := false
	defer func() {
		if locked {
			s.unlock(ctx)
		}
	}()
	defer func() {
		klog.V(5).Infof("Remote RPC to node %d: %s(%+v) returned", nodeID, method, req)
	}()

	// Ensure we can open data logical volume
	klog.V(5).Infof("Issueing remote RPC to node %d: %s(%+v)", nodeID, method, req)
	file, err := s.open()
	if err != nil {
		return err
	}
	defer file.Close()

	// Try to acquire and lock when status is EMPTY
	for i := 0; ; i++ {
		if i > 30 {
			/* FIXME: free timeout */
			return fmt.Errorf("deadline exceeded while trying to send request")
		}

		// Check if the slot is available
		st, err := readNodeHeader(file, nodeID)
		if err != nil {
			return err
		}
		if st != pb.DiskRpcType_DISK_RPC_PAYLOAD_EMPTY {
			select { case <-time.After(time.Second):}
			continue
		}

		// If available, lock and confirm availability
		s.lock(ctx)
		locked = true
		st, err = readNodeHeader(file, nodeID)
		if err != nil {
			return err
		}
		if st != pb.DiskRpcType_DISK_RPC_PAYLOAD_EMPTY {
			s.unlock(ctx)
			locked = false
			select { case <-time.After(time.Second):}
			continue
		}
		klog.V(6).Infof("Invoke: acquired sending slot with status %s", st)
		break
	}

	// Encode request
	var reqBuf []byte
	if req != nil {
		if reqBuf, err = proto.Marshal(req); err != nil {
			return fmt.Errorf("failed to serialize request to message: %v", err)
		}
	}
	reqMsg := &pb.DiskRpcMessage{
		Time:    ptypes.TimestampNow(),
		Method:  method,
		Request: reqBuf,
	}
	if err = s.send(ctx, file, nodeID, pb.DiskRpcType_DISK_RPC_PAYLOAD_REQUEST, reqMsg); err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}

	// Wait for response
	s.unlock(ctx)
	locked = false

	// Try to acquire and lock when status is RESPONSE
	for i := 0; ; i++ {
		if i > 30 {
			/* FIXME: free timeout */
			return fmt.Errorf("deadline exceeded while waiting for response")
		}

		// Check if the slot is available
		st, err := readNodeHeader(file, nodeID)
		if err != nil {
			return err
		}
		if st != pb.DiskRpcType_DISK_RPC_PAYLOAD_RESPONSE {
			select { case <-time.After(time.Second):}
			continue
		}

		// If available, lock and confirm availability
		s.lock(ctx)
		locked = true
		st, err = readNodeHeader(file, nodeID)
		if err != nil {
			return err
		}
		if st != pb.DiskRpcType_DISK_RPC_PAYLOAD_RESPONSE {
			s.unlock(ctx)
			locked = false
			select { case <-time.After(time.Second):}
			continue
		}
		klog.V(6).Infof("Invoke: acquired receiving slot with status %s", st)
		break
	}

	// Receive response
	resMsg, err := readNodeMessage(file, nodeID)
	if err != nil {
		return fmt.Errorf("failed to receive response: %v", err)
	}
	// Decode response
	if resMsg.GetResponse() != nil {
		if err = proto.Unmarshal(resMsg.Response, res); err != nil {
			return fmt.Errorf("failed to deserialize response from message: %v", err)
		}
	}
	// Decode error if any
	var resErr error
	if resMsg.GetErrorMsg() != "" || resMsg.GetErrorCode() != 0 {
		resErr = status.Error(codes.Code(resMsg.GetErrorCode()), resMsg.GetErrorMsg())
	}
	// Release slot
	if err = writeNodeHeader(file, nodeID, pb.DiskRpcType_DISK_RPC_PAYLOAD_EMPTY); err != nil {
		return fmt.Errorf("failed to free node slot after receiving a message: %v", err)
	}
	return resErr
}

func (s *diskRpc) cleanup(file *os.File) error {
	ctx := context.Background()
	// Acquire lock
	if err := s.lock(ctx); err != nil {
		return err
	}
	defer s.unlock(ctx)
	return writeNodeHeader(file, s.nodeID, pb.DiskRpcType_DISK_RPC_PAYLOAD_EMPTY)
}

func (s *diskRpc) recv(ctx context.Context, file *os.File, nodeID uint16) (pb.DiskRpcType, *pb.DiskRpcMessage, error) {
	// Read the status
	st, err := readNodeHeader(file, nodeID)
	if err != nil {
		return 0, nil, err
	}
	if st != pb.DiskRpcType_DISK_RPC_PAYLOAD_REQUEST && st != pb.DiskRpcType_DISK_RPC_PAYLOAD_RESPONSE {
		return 0, nil, nil
	}
	// Acquire the lock
	if err = s.lock(ctx); err != nil {
		return 0, nil, err
	}
	defer s.unlock(ctx)
	// Refresh status
	st, err = readNodeHeader(file, nodeID)
	if st != pb.DiskRpcType_DISK_RPC_PAYLOAD_REQUEST && st != pb.DiskRpcType_DISK_RPC_PAYLOAD_RESPONSE {
		return 0, nil, nil
	}
	// Read the node message and return it
	msg, err := readNodeMessage(file, nodeID)
	if err != nil {
		return 0, nil, err
	}
	return st, msg, nil

}

func (s *diskRpc) send(ctx context.Context, file *os.File, nodeID uint16, st pb.DiskRpcType, msg *pb.DiskRpcMessage) error {
	// Acquire the lock
	if err := s.lock(ctx); err != nil {
		return err
	}
	defer s.unlock(ctx)

	if err := writeNodeMessage(file, nodeID, msg); err != nil {
		return err
	}
	if err := writeNodeHeader(file, nodeID, st); err != nil {
		return err
	}
	return nil
}

func (s *diskRpc) handle(ctx context.Context, req *pb.DiskRpcMessage) (*pb.DiskRpcMessage, error) {
	// Resolve method and instance request
	target := s.target[0]
	m := reflect.ValueOf(target).MethodByName(req.GetMethod())
	if !m.IsValid() {
		return nil, fmt.Errorf("unknown method %q", req.GetMethod())
	}
	reqType := m.Type().In(1).Elem()
	reqObj := reflect.New(reqType).Interface().(proto.Message)

	// Unmarshal request
	err := proto.Unmarshal(req.GetRequest(), reqObj)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize request from message: %v", err)
	}

	// Invoke method
	klog.V(6).Infof("DiskRpc: calling %s(%+v)...", req.Method, protosanitizer.StripSecrets(reqObj))
	resVal := m.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(reqObj)})

	// Encode response
	klog.V(6).Infof("Serializing response (%+v, %+v)\n", resVal[0], resVal[1]) // FIXME stripsecrets - REMOVE
	var resObj proto.Message
	var resBuf []byte
	if !resVal[0].IsNil() {
		resObj := resVal[0].Interface().(proto.Message)
		if resBuf, err = proto.Marshal(resObj); err != nil {
			return nil, fmt.Errorf("failed to serialize response to message: %v", err)
		}
	}
	klog.V(6).Infof("Response buffer %d -> %t (%+v)\n", len(resBuf), resVal[0].IsNil(), resObj) //resVal[1]) // FIXME stripsecrets - REMOVE
	var resErrMsg string
	var resErrCode uint32
	if !resVal[1].IsNil() {
		err := resVal[1].Interface().(error)
		resErrMsg = err.Error()
		resErrCode = uint32(status.Code(err))
		klog.Errorf("DiskRpc: call %s(%+v) returned error %v", req.Method, protosanitizer.StripSecrets(req), err)
	} else {
		klog.V(5).Infof("DiskRpc: call %s(%+v) returned %+v", req.Method, protosanitizer.StripSecrets(reqObj), protosanitizer.StripSecrets(resObj))
	}
	res := &pb.DiskRpcMessage{
		Time:      ptypes.TimestampNow(),
		Response:  resBuf,
		ErrorCode: resErrCode,
		ErrorMsg:  resErrMsg,
	}
	return res, nil
}

func (s *diskRpc) watch() error {
	// Ensure we can open data logical volume
	file, err := s.open()
	if err != nil {
		return err
	}
	defer file.Close()

	i := 0
	ctx := context.Background()
	for last := pb.DiskRpcType_DISK_RPC_PAYLOAD_EMPTY; ; {
		select {
		case <-s.done:
			break
		default:
		}

		// Read header
		st, err := readNodeHeader(file, s.nodeID)
		if err != nil {
			return err
		}

		if last != st {
			klog.V(5).Infof("Watching for incoming messages, header is %s", st)
		} else {
			klog.V(6).Infof("Watching for incoming messages, header is %s", st)
		}
		if st == pb.DiskRpcType_DISK_RPC_PAYLOAD_REQUEST {
			_, req, err := s.recv(ctx, file, s.nodeID)
			if err != nil {
				return err
			}
			res, err := s.handle(ctx, req)
			if err != nil {
				klog.Errorf("Failed to handle RPC call: %v", err)
				if err := s.cleanup(file); err != nil {
					return err
				}
				continue
			}
			if err := s.send(ctx, file, s.nodeID, pb.DiskRpcType_DISK_RPC_PAYLOAD_RESPONSE, res); err != nil {
				klog.Errorf("Failed to send RPC response: %v", err)
				if err := s.cleanup(file); err != nil {
					return err
				}
				continue
			}
		} else if st == last && st != pb.DiskRpcType_DISK_RPC_PAYLOAD_EMPTY {
			i++

			if i > 45 /* FIXME! Arbitrary and hardcoded timeout! */ {
				// Stale status!
				klog.Warningf("Stale status detected (%s), discarding stale content", st)
				if err := s.cleanup(file); err != nil {
					return err
				}
			}
		} else {
			i = 0
			last = st
		}
		select { case <-time.After(1 * time.Second):}
		continue
	}
}

func writeFull(writer io.Writer, b []byte) error {
	for i := 0; i < len(b); {
		n, err := writer.Write(b[i:])
		if err != nil {
			return err
		}
		i += n
	}
	return nil
}
