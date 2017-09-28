package devicemanager

import "github.com/MSRCCS/grpalloc/types"

// DeviceManager manages multiple devices
type DevicesManager struct {
	Operational []bool
	Devices     []types.DeviceManager
}

// AddDevice adds a device to the manager
func (d *DevicesManager) AddDevice(device types.DeviceManager) {
	d.Devices = append(d.Devices, device)
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

// Capacity returns aggregate capacity
func (d *DevicesManager) Capacity() types.ResourceList {
	list := make(types.ResourceList)
	for i, device := range d.Devices {
		if d.Operational[i] {
			capD := device.Capacity()
			for k, v := range capD {
				list[k] = v
			}
		}
	}
	return list
}

// AllocateDevices allocates devices using device manager interface
func (d *DevicesManager) AllocateDevices(pod *types.PodInfo, cont *types.ContainerInfo) ([]types.Volume, []string, error) {
	volumes := []types.Volume{}
	devices := []string{}
	var errRet error
	errRet = nil
	for i, device := range d.Devices {
		if d.Operational[i] {
			volumeD, deviceD, err := device.AllocateDevices(pod, cont)
			if err == nil {
				volumes = append(volumes, volumeD...)
				devices = append(devices, deviceD...)
			} else {
				errRet = err
			}
		}
	}
	return volumes, devices, errRet
}
