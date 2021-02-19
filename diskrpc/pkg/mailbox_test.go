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
	"io/ioutil"
	"os"
	"reflect"
	"sync"
	"testing"
)

func TestMailBoxBasic(t *testing.T) {
	file := makeTmpFile(t, "TestMailBox", 1024*1024)
	defer os.Remove(file.Name())
	var mutex sync.Mutex
	mbox1, err := NewMailBox(1, &mutex, file.Name())
	if err != nil {
		t.Fatalf("failed to initialize mailbox: %v", err)
	}
	mbox2, err := NewMailBox(2, &mutex, file.Name())
	if err != nil {
		t.Fatalf("failed to initialize mailbox: %v", err)
	}

	err = mbox1.Send(&Message{Recipient: 2, Payload: []byte{1, 2, 3, 4, 5}})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
	err = mbox1.Send(&Message{Recipient: 2, Payload: []byte{5, 4, 3, 2, 1}})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
	msgs, err := mbox1.Recv()
	if err != nil {
		t.Fatalf("failed to receive messages: %v", err)
	}
	if len(msgs) > 0 {
		t.Fatalf("got = %v, want 0", len(msgs))
	}
	msgs, err = mbox2.Recv()
	if err != nil {
		t.Fatalf("failed to receive messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got = %v, want 0", len(msgs))
	}
}

var testPayload = []byte{1, 2, 3, 4, 5}

func TestMailBoxSelf(t *testing.T) {
	file := makeTmpFile(t, "TestMailBox", 1024*1024)
	defer os.Remove(file.Name())
	var mutex sync.Mutex
	mbox1, err := NewMailBox(1, &mutex, file.Name())
	if err != nil {
		t.Fatalf("failed to initialize mailbox: %v", err)
	}

	err = mbox1.Send(&Message{Recipient: 1, Payload: []byte{1, 2, 3, 4, 5}})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
	msgs, err := mbox1.Recv()
	if err != nil {
		t.Fatalf("failed to receive messages: %v", err)
	}

	want := []*Message{
		&Message{Sender: 1, Recipient: 1, Payload: testPayload},
	}
	if !reflect.DeepEqual(msgs, want) {
		t.Fatalf("got = %+v, want %+v", msgs, want)
	}
}

func TestMailBoxFill(t *testing.T) {
	file := makeTmpFile(t, "TestMailBox", 1024*1024)
	defer os.Remove(file.Name())
	var mutex sync.Mutex
	mbox1, err := NewMailBox(1, &mutex, file.Name())
	if err != nil {
		t.Fatalf("failed to initialize mailbox: %v", err)
	}
	mbox2, err := NewMailBox(2, &mutex, file.Name())
	if err != nil {
		t.Fatalf("failed to initialize mailbox: %v", err)
	}

	// Fill the mailbox.
	cnt := 0
	for {
		err = mbox1.Send(&Message{Recipient: 2, Payload: []byte{1, 2, 3, 4, 5}})
		if err != nil {
			break
		}
		cnt++
	}
	// Expect mbox2 to fail message Send: mailbox is full.
	err = mbox2.Send(&Message{Recipient: 2, Payload: []byte{1, 2, 3, 4, 5}})
	if err == nil {
		t.Fatalf("got = %v, want an error", err)
	}
	// Consume messages.
	msgs, err := mbox2.Recv()
	if err != nil {
		t.Fatalf("failed to receive messages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatalf("got = %v, want >0", len(msgs))
	}
	// Expect mbox1 Send to succed.
	err = mbox1.Send(&Message{Recipient: 2, Payload: []byte{1, 2, 3, 4, 5}})
	if err != nil {
		t.Fatalf("got = %v, want no error", err)
	}
}

func makeTmpFile(t *testing.T, id string, size int) *os.File {
	file, err := ioutil.TempFile("", "csi-sanlock-lvm."+id+".*.data")
	if err != nil {
		t.Fatalf("failed to get temp file: %v", err)
	}
	if err = file.Truncate(int64(size)); err != nil {
		t.Fatalf("failed to initialize temporary file: %v", err)
	}
	return file
}
