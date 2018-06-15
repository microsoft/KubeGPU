package main

import (
	"github.com/golang/glog"
	"fmt"
	"plugin"

	"github.com/Microsoft/KubeGPU/crishim/pkg/app"
	"github.com/Microsoft/KubeGPU/crishim/pkg/device"
	"github.com/Microsoft/KubeGPU/crishim/pkg/types"
)

func main() {
	// Add devices here
	// if err := device.DeviceManager.CreateAndAddDevice("nvidiagpu"); err != nil {
	// 	app.Die(fmt.Errorf("Adding device nvidiagpu fails with error %v", err))
	// }
	devicePlugins := {"nvidiagpuplugin.so"}
	device.DeviceManager.AddDevicesFromPlugins(devicePlugins)
	// start the device manager
	device.DeviceManager.Start()
	// run the app - parses all command line arguments
	app.RunApp()
}
