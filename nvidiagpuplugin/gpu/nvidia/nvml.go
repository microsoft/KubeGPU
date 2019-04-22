package nvidia

import (
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

func getDevices() (gpusInfo, error) {
	//gpus := &gpusInfo{}

	//gpus.versionInfo.Driver = nvml.GetDriverVersion()
	//gpus.versionInfo.CUDA = nvml.GetCudaDriverVersion()
	numGpus = nvml.GetDeviceCount()
	var devices []nvml.Device
	for i := 0; i < numGpus; i++ {
		dev, err := nvml.NewDevice(i)
		if err != nil {
			return devices, err
		}
		devices = append(devices, dev)
	}
	for i := 0; i < numGpus; i++ {
		for j := 0; j < numGpus; j++ {
			topo := nvml.P2PLinkUnknown
			if i != j {
				topo, err := nvml.GetP2PLink(devices[i], devices[j])
				if err != nil {
					return devices, err
				}
			}
			devices[i].Topology = append(devices[i].Topology, topo)
		}
	}

	return devices, nil
}
