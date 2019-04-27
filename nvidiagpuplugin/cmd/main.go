package main

import "github.com/Microsoft/KubeGPU/nvidiagpuplugin/gpu/nvidia"

func main() {
	devices, err := nvidia.GetDevices()
	print("Err: %v Devices: %+v", err, devices)
}
