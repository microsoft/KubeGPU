/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package upgrade

import (
	"io/ioutil"
	"os"
	"testing"

	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

func TestSessionIsInteractive(t *testing.T) {
	var tcases = []struct {
		name     string
		flags    *applyFlags
		expected bool
	}{
		{
			name: "Explicitly non-interactive",
			flags: &applyFlags{
				nonInteractiveMode: true,
			},
			expected: false,
		},
		{
			name: "Implicitly non-interactive since --dryRun is used",
			flags: &applyFlags{
				dryRun: true,
			},
			expected: false,
		},
		{
			name: "Implicitly non-interactive since --force is used",
			flags: &applyFlags{
				force: true,
			},
			expected: false,
		},
		{
			name:     "Interactive session",
			flags:    &applyFlags{},
			expected: true,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.flags.sessionIsInteractive() != tt.expected {
				t.Error("unexpected result")
			}
		})
	}
}

func TestGetPathManagerForUpgrade(t *testing.T) {

	haEtcd := &kubeadmapi.InitConfiguration{
		ClusterConfiguration: kubeadmapi.ClusterConfiguration{
			Etcd: kubeadmapi.Etcd{
				External: &kubeadmapi.ExternalEtcd{
					Endpoints: []string{"10.100.0.1:2379", "10.100.0.2:2379", "10.100.0.3:2379"},
				},
			},
		},
	}

	noHAEtcd := &kubeadmapi.InitConfiguration{}

	tests := []struct {
		name             string
		cfg              *kubeadmapi.InitConfiguration
		etcdUpgrade      bool
		shouldDeleteEtcd bool
	}{
		{
			name:             "ha etcd but no etcd upgrade",
			cfg:              haEtcd,
			etcdUpgrade:      false,
			shouldDeleteEtcd: true,
		},
		{
			name:             "non-ha etcd with etcd upgrade",
			cfg:              noHAEtcd,
			etcdUpgrade:      true,
			shouldDeleteEtcd: false,
		},
		{
			name:             "ha etcd and etcd upgrade",
			cfg:              haEtcd,
			etcdUpgrade:      true,
			shouldDeleteEtcd: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Use a temporary directory
			tmpdir, err := ioutil.TempDir("", "TestGetPathManagerForUpgrade")
			if err != nil {
				t.Fatalf("unexpected error making temporary directory: %v", err)
			}
			oldK8sDir := constants.KubernetesDir
			constants.KubernetesDir = tmpdir
			defer func() {
				constants.KubernetesDir = oldK8sDir
				os.RemoveAll(tmpdir)
			}()

			pathmgr, err := GetPathManagerForUpgrade(test.cfg, test.etcdUpgrade)
			if err != nil {
				t.Fatalf("unexpected error creating path manager: %v", err)
			}

			if _, err := os.Stat(pathmgr.BackupManifestDir()); os.IsNotExist(err) {
				t.Errorf("expected manifest dir %s to exist, but it did not (%v)", pathmgr.BackupManifestDir(), err)
			}

			if _, err := os.Stat(pathmgr.BackupEtcdDir()); os.IsNotExist(err) {
				t.Errorf("expected etcd dir %s to exist, but it did not (%v)", pathmgr.BackupEtcdDir(), err)
			}

			if err := pathmgr.CleanupDirs(); err != nil {
				t.Fatalf("unexpected error cleaning up directories: %v", err)
			}

			if _, err := os.Stat(pathmgr.BackupManifestDir()); os.IsNotExist(err) {
				t.Errorf("expected manifest dir %s to exist, but it did not (%v)", pathmgr.BackupManifestDir(), err)
			}

			if test.shouldDeleteEtcd {
				if _, err := os.Stat(pathmgr.BackupEtcdDir()); !os.IsNotExist(err) {
					t.Errorf("expected etcd dir %s not to exist, but it did (%v)", pathmgr.BackupEtcdDir(), err)
				}
			} else {
				if _, err := os.Stat(pathmgr.BackupEtcdDir()); os.IsNotExist(err) {
					t.Errorf("expected etcd dir %s to exist, but it did not", pathmgr.BackupEtcdDir())
				}
			}
		})
	}

}
