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
	"context"
	"fmt"
	"math"
	"reflect"
	"sync"
	"time"

	pb "github.com/aleofreddi/csi-sanlock-lvm/pkg/proto"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

// Multiplexer channel for RPC handlers.
type Channel uint8

// DiskRpc implements a synchronous RPC mechanism.
type DiskRpc interface {
	// Register a handler for the given channel. Use a nil handler to unregister
	// a target.
	Register(channel Channel, handler interface{}) error

	// Handle incoming and outgoing messages.
	Handle(ctx context.Context) error

	// Execute an RPC call.
	Invoke(ctx context.Context, nodeID MailBoxID, targetID Channel, method string, req proto.Message, res proto.Message) error
}

// diskRpc implements the DiskRpc semantics on top of a MailBox.
type diskRpc struct {
	mailBox MailBox

	// Mutex protects all the fields below.
	mutex sync.Mutex
	// A map that holds a handler object by channel.
	handlers map[Channel]interface{}
	// Pending requests, indexed by id.
	pending map[uuid.UUID]chan *pb.DiskRpcMessage
	// A queue of outgoing messages.
	outgoing []*Message
}

func NewDiskRpc(mb MailBox) (DiskRpc, error) {
	dr := &diskRpc{
		mailBox:  mb,
		handlers: make(map[Channel]interface{}),
		pending:  make(map[uuid.UUID]chan *pb.DiskRpcMessage),
	}
	return dr, nil
}

func (s *diskRpc) Register(channel Channel, handler interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if handler == nil {
		delete(s.handlers, channel)
	} else {
		s.handlers[channel] = handler
	}
	return nil
}

func (s *diskRpc) Invoke(ctx context.Context, nodeID MailBoxID, targetID Channel, method string, req proto.Message, res proto.Message) error {
	// Generate request ID.
	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("failed to generate a random uuid: %v", err)
	}
	defer func() {
		klog.V(5).Infof("[req=%s] Remote RPC to node %d: %s(%+v) returned", id, nodeID, method, req)
	}()
	klog.V(5).Infof("[req=%s] Issuing remote RPC to node %d: %s(%+v)", id, nodeID, method, req)

	// Marshal request.
	var reqBuf []byte
	if req != nil {
		if reqBuf, err = proto.Marshal(req); err != nil {
			return fmt.Errorf("failed to serialize request to message: %v", err)
		}
	}
	idBuf, err := id.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to serialize request uuid: %v", err)
	}
	dMsgBuf, err := proto.Marshal(&pb.DiskRpcMessage{
		Time:    ptypes.TimestampNow(),
		Type:    pb.DiskRpcType_DISK_RPC_TYPE_REQUEST,
		Uuid:    idBuf,
		Method:  method,
		Request: reqBuf,
	})
	if err != nil {
		return fmt.Errorf("failed to serialize request: %v", err)
	}

	// Schedule request.
	resCh := make(chan *pb.DiskRpcMessage)
	err = func() error {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		// TODO: hardcoded limit!
		if len(s.outgoing) > 64 {
			return status.Errorf(codes.ResourceExhausted, "too many queued request")
		}
		s.pending[id] = resCh
		s.outgoing = append(s.outgoing, &Message{
			Sender:    0,
			Recipient: MailBoxID(nodeID),
			Payload:   dMsgBuf,
		})
		return nil
	}()
	if err != nil {
		return err
	}
	defer func() {
		s.mutex.Lock()
		delete(s.pending, id)
		s.mutex.Unlock()
	}()

	// Get wait timeout duration.
	var wTime time.Duration
	if tout, ok := ctx.Deadline(); !ok {
		wTime = time.Now().Sub(tout)
	} else {
		// If no explicit deadline is set, we use a standard timeout value.
		// TODO: hardcoded timeout!
		wTime = 30 * time.Second
	}
	// Wait for the response.
	var dMsg *pb.DiskRpcMessage
	select {
	case dMsg = <-resCh:
	case <-time.After(wTime):
		klog.Errorf("[req=%s] Deadline exceeded", id)
		return context.DeadlineExceeded
	}

	// Unmarshal the message.
	var resErr error
	if dMsg.GetErrorMsg() != "" || dMsg.GetErrorCode() != 0 {
		resErr = status.Error(codes.Code(dMsg.GetErrorCode()), dMsg.GetErrorMsg())
	}
	// Unmarshal the response.
	if dMsg.GetResponse() != nil {
		if err = proto.Unmarshal(dMsg.Response, res); err != nil {
			return fmt.Errorf("failed to deserialize response from message: %v", err)
		}
	}
	return resErr
}

func (s *diskRpc) Handle(ctx context.Context) error {
	msgs, err := s.mailBox.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive messages: %v", err)
	}
	if len(msgs) > 0 {
		klog.V(3).Infof("Handling %d incoming messages", len(msgs))
	}
	for _, msg := range msgs {
		// Unmarshal the response.
		var dMsg pb.DiskRpcMessage
		if err = proto.Unmarshal(msg.Payload, &dMsg); err != nil {
			return fmt.Errorf("failed to deserialize response from message: %v", err)
		}
		klog.V(6).Infof("\tIncoming message %+v", decode(msg))
		switch dMsg.Type {
		case pb.DiskRpcType_DISK_RPC_TYPE_RESPONSE:
			if err := s.handleResponse(msg.Sender, &dMsg); err != nil {
				klog.Errorf("Failed to process incoming response: %v", err)
			}
		case pb.DiskRpcType_DISK_RPC_TYPE_REQUEST:
			if err := s.handleRequest(ctx, msg.Sender, &dMsg); err != nil {
				klog.Errorf("Failed to process incoming request: %v", err)
			}
		}
	}
	if len(msgs) > 0 {
		klog.V(3).Infof("Handling %d outgoing messages", len(msgs))
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	msgs = s.outgoing
	s.outgoing = nil
	for _, msg := range msgs {
		klog.V(6).Infof("\tOutgoing message %+v", decode(msg))
		if err := s.mailBox.Send(msg); err != nil {
			klog.Errorf("Failed to process outgoing message: %v", err)
		}
	}
	return nil
}

func (s *diskRpc) handleResponse(sender MailBoxID, dMsg *pb.DiskRpcMessage) error {
	id, err := uuid.FromBytes(dMsg.Uuid)
	if err != nil {
		return fmt.Errorf("failed to deserialize response id: %v", err)
	}
	ch, err := func() (chan *pb.DiskRpcMessage, error) {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		ch, ok := s.pending[id]
		if !ok {
			return nil, fmt.Errorf("response %s: request not found", id)
		}
		delete(s.pending, id)
		return ch, nil
	}()
	if err != nil {
		return err
	}
	ch <- dMsg
	return nil
}

func (s *diskRpc) handleRequest(ctx context.Context, sender MailBoxID, dMsg *pb.DiskRpcMessage) error {
	id, err := uuid.FromBytes(dMsg.Uuid)
	if err != nil {
		return fmt.Errorf("failed to deserialize request id: %v", err)
	}
	ch := dMsg.Channel
	if ch < 0 || ch > math.MaxUint8 {
		return fmt.Errorf("request %s: invalid channel %d", id, ch)
	}
	// Resolve method and instance request.
	target, ok := s.handlers[Channel(ch)]
	if !ok {
		return fmt.Errorf("request %s: unknown channel %d", id, ch)
	}
	m := reflect.ValueOf(target).MethodByName(dMsg.GetMethod())
	if !m.IsValid() {
		return fmt.Errorf("request %s: unknown method %q", id, dMsg.GetMethod())
	}
	reqType := m.Type().In(1).Elem()
	reqObj := reflect.New(reqType).Interface().(proto.Message)
	// Unmarshal request.
	if err := proto.Unmarshal(dMsg.GetRequest(), reqObj); err != nil {
		return fmt.Errorf("failed to deserialize request from message: %v", err)
	}
	// Invoke method.
	klog.V(6).Infof("DiskRpc: calling %s(%+v)...", dMsg.Method, protosanitizer.StripSecrets(reqObj))
	resVal := m.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(reqObj)})
	// Encode response.
	var resObj proto.Message
	var resBuf []byte
	if !resVal[0].IsNil() {
		resObj := resVal[0].Interface().(proto.Message)
		if resBuf, err = proto.Marshal(resObj); err != nil {
			return fmt.Errorf("failed to serialize response to message: %v", err)
		}
	}
	var resErrMsg string
	var resErrCode uint32
	if !resVal[1].IsNil() {
		err := resVal[1].Interface().(error)
		resErrMsg = err.Error()
		resErrCode = uint32(status.Code(err))
		klog.Errorf("DiskRpc: call %s(%+v) returned error %v", dMsg.Method, protosanitizer.StripSecrets(reqObj), err)
	} else {
		klog.V(5).Infof("DiskRpc: call %s(%+v) returned %+v", dMsg.Method, protosanitizer.StripSecrets(reqObj), protosanitizer.StripSecrets(resObj))
	}
	dMsgBuf, err := proto.Marshal(&pb.DiskRpcMessage{
		Time:      ptypes.TimestampNow(),
		Type:      pb.DiskRpcType_DISK_RPC_TYPE_RESPONSE,
		Uuid:      dMsg.Uuid,
		Channel:   dMsg.Channel,
		Response:  resBuf,
		ErrorCode: resErrCode,
		ErrorMsg:  resErrMsg,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// TODO: hardcoded limit!
	if len(s.outgoing) > 4*64 {
		return status.Errorf(codes.ResourceExhausted, "too many queued request")
	}
	s.outgoing = append(s.outgoing, &Message{
		Sender:    0,
		Recipient: sender,
		Payload:   dMsgBuf,
	})
	return nil
}

func decode(msg *Message) string {
	var dMsg pb.DiskRpcMessage
	if err := proto.Unmarshal(msg.Payload, &dMsg); err != nil {
		return "broken message"
	}
	id, _ := uuid.FromBytes(dMsg.Uuid)
	return fmt.Sprintf("%d->%d %s(%s)", msg.Sender, msg.Recipient, dMsg.Type, id)
}
