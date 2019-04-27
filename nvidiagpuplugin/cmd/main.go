package main

import (
	"fmt"

	"github.com/Microsoft/KubeGPU/nvidiagpuplugin/gpu/nvidia"
)

func main() {
	devices, err := nvidia.GetDevices()
	fmt.Printf("Err: %v Devices: %+v\n", err, devices)
}
