package main

import (
	"flag"
	"fmt"

	devtypes "github.com/Microsoft/KubeDevice-API/pkg/device"
	"github.com/Microsoft/KubeDevice-API/pkg/types"
	"github.com/Microsoft/KubeGPU/nvidiagpuplugin/gpu/nvgputypes"
)

func main() {
	var usePlugin = flag.Bool("plugin", false, "Use plugin to find devices.")
	flag.Parse()

	if !*usePlugin {
		fmt.Printf("Not using plugin\n")
		devices, err := nvgputypes.GetDevices()
		fmt.Printf("Err: %v Devices: %+v\n", err, devices)
	} else {
		fmt.Printf("Using plugin\n")
		d, err := devtypes.CreateDeviceFromPlugin("/usr/local/KubeExt/devices/nvidiagpuplugin.so")
		if err != nil {
			fmt.Printf("Error creating plugin - error encountered %v", err)
		} else {
			d.New()
			d.Start()
			nodeInfo := types.NewNodeInfo()
			d.UpdateNodeInfo(nodeInfo)
			fmt.Printf("NodeInfo: %+v\n", nodeInfo)
		}
	}
}
