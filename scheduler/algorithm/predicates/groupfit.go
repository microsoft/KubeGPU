package predicates

import (
	"fmt"

	"github.com/KubeGPU/kubeinterface"
	"github.com/KubeGPU/scheduler/schedulercache"
	"github.com/KubeGPU/grpalloc"
	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
)

func PodFitsGroupResources(pod *v1.Pod, meta interface{}, nodeInfo *schedulercache.NodeInfo) (bool, []algorithm.PredicateFailureReason, error) {
	// grab node information
	nodeEx := nodeInfo.nodeEx
	if nodeEx == nil {
		return false, nil, fmt.Errorf("node not found")
	}
	// now extract podInfo
	podInfo := kubeinterface.KubePodInfoToPodInfo(&pod.Spec)
	// check for fit on node
	grpalloc.
}
