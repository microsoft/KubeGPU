package device

import (
	"reflect"

	"github.com/KubeGPU/gpu/nvidia"
	"github.com/KubeGPU/kubeinterface"
	"github.com/KubeGPU/scheduler/algorithm"
	"github.com/KubeGPU/types"
	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

var DeviceSchedulerRegistry = map[string]reflect.Type{
	(&nvidia.NvidiaGPUScheduler{}).GetName(): reflect.TypeOf(nvidia.NvidiaGPUScheduler{}),
}

type DevicesScheduler struct {
	Devices           []types.DeviceScheduler
	RunGroupScheduler []bool
}

func (ds *DevicesScheduler) CreateAndAddDeviceScheduler(device string) error {
	o := reflect.New(DeviceSchedulerRegistry[device])
	t := o.Interface().(types.DeviceScheduler)
	ds.Devices = append(ds.Devices, t)
	usingGroupScheduler := t.UsingGroupScheduler()
	if usingGroupScheduler {
		for i := range ds.RunGroupScheduler {
			ds.RunGroupScheduler[i] = false
		}
		ds.RunGroupScheduler = append(ds.RunGroupScheduler, true)
	} else {
		ds.RunGroupScheduler = append(ds.RunGroupScheduler, false)
	}
	return nil
}

func GetPodAndNode(pod *v1.Pod, nodeInfo *schedulercache) (*types.PodInfo, *types.NodeInfo, error) {
	// grab node information
	nodeEx := nodeInfo.nodeEx
	if nodeEx == nil {
		return nil, nil, fmt.Errorf("node not found")
	}
	podInfo := KubePodInfoToPodInfo(&pod.Spec)
	return podInfo, nodeEx, nil
}


// predicate
func (ds *DevicesScheduler) PodFitsGroupResources(pod *v1.Pod, meta interface{}, node *schedulercache.NodeInfo) (bool, []algorithm.PredicateFailureReason, error) {
	podInfo, nodeInfo, err := kubeinterface.GetPodAndNode(pod, node)
	if err != nil {
		return false, nil, err
	}
	totalScore := 0.0
	totalFit := true
	var totalReasons []algoruthm.PredicateFailureReason
	for index, d := range ds.Devices {
		fit, reasons, score := d.PodFitsDevice(nodeInfo, podInfo, ds.RunGroupScheduler[index])
		// early terminate? - but score will not be correct then
		totalScore += score
		totalFit &= fit
		totalReasons = append(totalReasons, reasons)
	}
	return totalFit, totalReasons, nil
}

// allocate devices & write into annotations
func (ds *DevicesScheduler) PodAllocate(pod *v1.Pod, node *schedulercache.NodeInfo) error {
	podInfo, nodeInfo, err := kubeinterface.GetPodAndNode(pod, node)
	if err != nil {
		return false, nil, err
	}
	for index, d := range ds.Devices {
		err = d.PodAllocate(nodeInfo, podInfo, ds.RunGroupScheduler[index])
		if err != nil {
			return err
		}
	}
	kubeinterface.PodInfoToAnnotation(&pod.ObjectMeta, podInfo)
	return nil
}
