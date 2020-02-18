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
	"bytes"
	"os/exec"
)

type commander interface {
	Exec(exe string, args ...string) (code int, stdout, stderr []byte, err error)
}

type osCommander struct{}

func (osCommander) Exec(exe string, args ...string) (code int, stdout, stderr []byte, err error) {
	proc := exec.Command(exe, args...)
	stdoutBuf, stderrBuf := new(bytes.Buffer), new(bytes.Buffer)
	proc.Stdout = stdoutBuf
	proc.Stderr = stderrBuf
	err = proc.Run()
	return proc.ProcessState.ExitCode(), stdoutBuf.Bytes(), stderrBuf.Bytes(), err
}

func NewCommander() commander {
	return osCommander{}
}
