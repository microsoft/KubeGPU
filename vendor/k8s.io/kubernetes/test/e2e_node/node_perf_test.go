/*
Copyright 2018 The Kubernetes Authors.

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

package e2e_node

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e_node/perf/workloads"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// makeNodePerfPod returns a pod with the information provided from the workload.
func makeNodePerfPod(w workloads.NodePerfWorkload) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-pod", w.Name()),
		},
		Spec: w.PodSpec(),
	}
}

func setKubeletConfig(f *framework.Framework, cfg *kubeletconfig.KubeletConfiguration) {
	if cfg != nil {
		framework.ExpectNoError(setKubeletConfiguration(f, cfg))
	}

	// Wait for the Kubelet to be ready.
	Eventually(func() bool {
		nodeList := framework.GetReadySchedulableNodesOrDie(f.ClientSet)
		return len(nodeList.Items) == 1
	}, time.Minute, time.Second).Should(BeTrue())
}

// Serial because the test updates kubelet configuration.
// Slow by design.
var _ = SIGDescribe("Node Performance Testing [Serial] [Slow]", func() {
	f := framework.NewDefaultFramework("node-performance-testing")
	var (
		wl     workloads.NodePerfWorkload
		oldCfg *kubeletconfig.KubeletConfiguration
		newCfg *kubeletconfig.KubeletConfiguration
		pod    *corev1.Pod
	)
	JustBeforeEach(func() {
		err := wl.PreTestExec()
		framework.ExpectNoError(err)
		oldCfg, err = getCurrentKubeletConfig()
		framework.ExpectNoError(err)
		newCfg, err = wl.KubeletConfig(oldCfg)
		framework.ExpectNoError(err)
		setKubeletConfig(f, newCfg)
	})

	cleanup := func() {
		gp := int64(0)
		delOpts := metav1.DeleteOptions{
			GracePeriodSeconds: &gp,
		}
		f.PodClient().DeleteSync(pod.Name, &delOpts, framework.DefaultPodDeletionTimeout)
		By("running the post test exec from the workload")
		err := wl.PostTestExec()
		framework.ExpectNoError(err)
		setKubeletConfig(f, oldCfg)
	}

	runWorkload := func() {
		By("running the workload and waiting for success")
		// Make the pod for the workload.
		pod = makeNodePerfPod(wl)
		// Create the pod.
		pod = f.PodClient().CreateSync(pod)
		// Wait for pod success.
		f.PodClient().WaitForSuccess(pod.Name, wl.Timeout())
		podLogs, err := framework.GetPodLogs(f.ClientSet, f.Namespace.Name, pod.Name, pod.Spec.Containers[0].Name)
		framework.ExpectNoError(err)
		perf, err := wl.ExtractPerformanceFromLogs(podLogs)
		framework.ExpectNoError(err)
		framework.Logf("Time to complete workload %s: %v", wl.Name(), perf)
	}

	Context("Run node performance testing with pre-defined workloads", func() {
		BeforeEach(func() {
			wl = workloads.NodePerfWorkloads[0]
		})
		It("NAS parallel benchmark (NPB) suite - Integer Sort (IS) workload", func() {
			defer cleanup()
			runWorkload()
		})
	})
	Context("Run node performance testing with pre-defined workloads", func() {
		BeforeEach(func() {
			wl = workloads.NodePerfWorkloads[1]
		})
		It("NAS parallel benchmark (NPB) suite - Embarrassingly Parallel (EP) workload", func() {
			defer cleanup()
			runWorkload()
		})
	})
	Context("Run node performance testing with pre-defined workloads", func() {
		BeforeEach(func() {
			wl = workloads.NodePerfWorkloads[2]
		})
		It("TensorFlow workload", func() {
			defer cleanup()
			runWorkload()
		})
	})
})
