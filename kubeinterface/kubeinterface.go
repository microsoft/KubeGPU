package kubeinterface

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/KubeGPU/types"
	kubev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
)

func addResourceList64(keyPrefix string, a map[string]string, list map[types.ResourceName]int64) {
	for k, v := range list {
		a[keyPrefix+"/"+string(k)] = strconv.FormatInt(v, 10)
	}
}

func addResourceList32(keyPrefix string, a map[string]string, list map[types.ResourceName]int32) {
	for k, v := range list {
		a[keyPrefix+"/"+string(k)] = strconv.FormatInt(int64(v), 10)
	}
}

func addResourceListName(keyPrefix string, a map[string]string, list map[types.ResourceName]types.ResourceName) {
	for k, v := range list {
		a[keyPrefix+"/"+string(k)] = string(v)
	}
}

func getToResourceList64(keyPrefix string, key string, val string, list map[types.ResourceName]int64) error {
	keyPrefix = keyPrefix + "/"
	if strings.HasPrefix(key, keyPrefix) {
		v, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		list[types.ResourceName(strings.TrimPrefix(key, keyPrefix))] = v
	}
	return nil
}

func getToResourceList32(keyPrefix string, key string, val string, list map[types.ResourceName]int32) error {
	keyPrefix = keyPrefix + "/"
	if strings.HasPrefix(key, keyPrefix) {
		v, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return err
		}
		list[types.ResourceName(strings.TrimPrefix(key, keyPrefix))] = int32(v)
	}
	return nil
}

func getToResourceListName(keyPrefix string, key string, val string, list map[types.ResourceName]types.ResourceName) error {
	keyPrefix = keyPrefix + "/"
	if strings.HasPrefix(key, keyPrefix) {
		list[types.ResourceName(strings.TrimPrefix(key, keyPrefix))] = types.ResourceName(val)
	}
	return nil
}

// NodeInfoToAnnotation is used by device advertiser to convert node info to annotation
func NodeInfoToAnnotation(meta *metav1.ObjectMeta, nodeInfo *types.NodeInfo) {
	a := meta.Annotations
	a["NodeInfo/Name"] = nodeInfo.Name
	addResourceList64("NodeInfo/Capacity", a, nodeInfo.Capacity)
	addResourceList64("NodeInfo/Allocatable", a, nodeInfo.Allocatable)
	addResourceList64("NodeInfo/Used", a, nodeInfo.Used)
	addResourceList32("NodeInfo/Scorer", a, nodeInfo.Scorer)
}

// AnnotationToNodeInfo is used by scheduler to convert annotation to node info
func AnnotationToNodeInfo(meta *metav1.ObjectMeta) (*types.NodeInfo, error) {
	nodeInfo := types.NewNodeInfo()
	a := meta.Annotations
	for k, v := range a {
		if k == "NodeInfo/Name" {
			nodeInfo.Name = k
		} else {
			err := getToResourceList64("NodeInfo/Capacity", k, v, nodeInfo.Capacity)
			if err != nil {
				return nil, err
			}
			err = getToResourceList64("NodeInfo/Allocatable", k, v, nodeInfo.Allocatable)
			if err != nil {
				return nil, err
			}
			err = getToResourceList64("NodeInfo/Used", k, v, nodeInfo.Used)
			if err != nil {
				return nil, err
			}
			err = getToResourceList32("NodeInfo/Scorer", k, v, nodeInfo.Scorer)
			if err != nil {
				return nil, err
			}
		}
	}
	return nodeInfo, nil
}

func addContainersToPodInfo(podInfo *types.PodInfo, conts []kubev1.Container) {
	for _, c := range conts {
		cont := types.NewContainerInfo()
		cont.Name = c.Name
		for kr, vr := range c.Resources.Requests {
			cont.Requests[types.ResourceName(kr)] = vr.Value()
		}
		podInfo.InitContainers = append(podInfo.InitContainers, *cont)
	}
}

// KubePodInfoToPodInfo converts kubernetes pod info to group scheduler's simpler struct
func KubePodInfoToPodInfo(kubePodInfo *kubev1.PodSpec) *types.PodInfo {
	podInfo := &types.PodInfo{}
	addContainersToPodInfo(podInfo, kubePodInfo.InitContainers)
	addContainersToPodInfo(podInfo, kubePodInfo.Containers)
	return podInfo
}

// PodInfoToAnnotation is used by scheduler to write allocate from field into annotations
// only allocate from needs to be written, other info is already avialable in pod spec
func PodInfoToAnnotation(meta *metav1.ObjectMeta, podInfo *types.PodInfo) {
	for _, c := range podInfo.InitContainers {
		keyPrefix := fmt.Sprintf("PodInfo/InitContainer/%s/AllocateFrom", c.Name)
		addResourceListName(keyPrefix, meta.Annotations, c.AllocateFrom)
	}
	for _, c := range podInfo.RunningContainers {
		keyPrefix := fmt.Sprintf("PodInfo/RunningContainer/%s/AllocateFrom", c.Name)
		addResourceListName(keyPrefix, meta.Annotations, c.AllocateFrom)
	}
}

func addLocToContainerInfo(allocFrom types.ResourceLocation, conts []types.ContainerInfo) {
	contMap := make(map[string]int) // maps container name to index
	re := regexp.MustCompile(`(.*?)/` + "AllocateFrom" + `/(.*)`)
	for allocFrom, allocTo := range allocFrom {
		matches := re.FindStringSubmatch(string(allocFrom))
		if len(matches) == 3 {
			contName := matches[1]
			contRes := matches[2]
			index, ok := contMap[contName]
			if !ok {
				index = len(conts)
				contMap[contName] = index
				newContainer := types.NewContainerInfo()
				newContainer.Name = contName // not needed as we have map, but add anyways
				conts = append(conts, *newContainer)
			}
			conts[index].AllocateFrom[types.ResourceName(contRes)] = allocTo
		}
	}
}

// AnnotationToPodInfo is used by CRI to obtain AllocateFrom written by scheduler
func AnnotationToPodInfo(meta *metav1.ObjectMeta, podInfo *types.PodInfo) {
	initAllocFrom := make(types.ResourceLocation)
	runningAllocFrom := make(types.ResourceLocation)
	for k, v := range meta.Annotations {
		getToResourceListName("PodInfo/InitContainer", k, v, initAllocFrom)
		getToResourceListName("PodInfo/RunningContainer", k, v, runningAllocFrom)
	}
	addLocToContainerInfo(initAllocFrom, podInfo.InitContainers)
	addLocToContainerInfo(runningAllocFrom, podInfo.RunningContainers)
}

// From nodeutil

// PatchNodeStatus patches node status.
func PatchNodeStatus(c v1core.CoreV1Interface, nodeName kubetypes.NodeName, oldNode *kubev1.Node, newNode *kubev1.Node) (*kubev1.Node, error) {
	oldData, err := json.Marshal(oldNode)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal old node %#v for node %q: %v", oldNode, nodeName, err)
	}

	// Reset spec to make sure only patch for Status or ObjectMeta is generated.
	// Note that we don't reset ObjectMeta here, because:
	// 1. This aligns with Nodes().UpdateStatus().
	// 2. Some component does use this to update node annotations.
	newNode.Spec = oldNode.Spec
	newData, err := json.Marshal(newNode)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new node %#v for node %q: %v", newNode, nodeName, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, kubev1.Node{})
	if err != nil {
		return nil, fmt.Errorf("failed to create patch for node %q: %v", nodeName, err)
	}

	updatedNode, err := c.Nodes().Patch(string(nodeName), kubetypes.StrategicMergePatchType, patchBytes, "status")
	if err != nil {
		return nil, fmt.Errorf("failed to patch status %q for node %q: %v", patchBytes, nodeName, err)
	}
	return updatedNode, nil
}
