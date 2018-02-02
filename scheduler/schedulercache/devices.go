package schedulercache

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/Microsoft/KubeGPU/device"
	"github.com/Microsoft/KubeGPU/kubeinterface"
	extypes "github.com/Microsoft/KubeGPU/types"

	"k8s.io/api/core/v1"
)

func GetPodAndNode(pod *v1.Pod, node *NodeInfo, invalidatePodAnnotations bool) (*extypes.PodInfo, *extypes.NodeInfo, error) {
	// grab node information
	nodeInfo := node.nodeEx
	if nodeInfo == nil {
		return nil, nil, fmt.Errorf("node not found")
	}
	podInfo, err := kubeinterface.KubePodInfoToPodInfo(pod, invalidatePodAnnotations)
	if err != nil {
		return nil, nil, err
	}
	if !invalidatePodAnnotations {
		nodeName := node.node.ObjectMeta.Name
		// empty strin for default pods (not from scheduler)
		if (podInfo.NodeName != nodeName) && (podInfo.NodeName != "") {
			errStr := fmt.Sprintf("Node name is not correct - pod expects %v, but node has %v", podInfo.NodeName, nodeName)
			glog.Errorf(errStr)
			return nil, nil, fmt.Errorf(errStr)
		}
	}
	return podInfo, nodeInfo, nil
}

func TakePodDeviceResources(pod *v1.Pod, node *NodeInfo) error {
	// convert pod annotations to resources and use them -- should not return error as pod annotations should be correct
	podInfo, nodeInfo, err := GetPodAndNode(pod, node, false)
	if err != nil {
		return err
	}
	return device.DeviceScheduler.TakePodResources(podInfo, nodeInfo)
}

func ReturnPodDeviceResources(pod *v1.Pod, node *NodeInfo) error {
	podInfo, nodeInfo, err := GetPodAndNode(pod, node, false)
	if err != nil {
		return err
	}
	return device.DeviceScheduler.ReturnPodResources(podInfo, nodeInfo)
}


