package device

import (
	"fmt"
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
	Devices []types.DeviceScheduler
}

func (d *DevicesScheduler) CreateAndAddDeviceScheduler(device string) error {
	o := reflect.New(DeviceSchedulerRegistry[device])
	t := o.Interface().(types.DeviceScheduler)
	d.Devices = append(d.Devices, t)
	return nil
}

// translate all device resources
func (ds *DevicesScheduler) TranslateResources(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) {
	for index := range podInfo.InitContainers {
		for _, device := range ds.Devices {
			podInfo.InitContainers[index].Requests = device.TranslateResource(nodeInfo.Allocatable, podInfo.InitContainers[index].Requests)
		}
	}
	for index := range podInfo.RunningContainers {
		for _, device := range ds.Devices {
			podInfo.RunningContainers[index].Requests = device.TranslateResource(nodeInfo.Allocatable, podInfo.RunningContainers[index].Requests)
		}
	}
}

// predicate
func (ds *DevicesScheduler) PodFitsGroupResources(pod *v1.Pod, meta interface{}, nodeInfo *schedulercache.NodeInfo) (bool, []algorithm.PredicateFailureReason, error) {
	// grab node information
	nodeEx := nodeInfo.nodeEx
	if nodeEx == nil {
		return false, nil, fmt.Errorf("node not found")
	}
	// now extract podInfo & resource translation
	podInfo := kubeinterface.KubePodInfoToPodInfo(&pod.Spec)
	ds.TranslateResources(nodeEx, podInfo)

	totalScore := 0.0
	totalFit := true
	var totalReasons []algoruthm.PredicateFailureReason
	for index, d := range ds.Devices {
		fit, reasons, score := d.PodFitsDevice(nodeEx, podInfo)
		totalScore += score
		totalFit &= fit
		totalReasons = append(totalReasons, reasons)
	}

	return totalFit, totalReasons, nil
}

// allocate devices
func (ds *DevicesScheduler) PodAllocate(pod *v1.Pod, nodeInfo *schedulercache.NodeInfo) error {

}
