package predicates

import (
	"k8s.io/api/core/v1"
	"github.com/Microsoft/KubeGPU/device"
	"github.com/Microsoft/KubeGPU/scheduler/algorithm"
	"github.com/Microsoft/KubeGPU/scheduler/schedulercache"
)

func PodFitsDevices(pod *v1.Pod, meta algorithm.PredicateMetadata, node *schedulercache.NodeInfo) (bool, []algorithm.PredicateFailureReason, error) {
	podInfo, nodeInfo, err := schedulercache.GetPodAndNode(pod, node, true)
	if err != nil {
		return false, nil, err
	}
	fits, reasons, _ := device.DeviceScheduler.PodFitsResources(podInfo, nodeInfo, false) // no need to fill allocatefrom yey
	var failureReasons []algorithm.PredicateFailureReason
	for _, reason := range reasons {
		rName, requested, used, capacity := reason.GetInfo()
		krName := string(rName)
		failureReasons = append(failureReasons, NewInsufficientResourceError(v1.ResourceName(krName), requested, used, capacity))
	}
	return fits, failureReasons, nil
}	

