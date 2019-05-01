package main

import (
	"github.com/Microsoft/KubeDevice-API/pkg/device"
	"github.com/Microsoft/KubeGPU/nvidiagpuplugin/gpu/nvidia"
)

func CreateDevicePlugin() (device.Device, error) {
	return nvidia.NewNvidiaGPUManager()
}
