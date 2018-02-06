package kubeinterface

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/Microsoft/KubeGPU/types"
	"github.com/Microsoft/KubeGPU/utils"
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
	glog.V(4).Infof("NodeInfo: %+v converted to Annotations: %v", nodeInfo, meta.Annotations)
}

// AnnotationToNodeInfo is used by scheduler to convert annotation to node info
func AnnotationToNodeInfo(meta *metav1.ObjectMeta) (*types.NodeInfo, error) {
	nodeInfo := types.NewNodeInfo()
	for k, v := range meta.Annotations {
		if k == "NodeInfo/Name" {
			nodeInfo.Name = v
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
	glog.V(4).Infof("Annotations: %v converted to NodeInfo: %+v", meta.Annotations, nodeInfo)
	return nodeInfo, nil
}

func ClearPodInfoAnnotations(meta *metav1.ObjectMeta) {
	if meta.Annotations != nil {
		newAnnotations := make(map[string]string)
		re := regexp.MustCompile(`PodInfo/.*?/.*?/(AllocateFrom|DevRequests)`)
		for k, v := range meta.Annotations {
			if k != "PodInfo/ValidForNode" {
				matches := re.FindStringSubmatch(k)
				if len(matches) == 0 {
					newAnnotations[k] = v
				}
			}
		}
		meta.Annotations = newAnnotations
	}
}

func addContainersToPodInfo(containers []types.ContainerInfo, conts []kubev1.Container) []types.ContainerInfo {
	for _, c := range conts {
		cont := types.NewContainerInfo()
		cont.Name = c.Name
		for kr, vr := range c.Resources.Requests {
			cont.KubeRequests[types.ResourceName(kr)] = vr.Value()
		}
		containers = append(containers, *cont)
	}
	return containers
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

func getContainer(containerName string, contMap map[string]int, conts *[]types.ContainerInfo) *types.ContainerInfo {
	index, ok := contMap[containerName]
	if !ok {
		index = len(*conts)
		contMap[containerName] = index
		newContainer := types.NewContainerInfo()
		newContainer.Name = containerName
		*conts = append(*conts, *newContainer)
	}
	return &(*conts)[index]
}

func generateContainerInfo(info map[string]string, conts *[]types.ContainerInfo) error {
	contMap := make(map[string]int) // container name to index map
	// initialize
	for index, cont := range *conts {
		contMap[cont.Name] = index
	}
	reqs := getFromContainerInfo(info, "Requests")
	//fmt.Printf("Reqs:\n%v\n", reqs)
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
	return nil
}

// annotationToPodInfo is used by CRI to obtain AllocateFrom written by scheduler
func annotationToPodInfo(meta *metav1.ObjectMeta, podInfo *types.PodInfo) error {
	init := make(map[string]string)
	running := make(map[string]string)
	for k, v := range meta.Annotations {
		getToStringMap("PodInfo/InitContainer", k, v, init)
		getToStringMap("PodInfo/RunningContainer", k, v, running)
		if k == "PodInfo/ValidForNode" {
			podInfo.NodeName = v
		}
	}
	//fmt.Printf("Init:\n%v\n", init)
	//fmt.Printf("Running:\n%v\n", running)
	err := generateContainerInfo(init, &podInfo.InitContainers)
	if (err != nil) {
		return err
	}
	err = generateContainerInfo(running, &podInfo.RunningContainers)
	if (err != nil) {
		return err
	}
	return nil
}

// KubePodInfoToPodInfo converts kubernetes pod info to group scheduler's simpler struct
func KubePodInfoToPodInfo(kubePodInfo *kubev1.Pod, invalidateExistingAnnotations bool) (*types.PodInfo, error) {
	podInfo := &types.PodInfo{}
	// if desired, clear existing pod annotations for DevRequests, AllocateFrom, NodeName
	if invalidateExistingAnnotations {
		ClearPodInfoAnnotations(&kubePodInfo.ObjectMeta)
	}
	// add default kuberenetes requests
	podInfo.Name = kubePodInfo.ObjectMeta.Name
	podInfo.InitContainers = addContainersToPodInfo(podInfo.InitContainers, kubePodInfo.Spec.InitContainers)
	podInfo.RunningContainers = addContainersToPodInfo(podInfo.RunningContainers, kubePodInfo.Spec.Containers)
	// generate new requests from annotations
	err := annotationToPodInfo(&kubePodInfo.ObjectMeta, podInfo)
	if err != nil {
		return nil, err
	}
	if invalidateExistingAnnotations {
		// now copy original requests to device requests
		for index := range podInfo.InitContainers {
			for name, req := range podInfo.InitContainers[index].Requests { // from annotation
				podInfo.InitContainers[index].DevRequests[name] = req
			}
		}
		for index := range podInfo.RunningContainers {
			for name, req := range podInfo.RunningContainers[index].Requests {
				podInfo.RunningContainers[index].DevRequests[name] = req
			}
		}
	}
	glog.V(4).Infof("Kubernetes pod: %+v converted to device scheduler podinfo: %v", kubePodInfo, podInfo)
	return podInfo, nil
}

// PodInfoToAnnotation is used by scheduler to write allocate from field into annotations
// only allocate from needs to be written, other info is already avialable in pod spec
func PodInfoToAnnotation(meta *metav1.ObjectMeta, podInfo *types.PodInfo) {
	for _, c := range podInfo.InitContainers {
		keyPrefix := fmt.Sprintf("PodInfo/InitContainer/%s/AllocateFrom", c.Name)
		addResourceListName(keyPrefix, meta.Annotations, c.AllocateFrom)
		keyPrefix = fmt.Sprintf("PodInfo/InitContainer/%s/Requests", c.Name)
		addResourceList64(keyPrefix, meta.Annotations, c.Requests)
		keyPrefix = fmt.Sprintf("PodInfo/InitContainer/%s/DevRequests", c.Name)
		addResourceList64(keyPrefix, meta.Annotations, c.DevRequests)
		keyPrefix = fmt.Sprintf("PodInfo/InitContainer/%s/Scorer", c.Name)
		addResourceList32(keyPrefix, meta.Annotations, c.Scorer)
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
	}
	meta.Annotations["PodInfo/ValidForNode"] = podInfo.NodeName
	glog.V(4).Infof("PodInfo: %+v written to annotations: %v", podInfo, meta.Annotations)
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

	updatedNode, err := c.Nodes().Patch(nodeName, kubetypes.StrategicMergePatchType, patchBytes, "metadata")
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

