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
	"fmt"
	"os/exec"
	"time"

	"k8s.io/klog"
)

func vgLock() error {
	vgchange := exec.Command("vgchange", "--lockstart", "--verbose")
	stdoutBuf, stderrBuf := new(bytes.Buffer), new(bytes.Buffer)
	vgchange.Stdout = stdoutBuf
	vgchange.Stderr = stderrBuf
	err := vgchange.Run()
	if err != nil {
	}
	return err
}

func StartLock(id uint16, volumeGroups []string) error {
	if err := daemonize("wdmd", "-D"); err != nil {
		return err
	}
	if err := daemonize("sanlock", "daemon", "-D"); err != nil {
		return err
	}
	if err := daemonize("lvmlockd", "--host-id", fmt.Sprintf("%d", id), "-f"); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	klog.Infof("Starting global lock (can take up to 3 minutes)")
	vgchange := exec.Command("vgchange", "--lockstart", "--verbose")
	if err := vgchange.Run(); err != nil {
		// On Ubuntu 20 LTS, with LVM 2.03.07(2) (2019-11-30), I've encountered a bug where
		// the first lockstart fails with error code 5, and a second one succeeds.
		//
		// So we wait 1 second and retry.
		time.Sleep(1 * time.Second)
		vgchange = exec.Command("vgchange", "--lockstart", "--verbose")
		if err = vgchange.Run(); err != nil {
			return fmt.Errorf("failed to start global lock: %v", err)
		}
	}
	klog.Info("Global lock started")
	return nil
}

func daemonize(executable string, args ...string) error {
	klog.Infof("Running %s with args %v", executable, args)
	cmd := exec.Command(executable, args...)
	err := cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		err = cmd.Wait()
		klog.Fatalf("Process %s terminated with error: %v", executable, err.Error())
	}()
	return nil
}
