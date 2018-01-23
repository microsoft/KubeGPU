package schedulercache

import (
	"fmt"

	//"github.com/golang/glog"
	extypes "github.com/KubeGPU/types"
	"github.com/KubeGPU/device"
	"github.com/KubeGPU/kubeinterface"

	"k8s.io/api/core/v1"
)

func GetPodAndNode(pod *v1.Pod, node *NodeInfo) (*extypes.PodInfo, *extypes.NodeInfo, error) {
	// grab node information
	nodeInfo := node.nodeEx
	if nodeInfo == nil {
		return nil, nil, fmt.Errorf("node not found")
	}
	podInfo, err := kubeinterface.KubePodInfoToPodInfo(pod)
	if err != nil {
		return nil, nil, err
	}
	return podInfo, nodeInfo, nil
}

func TakePodDeviceResources(pod *v1.Pod, node *NodeInfo) error {
	// convert pod annotations to resources and use them -- should not return error as pod annotations should be correct
	podInfo, nodeInfo, err := GetPodAndNode(pod, node)
	if err != nil {
		return err
	}
	return device.DeviceScheduler.TakePodResources(podInfo, nodeInfo)
}

func ReturnPodDeviceResources(pod *v1.Pod, node *NodeInfo) error {
	podInfo, nodeInfo, err := GetPodAndNode(pod, node)
	if err != nil {
		return err
	}
	return device.DeviceScheduler.ReturnPodResources(podInfo, nodeInfo)	
}

//kubeinterface.PodInfoToAnnotation(&pod.ObjectMeta, podInfo)
