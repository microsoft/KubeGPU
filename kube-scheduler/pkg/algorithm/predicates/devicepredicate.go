package predicates

import (
	"github.com/Microsoft/KubeGPU/device-scheduler/device"
	"github.com/Microsoft/KubeGPU/kube-scheduler/pkg/nodeinfo"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
)

func PodFitsDevices(pod *v1.Pod, meta PredicateMetadata, node *nodeinfo.NodeInfo) (bool, []PredicateFailureReason, error) {
	podInfo, nodeInfo, err := nodeinfo.GetPodAndNode(pod, node, true)
	if err != nil {
		glog.Errorf("GetPodAndNode encounters error %v", err)
		return false, nil, err
	}
	glog.V(4).Infof("Attempting to schedule devices for pod %+v on node %+v", podInfo, nodeInfo)
	fits, reasons, _ := device.DeviceScheduler.PodFitsResources(podInfo, nodeInfo, false) // no need to fill allocatefrom yey
	var failureReasons []PredicateFailureReason
	for _, reason := range reasons {
		rName, requested, used, capacity := reason.GetInfo()
		krName := string(rName)
		failureReasons = append(failureReasons, NewInsufficientResourceError(v1.ResourceName(krName), requested, used, capacity))
	}
	return fits, failureReasons, nil
}
