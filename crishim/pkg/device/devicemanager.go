package device

import (
	"reflect"
	"plugin"

	"github.com/Microsoft/KubeGPU/types"
	"github.com/golang/glog"
)

// DeviceManager manages multiple devices
type DevicesManager struct {
	Operational []bool
	Devices     []types.Device
}

// essentially a static variable
var DeviceManager = &DevicesManager{}

// AddDevice adds a device to the manager
func (d *DevicesManager) AddDevice(device types.Device) {
	d.Devices = append(d.Devices, device)
	d.Operational = append(d.Operational, false)
}

func (d *DevicesManager) CreateAddDevice(t types.Device) error {
	err := t.New()
	if err != nil {
		return err
	}
	d.AddDevice(t)
	return nil
}

func (d *DevicesManager) CreateAndAddDeviceType(devType Type) error {
	o := reflect.New(devType)
	t := o.Interface().(types.Device)
	return d.CreateAddDevice(t)
}

// func (d *DevicesManager) CreateAndAddDevice(deviceName string) error {
// 	return d.CreateAndAddDeviceType(DeviceRegistry[device])
// }

func (d *DevicesManager) AddDevicesFromPlugins(pluginNames string[]) {
	for _, pluginName := range plugins {
		var device types.Device
		device = nil
		p, err := plugin.Open(pluginName)
		if err == nil {
			f, err := p.Lookup("CreateDevicePlugin")
			if err == nil {
				err, d := f.(func() (error, types.device))()
				if err == nil {
					device = d
					err = device.New()
					if err != nil {
						device = nil
					}
				}
			}
		}
		if device == nil {
			glog.Errorf("Unable to add plugin %s", pluginName)
		} else {
			d.AddDevice(device)
		}
	}
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
