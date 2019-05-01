package main

import (
	"github.com/Microsoft/KubeDevice-API/pkg/devicescheduler"
	"github.com/Microsoft/KubeGPU/gpuschedulerplugin"
)

func CreateDeviceSchedulerPlugin() (devicescheduler.DeviceScheduler, error) {
	gpuScheduler := &gpuschedulerplugin.NvidiaGPUScheduler{}
	return gpuScheduler, nil
}
