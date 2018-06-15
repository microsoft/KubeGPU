package main

import (
	"github.com/Microsoft/KubeGPU/crishim/pkg/device"
	"github.com/Microsoft/KubeGPU/nvidiagpuplugin/gpu/nvidia"
)

func CreateDevicePlugin() (error, types.Device) {
	gpuManager := &nvidia.NvidiaGPUManager{}
	err := device.CreateAddDevice(gpuManager)
	if err != nil {
		return err, nil
	}
	return nil, &gpuManager
}
