package predicates

import (
	"k8s.io/api/core/v1"
	"github.com/KubeGPU/device"
	"github.com/KubeGPU/scheduler/algorithm"
	"github.com/KubeGPU/scheduler/schedulercache"
)

func PodFitsDevices(pod *v1.Pod, meta algorithm.PredicateMetadata, node *schedulercache.NodeInfo) (bool, []algorithm.PredicateFailureReason, error) {
	podInfo, nodeInfo, err := schedulercache.GetPodAndNode(pod, node)
	if err != nil {
		return false, nil, err
	}
	fits, reasons, err := device.DeviceScheduler.PodFitsResources(podInfo, nodeInfo)
	var failureReasons []algorithm.PredicateFailureReason
	for _, reason := range reasons {
		rName, requested, used, capacity := reason.GetInfo()
		krName := string(rName)
		failureReasons = append(failureReasons, NewInsufficientResourceError(v1.ResourceName(krName), requested, used, capacity))
	}
	return fits, failureReasons, err
}	

