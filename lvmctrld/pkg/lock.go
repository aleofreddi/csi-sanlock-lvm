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
	"k8s.io/klog"
	"os/exec"
	"time"
)

func StartLock(hostId uint16, volumeGroups []string) error {
	if err := daemonize("sanlock", "daemon", "-w", "0", "-D"); err != nil {
		return err
	}
	if err := daemonize("lvmlockd", "--host-id", fmt.Sprintf("%d", hostId), "-f"); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	klog.Infof("Starting global lock for volume groups %s (can take up to 3 minutes)", volumeGroups)
	vgchange := exec.Command("vgchange", "--lockstart")
	if err := vgchange.Run(); err != nil {
		return fmt.Errorf("failed to start lock: %s", err.Error())
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
