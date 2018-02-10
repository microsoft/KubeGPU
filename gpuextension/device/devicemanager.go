package device

import (
	"reflect"

	"github.com/Microsoft/KubeGPU/gpuextension/gpu/nvidia"
	"github.com/Microsoft/KubeGPU/types"
	"github.com/golang/glog"
)

var DeviceRegistry = map[string]reflect.Type{
	(&nvidia.NvidiaGPUManager{}).GetName(): reflect.TypeOf(nvidia.NvidiaGPUManager{}),
}

// DeviceManager manages multiple devices
type DevicesManager struct {
	Operational []bool
	Devices     []types.Device
}

// AddDevice adds a device to the manager
func (d *DevicesManager) AddDevice(device types.Device) {
	d.Devices = append(d.Devices, device)
	d.Operational = append(d.Operational, false)
}

func (d *DevicesManager) CreateAndAddDevice(device string) error {
	o := reflect.New(DeviceRegistry[device])
	t := o.Interface().(types.Device)
	err := t.New()
	if err != nil {
		return err
	}
	d.AddDevice(t)
	return nil
}

// Start starts all devices in manager
func (d *DevicesManager) Start() {
	for i, device := range d.Devices {
		err := device.Start()
		if err == nil {
			d.Operational[i] = true
		} else {
			d.Operational[i] = false
		}
	}
}

// UpdateNodeInfo updates a node info strucutre with resources available on device
func (d *DevicesManager) UpdateNodeInfo(info *types.NodeInfo) {
	for i, device := range d.Devices {
		if d.Operational[i] {
			err := device.UpdateNodeInfo(info)
			if err != nil {
				glog.Errorf("Unable to update device %s encounter error %v", device.GetName(), err)
			}
		}
	}
}

// AllocateDevices allocates devices using device manager interface
func (d *DevicesManager) AllocateDevices(pod *types.PodInfo, cont *types.ContainerInfo) ([]types.Volume, []string, error) {
	volumes := []types.Volume{}
	devices := []string{}
	var errRet error
	errRet = nil
	for i, device := range d.Devices {
		if d.Operational[i] {
			volumeD, deviceD, err := device.Allocate(pod, cont)
			if err == nil {
				// appending nil to nil is okay
				volumes = append(volumes, volumeD...)
				devices = append(devices, deviceD...)
			} else {
				errRet = err
			}
		}
	}
	return volumes, devices, errRet
}
