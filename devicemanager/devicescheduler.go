package devicemanager

import (
	"reflect"

	"github.com/KubeGPU/gpu/nvidia"
	"github.com/KubeGPU/types"
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
