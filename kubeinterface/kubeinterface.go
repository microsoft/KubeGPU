package kubeinterface

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	"github.com/Microsoft/KubeGPU/types"
	kubev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
)

// func escapeStr(origStr string) string {
// 	str1 := strings.Replace(origStr, ".", ".0", -1) // escape the escape character
// 	str2 := strings.Replace(str1, "/", ".1", -1) // esacpe all "/" to ".1", continue escaping others if needed (can use ".2", ".3", etc.)
// 	return str2
// }

// func unescapeStr(escapeStr string) string {
// 	str1 := strings.Replace(escapeStr, ".0", ".", -1) // unescape the escape character
// 	str2 := strings.Replace(str1, ".1", "/", -1) // unescape all ".1" to "/", continue unescaping others if needed
// 	return str2
// }

// NodeInfoToAnnotation is used by device advertiser to convert node info to annotation
func NodeInfoToAnnotation(meta *metav1.ObjectMeta, nodeInfo *types.NodeInfo) error {
	info, err := json.Marshal(nodeInfo)
	if err != nil {
		return err
	}
	meta.Annotations["node.alpha/DeviceInformation"] = string(info)
	glog.V(4).Infof("NodeInfo: %+v converted to Annotations: %v", nodeInfo, meta.Annotations)
	return nil
}

// AnnotationToNodeInfo is used by scheduler to convert annotation to node info
func AnnotationToNodeInfo(meta *metav1.ObjectMeta) (*types.NodeInfo, error) {
	nodeInfo := types.NewNodeInfo()
	if (meta.Annotations != nil) {
		nodeInfoStr, ok := meta.Annotations["node.alpha/DeviceInformation"]
		if ok {
			err := json.Unmarshal([]byte(nodeInfoStr), nodeInfo)
			if err != nil {
				return nil, err
			}
		}
	}
	glog.V(4).Infof("Annotations: %v converted to NodeInfo: %+v", meta.Annotations, nodeInfo)
	return nodeInfo, nil
}

func addContainersToPodInfo(containers map[string]types.ContainerInfo, conts []kubev1.Container, invalidateExistingAnnotations bool) {
	for _, c := range conts {
		cont, ok := containers[c.Name]
		if !ok {
			cont = *types.NewContainerInfo()
		}
		contF := types.FillContainerInfo(&cont)
		for kr, vr := range c.Resources.Requests {
			contF.KubeRequests[types.ResourceName(kr)] = vr.Value()
		}
		containers[c.Name] = *contF
	}
	if invalidateExistingAnnotations {
		for contName, cont := range containers {
			cont.AllocateFrom = make(types.ResourceLocation) // overwrite allocatefrom
			cont.DevRequests = make(types.ResourceList)
			for reqKey, reqVal := range cont.Requests {
				cont.DevRequests[reqKey] = reqVal
			}
			containers[contName] = cont
		}
	}
}

// KubePodInfoToPodInfo converts kubernetes pod info to group scheduler's simpler struct
func KubePodInfoToPodInfo(kubePodInfo *kubev1.Pod, invalidateExistingAnnotations bool) (*types.PodInfo, error) {
	podInfo := types.NewPodInfo()
	// unmarshal from annotations
	if (kubePodInfo.ObjectMeta.Annotations != nil) {
		podInfoStr, ok := kubePodInfo.ObjectMeta.Annotations["pod.alpha/DeviceInformation"]
		if ok {
			err := json.Unmarshal([]byte(podInfoStr), podInfo)
			if err != nil {
				return nil, err
			}
		}
	}
	podInfo.Name = kubePodInfo.ObjectMeta.Name
	// add default kuberenetes requests to "KubeRequests" field & clear if desired
	addContainersToPodInfo(podInfo.InitContainers, kubePodInfo.Spec.InitContainers, invalidateExistingAnnotations)
	addContainersToPodInfo(podInfo.RunningContainers, kubePodInfo.Spec.Containers, invalidateExistingAnnotations)
	if invalidateExistingAnnotations {
		podInfo.NodeName = ""
	}
	glog.V(4).Infof("Kubernetes pod: %+v converted to device scheduler podinfo: %v", kubePodInfo, podInfo)
	return podInfo, nil
}

func PodInfoToAnnotation(meta *metav1.ObjectMeta, podInfo *types.PodInfo) error {
	// marshal the whole structure
	info, err := json.Marshal(podInfo)
	if err != nil {
		return err
	}	
	meta.Annotations["pod.alpha/DeviceInformation"] = string(info)
	return nil
}

// ==================================

func GetPatchBytes(c v1core.CoreV1Interface, resourceName string, old, new, dataStruct interface{}) ([]byte, error) {
	oldData, err := json.Marshal(old)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old resource %#v with name %s: %v", old, resourceName, err)
	}

	newData, err := json.Marshal(new)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new resource %#v with name %s: %v", new, resourceName, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, dataStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to create patch for resource %s: %v", resourceName, err)
	}
	return patchBytes, nil
}

func PatchNodeMetadata(c v1core.CoreV1Interface, nodeName string, oldNode *kubev1.Node, newNode *kubev1.Node) (*kubev1.Node, error) {
	patchBytes, err := GetPatchBytes(c, nodeName, oldNode, newNode, kubev1.Node{})
	if err != nil {
		return nil, err
	}

	updatedNode, err := c.Nodes().Patch(nodeName, kubetypes.StrategicMergePatchType, patchBytes)
	if err != nil {
		errStr := fmt.Sprintf("failed to patch metadata %q for node %q: %v", patchBytes, nodeName, err)
		glog.Errorf(errStr)
		return nil, fmt.Errorf(errStr)
	}
	return updatedNode, nil
}

func PatchPodMetadata(c v1core.CoreV1Interface, podName string, oldPod *kubev1.Pod, newPod *kubev1.Pod) (*kubev1.Pod, error) {
	patchBytes, err := GetPatchBytes(c, podName, oldPod, newPod, kubev1.Pod{})
	if err != nil {
		return nil, err
	}

	updatedPod, err := c.Pods(oldPod.ObjectMeta.Namespace).Patch(podName, kubetypes.StrategicMergePatchType, patchBytes, "metadata")
	if err != nil {
		errStr := fmt.Sprintf("failed topatch metadata %q for pod %q: %v", patchBytes, podName, err)
		glog.Errorf(errStr)
		return nil, fmt.Errorf(errStr)
	}
	return updatedPod, nil
}

func UpdatePodMetadata(c v1core.CoreV1Interface, newPod *kubev1.Pod) (*kubev1.Pod, error) {
	// full update does not work since nodename change in pod spec is rejected
	// return c.Pods(newPod.ObjectMeta.Namespace).Update(newPod)
	// get current pod
	oldPod, err := c.Pods(newPod.ObjectMeta.Namespace).Get(newPod.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// validate
	if (newPod.ObjectMeta.Name != oldPod.ObjectMeta.Name) || (newPod.ObjectMeta.Namespace != oldPod.ObjectMeta.Namespace) {
		return nil, fmt.Errorf("new pod does not match old, new: %v, old: %v", newPod.ObjectMeta, oldPod.ObjectMeta)
	}
	// create newPod which is clone of oldPod
	modifiedPod := oldPod.DeepCopy()
	modifiedPod.ObjectMeta.Annotations = newPod.ObjectMeta.Annotations // take new annotations
	// now perform update - guarantee that only annotations will be modified
	//return PatchPodMetadata(c, modifiedPod.ObjectMeta.Name, oldPod, modifiedPod)
	return c.Pods(modifiedPod.ObjectMeta.Namespace).Update(modifiedPod)
}

