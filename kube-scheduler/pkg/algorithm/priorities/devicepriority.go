package priorities

import (
	"github.com/Microsoft/KubeGPU/device-scheduler/device"
	schedulerapi "github.com/Microsoft/KubeGPU/kube-scheduler/pkg/api"
	"github.com/Microsoft/KubeGPU/kube-scheduler/pkg/schedulercache"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
)

// prioritizer
func PodDevicePriority(pod *v1.Pod, meta interface{}, node *schedulercache.NodeInfo) (schedulerapi.HostPriority, error) {
	podInfo, nodeInfo, err := schedulercache.GetPodAndNode(pod, node, true)
	if err != nil {
		glog.Errorf("GetPodAndNode encounters error %v", err)
		return schedulerapi.HostPriority{}, err
	}
	score := int(float64(schedulerapi.MaxPriority) * device.DeviceScheduler.PodPriority(podInfo, nodeInfo))
	glog.V(4).Infof("Device priority for pod %+v on node %+v is %d", podInfo, nodeInfo, score)
	return schedulerapi.HostPriority{
		Host:  node.Node().Name,
		Score: score,
	}, nil
}
