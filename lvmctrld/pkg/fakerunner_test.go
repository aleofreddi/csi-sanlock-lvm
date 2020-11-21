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

package lvmctrld

import (
	"fmt"
	"testing"
)

type fakeCommand struct {
	exe    string
	args   []string
	rc     int
	stdout string
	stderr string
	err    error
}

type fakeRunner struct {
	t *testing.T

	executions []fakeCommand
	current    int
}

func (c *fakeRunner) Exec(exe string, args ...string) (code int, stdout, stderr []byte, err error) {
	if c.current == len(c.executions) {
		c.t.Errorf("unexpected exec invocation (%s, %v)", exe, args)
		return 255, []byte{}, []byte{}, fmt.Errorf("unexpected invocation")
	}
	expected := c.executions[c.current]
	if expected.exe != exe || len(expected.args) != len(args) {
		c.t.Errorf("exec: got invocation (%s, %v), expected (%s, %v)", exe, args, expected.exe, expected.args)
		return 255, []byte{}, []byte{}, fmt.Errorf("unexpected invocation")
	}
	for i, v := range expected.args {
		if v != args[i] {
			c.t.Errorf("exec: got invocation (%s, %v), expected (%s, %v)", exe, args, expected.exe, expected.args)
			return 255, []byte{}, []byte{}, fmt.Errorf("unexpected invocation")
		}
	}
	c.current++
	return expected.rc, []byte(expected.stdout), []byte(expected.stderr), expected.err
}
