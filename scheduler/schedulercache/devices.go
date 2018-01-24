package schedulercache

import (
	"fmt"

	//"github.com/golang/glog"
	extypes "github.com/KubeGPU/types"
	"github.com/KubeGPU/device"
	"github.com/KubeGPU/kubeinterface"

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
		if podInfo.NodeName != node.Node.metav1.ObjectMeta.Name {
			return nil, nil, fmt.Errorf("Node name is not correct - pod expects %v, but node has %v", podInfo.NodeName, node.Node.Name)
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

//kubeinterface.PodInfoToAnnotation(&pod.ObjectMeta, podInfo)
