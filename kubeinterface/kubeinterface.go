package kubeinterface

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/KubeGPU/types"
	"github.com/KubeGPU/utils"
	kubev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Add values to annotation map
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

// Get values from annotation map
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

func getToStringMap(keyPrefix string, key string, val string, list map[string]string) error {
	keyPrefix = keyPrefix + "/"
	if strings.HasPrefix(key, keyPrefix) {
		list[strings.TrimPrefix(key, keyPrefix)] = val
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
	for k, v := range meta.Annotations {
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

func clearPodInfoAnnotations(meta *metav1.ObjectMeta) {
	var newAnnotations map[string]string
	re = regexp.MustCompile(`PodInfo/.*?/.*?/(AllocateFrom|DevRequests|ValidForNode)`)
	for k, v := range meta.Annotations {
		matches := re.FindStringSubmatch(k)
		if len(matches) == 0 {
			newAnnotations[k] = v
		}
	}
	meta.Annotations = newAnnotations
}

func addContainersToPodInfo(podInfo *types.PodInfo, conts []kubev1.Container) {
	for _, c := range conts {
		cont := types.NewContainerInfo()
		cont.Name = c.Name
		for kr, vr := range c.Resources.Requests {
			cont.KubeRequests[types.ResourceName(kr)] = vr.Value()
		}
		podInfo.InitContainers = append(podInfo.InitContainers, *cont)
	}
}

// KubePodInfoToPodInfo converts kubernetes pod info to group scheduler's simpler struct
func KubePodInfoToPodInfo(kubePodInfo *kubev1.Pod, invalidateExistingAnnotations bool) (*types.PodInfo, error) {
	podInfo := &types.PodInfo{}
	// add default kuberenetes requests
	addContainersToPodInfo(podInfo, kubePodInfo.Spec.InitContainers)
	addContainersToPodInfo(podInfo, kubePodInfo.Spec.Containers)
	// generate new requests from annotations
	if invalidateExistingAnnotations {
		clearPodInfoAnnotations(&kubePodInfo.ObjectMeta)
	}
	err := annotationToPodInfo(&kubePodInfo.ObjectMeta, podInfo)
	if err != nil {
		return nil, err
	}
	if invalidateExistingAnnotations {
		// now copy original requests to device requests
		for name, req := podInfo.Requests {
			podInfo.DevRequests[name] = req
		}
	}
	return podInfo, nil
}

// PodInfoToAnnotation is used by scheduler to write allocate from field into annotations
// only allocate from needs to be written, other info is already avialable in pod spec
func PodInfoToAnnotation(meta *metav1.ObjectMeta, podInfo *types.PodInfo, nodeName string) {
	for _, c := range podInfo.InitContainers {
		keyPrefix := fmt.Sprintf("PodInfo/InitContainer/%s/AllocateFrom", c.Name)
		addResourceListName(keyPrefix, meta.Annotations, c.AllocateFrom)
		keyPrefix = fmt.Sprintf("PodInfo/InitContainer/%s/Requests", c.Name)
		addResourceList64(keyPrefix, meta.Annotations, c.Requests)
		keyPrefix = fmt.Sprintf("PodInfo/InitContainer/%s/DevRequests", c.Name)
		addResourceList64(keyPrefix, meta.Annotations, c.DevRequests)
		keyPrefix = fmt.Sprintf("PodInfo/InitContainer/%s/Scorer", c.Name)
		addResourceList32(keyPrefix, meta.Annotations, c.Scorer)
		key := fmt.Sprintf("PodInfo/InitContainer/%s/ValidForNode/Name", c.Name)
		meta.Annotations[key] = nodeName
	}
	for _, c := range podInfo.RunningContainers {
		keyPrefix := fmt.Sprintf("PodInfo/RunningContainer/%s/AllocateFrom", c.Name)
		addResourceListName(keyPrefix, meta.Annotations, c.AllocateFrom)
		keyPrefix = fmt.Sprintf("PodInfo/RunningContainer/%s/Requests", c.Name)
		addResourceList64(keyPrefix, meta.Annotations, c.Requests)
		keyPrefix = fmt.Sprintf("PodInfo/RunningContainer/%s/DevRequests", c.Name)
		addResourceList64(keyPrefix, meta.Annotations, c.DevRequests)
		keyPrefix = fmt.Sprintf("PodInfo/RunningContainer/%s/Scorer", c.Name)
		addResourceList32(keyPrefix, meta.Annotations, c.Scorer)
		key := fmt.Sprintf("PodInfo/RunningContainer/%s/ValidForNode/Name", c.Name)
		meta.Annotations[key] = nodeName
	}
}

func getFromContainerInfo(info map[string]string, searchFor string) map[string](map[string]string) {
	infoMap := make(map[string](map[string]string))
	re := regexp.MustCompile(`(.*?)/` + searchFor + `/(.*)`)
	for key, val := range info {
		matches := re.FindStringSubmatch(string(key))
		if len(matches) == 3 {
			utils.AssignMap(infoMap, []string{matches[1], matches[2]}, val)
		}
	}
	return infoMap
}

func getContainer(containerName string, contMap map[string]int, conts []types.ContainerInfo) *types.ContainerInfo {
	index, ok := contMap[containerName]
	if !ok {
		index = len(conts)
		contMap[containerName] = index
		newContainer := types.NewContainerInfo()
		newContainer.Name = containerName
		conts = append(conts, *newContainer)
	}
	return &conts[index]
}

func generateContainerInfo(info map[string]string, conts []types.ContainerInfo) error {
	contMap := make(map[string]int) // container name to index map
	reqs := getFromContainerInfo(info, "Requests")
	var err error
	for cName, cont := range reqs {
		container := getContainer(cName, contMap, conts)
		for key, val := range cont {
			container.Requests[types.ResourceName(key)], err = strconv.ParseInt(val, 10, 64)
			if err != nil {
				return err
			}
		}
	}
	reqs = getFromContainerInfo(info, "Scorer")
	for cName, cont := range reqs {
		container := getContainer(cName, contMap, conts)
		for key, val := range cont {
			int64Val, err := strconv.ParseInt(val, 10, 32)
			if err != nil {
				return err
			}
			container.Scorer[types.ResourceName(key)] = int32(int64Val)
		}
	}
	// following will only exist if they have not been invalidated
	reqs = getFromContainerInfo(info, "DevRequests")
	for cName, cont := range reqs {
		container := getContainer(cName, contMap, conts)
		for key, val := range cont {
			container.DevRequests[types.ResourceName(key)], err = strconv.ParseInt(val, 10, 64)
			if err != nil {
				return err
			}
		}
	}	
	reqs = getFromContainerInfo(info, "AllocateFrom")
	for cName, cont := range reqs {
		container := getContainer(cName, contMap, conts)
		for key, val := range cont {
			container.AllocateFrom[types.ResourceName(key)] = types.ResourceName(val)
		}
	}
	reqs = getFromContainerInfo(info, "ValidForNode")
	for cName, cont := range reqs {
		container := getContainer(cName, contMap, conts)
		if len(cont) > 1 {
			return fmt.Errorf("ValidForNode should only have one value, has %v", cont)
		}
		for key, val := range cont {
			container.NodeName = val
		}
	}
	return nil
}

// annotationToPodInfo is used by CRI to obtain AllocateFrom written by scheduler
func annotationToPodInfo(meta *metav1.ObjectMeta, podInfo *types.PodInfo) error {
	init := make(map[string]string)
	running := make(map[string]string)
	for k, v := range meta.Annotations {
		getToStringMap("PodInfo/InitContainer", k, v, init)
		getToStringMap("PodInfo/RunningContainer", k, v, running)
	}
	err := generateContainerInfo(init, podInfo.InitContainers)
	if (err != nil) {
		return err
	}
	err = generateContainerInfo(running, podInfo.RunningContainers)
	if (err != nil) {
		return err
	}
	return nil
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

