package main

import (
	"flag"
	"fmt"
	"os"

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
			err := d.New()
			if err != nil {
				fmt.Printf("New encounters error %v", err)
				os.Exit(1)
			}
			err = d.Start()
			if err != nil {
				fmt.Printf("Start encounters error %v", err)
				os.Exit(1)
			}
			nodeInfo := types.NewNodeInfo()
			err = d.UpdateNodeInfo(nodeInfo)
			if err != nil {
				fmt.Printf("UpdateNodeInfo encounters error %v", err)
			}
			fmt.Printf("NodeInfo: %+v\n", nodeInfo)
		}
	}
}
