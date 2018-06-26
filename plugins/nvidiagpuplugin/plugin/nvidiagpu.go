package main

import (
	"github.com/Microsoft/KubeGPU/crishim/pkg/types"
	"github.com/Microsoft/KubeGPU/plugins/nvidiagpuplugin/gpu/nvidia"
)

func CreateDevicePlugin() (error, types.Device) {
	gpuManager := &nvidia.NvidiaGPUManager{}
	return nil, gpuManager
}
