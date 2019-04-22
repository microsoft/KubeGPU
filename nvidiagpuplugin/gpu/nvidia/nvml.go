package nvidia

import (
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

func getDevices() (*gpusInfo, error) {
	numGpus, err := nvml.GetDeviceCount()
	if err != nil {
		return nil, err
	}
	var devices []nvml.Device
	for i := uint(0); i < numGpus; i++ {
		dev, err := nvml.NewDevice(i)
		if err != nil {
			return nil, err
		}
		devices = append(devices, *dev)
	}
	for i := uint(0); i < numGpus; i++ {
		for j := uint(0); j < numGpus; j++ {
			topo := nvml.P2PLink{BusID: devices[j].PCI.BusID, Link: nvml.P2PLinkUnknown}
			if i != j {
				topoType, err := nvml.GetP2PLink(&devices[i], &devices[j])
				if err != nil {
					return nil, err
				}
				topo.Link = topoType
			}
			devices[i].Topology = append(devices[i].Topology, topo)
		}
	}

	gpus := &gpusInfo{}
	gpus.Version.Driver, err = nvml.GetDriverVersion()
	if err != nil {
		return nil, err
	}
	gpus.Version.CUDA = "" // unsupported for now
	for i := uint(0); i < numGpus; i++ {
		gpu := gpuInfo{}
		gpu.ID = devices[i].UUID
		gpu.Model = *devices[i].Model
		gpu.Path = devices[i].Path
		gpu.Memory = memoryInfo{Global: int64(*devices[i].Memory)}
		gpu.PCI = pciInfo{BusID: devices[i].PCI.BusID, Bandwidth: int64(*devices[i].PCI.Bandwidth)}
		var topos []topologyInfo
		for j := uint(0); j < numGpus; j++ {
			if i != j {
				topos = append(topos, topologyInfo{BusID: devices[i].Topology[j].BusID, Link: int32(devices[i].Topology[j].Link)})
			}
		}
		gpu.Topology = topos
		gpus.Gpus = append(gpus.Gpus, gpu)
	}

	return gpus, nil
}
