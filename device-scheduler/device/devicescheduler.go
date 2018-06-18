package device

import (
	"plugin"

	sctypes "github.com/Microsoft/KubeGPU/device-scheduler/types"
	"github.com/Microsoft/KubeGPU/types"
	"github.com/golang/glog"
)

// var DeviceSchedulerRegistry = map[string]reflect.Type{
// 	(&nvidia.NvidiaGPUScheduler{}).GetName(): reflect.TypeOf(nvidia.NvidiaGPUScheduler{}),
// }

type DevicesScheduler struct {
	Devices           []sctypes.DeviceScheduler
	RunGroupScheduler []bool
}

// essentially a static variable
var DeviceScheduler = &DevicesScheduler{}

func (ds *DevicesScheduler) AddDevice(device sctypes.DeviceScheduler) {
	ds.Devices = append(ds.Devices, device)
	usingGroupScheduler := device.UsingGroupScheduler()
	glog.V(3).Infof("Registering device scheduler %s, using group scheduler %v", device, usingGroupScheduler)
	if usingGroupScheduler {
		for i := range ds.RunGroupScheduler {
			ds.RunGroupScheduler[i] = false
		}
		ds.RunGroupScheduler = append(ds.RunGroupScheduler, true)
	} else {
		ds.RunGroupScheduler = append(ds.RunGroupScheduler, false)
	}
}

func (ds *DevicesScheduler) AddDevicesSchedulerFromPlugins(pluginNames []string) {
	for _, pluginName := range pluginNames {
		var device sctypes.DeviceScheduler
		device = nil
		p, err := plugin.Open(pluginName)
		if err == nil {
			f, err := p.Lookup("CreateDeviceSchedulerPlugin")
			if err == nil {
				err, d := f.(func() (error, sctypes.DeviceScheduler))()
				if err == nil {
					device = d
				} else {
					glog.Errorf("Schduler Plugin %s creation fails with error %v", pluginName, err)
				}
			} else {
				glog.Errorf("Scheudler Plugin %s function lookup fails with error %v", pluginName, err)
			}
		} else {
			glog.Errorf("Scheduler plugin %s open fails with error %v", pluginName, err)
		}
		if device == nil {
			glog.Errorf("Unable to add scheduler plugin %s", pluginName)
		} else {
			ds.AddDevice(device)
		}
	}
}

// AddNode adds node reources to devices scheduler
func (ds *DevicesScheduler) AddNode(nodeName string, nodeInfo *types.NodeInfo) {
	for _, d := range ds.Devices {
		d.AddNode(nodeName, nodeInfo)
	}
}

// RemoveNode removes node resources
func (ds *DevicesScheduler) RemoveNode(nodeName string) {
	for _, d := range ds.Devices {
		d.RemoveNode(nodeName)
	}
}

// func (ds *DevicesScheduler) CreateAndAddDeviceScheduler(device string) error {
// 	o := reflect.New(DeviceSchedulerRegistry[device])
// 	t := o.Interface().(types.DeviceScheduler)
// 	ds.AddDevice(t)
// 	return nil
// }

// predicate
func (ds *DevicesScheduler) PodFitsResources(podInfo *types.PodInfo, nodeInfo *types.NodeInfo, fillAllocateFrom bool) (bool, []sctypes.PredicateFailureReason, float64) {
	totalScore := 0.0
	totalFit := true
	var totalReasons []sctypes.PredicateFailureReason
	for index, d := range ds.Devices {
		fit, reasons, score := d.PodFitsDevice(nodeInfo, podInfo, fillAllocateFrom, ds.RunGroupScheduler[index])
		// early terminate? - but score will not be correct then
		totalScore += score
		totalFit = totalFit && fit
		totalReasons = append(totalReasons, reasons...)
	}
	return totalFit, totalReasons, totalScore
}

// allocate devices & write into annotations
func (ds *DevicesScheduler) PodAllocate(podInfo *types.PodInfo, nodeInfo *types.NodeInfo) error {
	for index, d := range ds.Devices {
		err := d.PodAllocate(nodeInfo, podInfo, ds.RunGroupScheduler[index])
		if err != nil {
			return err
		}
	}
	return nil
}

// take pod resources used by devices
func (ds *DevicesScheduler) TakePodResources(podInfo *types.PodInfo, nodeInfo *types.NodeInfo) error {
	for index, d := range ds.Devices {
		err := d.TakePodResources(nodeInfo, podInfo, ds.RunGroupScheduler[index])
		if err != nil {
			return err
		}
	}
	return nil
}

// return pod resources used by devices
func (ds *DevicesScheduler) ReturnPodResources(podInfo *types.PodInfo, nodeInfo *types.NodeInfo) error {
	for index, d := range ds.Devices {
		err := d.ReturnPodResources(nodeInfo, podInfo, ds.RunGroupScheduler[index])
		if err != nil {
			return err
		}
	}
	return nil
}
