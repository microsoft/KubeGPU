package main

import (
	"fmt"
	"os"

	mnvml "github.com/Microsoft/KubeGPU/nvidiagpuplugin/gpu/nvml"
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

func printInfo() {
	err := nvml.Init()
	nvmlFound := false
	shutDown := func() {
		fmt.Printf("Performing NVML Shutdown\n")
		if nvmlFound {
			nvml.Shutdown()
		}
	}
	defer shutDown()
	if err != nil {
		fmt.Printf("Error initializing NVML: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Initialized NVML\n")
	nvmlFound = true
	numGpus, err := nvml.GetDeviceCount()
	if err != nil {
		fmt.Printf("Error gettting device count %v\n", err)
		os.Exit(1)
	}
	var devices []nvml.Device
	for i := uint(0); i < numGpus; i++ {
		dev, err := nvml.NewDevice(i)
		if err != nil {
			fmt.Printf("Error getting info on device %d, err: %v\n", i, err)
		} else {
			devices = append(devices, *dev)
		}
	}
	numGpus = uint(len(devices))
	fmt.Printf("Number of valid devices: %d\n", numGpus)
	for i := uint(0); i < numGpus; i++ {
		for j := uint(0); j < numGpus; j++ {
			topo := nvml.P2PLink{BusID: devices[j].PCI.BusID, Link: nvml.P2PLinkUnknown}
			if i != j {
				topoType, err := nvml.GetP2PLink(&devices[i], &devices[j])
				if err != nil {
					fmt.Printf("Error getting topology between %d and %d\n", i, j)
				}
				topo.Link = topoType
			}
			devices[i].Topology = append(devices[i].Topology, topo)
		}
	}
	for i := uint(0); i < numGpus; i++ {
		fmt.Printf("Devce %d info:\n", i)
		fmt.Printf("%+v\n", devices[i])
	}
}

func main() {
	if len(os.Args) > 1 {
		fmt.Printf("%s", string(mnvml.GetDevicesJSON()))
	} else {
		printInfo()
	}
}
