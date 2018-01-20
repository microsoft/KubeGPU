package predicates

import (
	"k8s.io/api/core/v1"
	"github.com/KubeGPU/scheduler/schedulercache"
	"github.com/KubeGPU/kubeinterface"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
)

func PodFitsGroupResources(pod *v1.Pod, meta interface{}, nodeInfo *schedulercache.NodeInfo) (bool, []algorithm.PredicateFailureReason, error) {
	nodeEx := nodeInfo.NodeEx
	if nodeEx == nil {
		return false, nil, fmt.Errorf("node not found")
	}
	// now extract podInfo
	podInfo = kubeinterface.
}