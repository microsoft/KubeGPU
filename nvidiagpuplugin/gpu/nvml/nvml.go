package nvml

import (
	"encoding/json"

	"github.com/Microsoft/KubeGPU/nvidiagpuplugin/gpu/nvgputypes"
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

// GetDevices returns the device information
func GetDevices() (*nvgputypes.GpusInfo, error) {
	err := nvml.Init()
	nvmlFound := false
	shutDown := func() {
		if nvmlFound {
			nvml.Shutdown()
		}
	}
	defer shutDown()
	//fmt.Printf("Initialized NVML\n")
	if err != nil {
		return nil, err
	}
	nvmlFound = true
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

	gpus := &nvgputypes.GpusInfo{}
	gpus.Version.Driver, err = nvml.GetDriverVersion()
	if err != nil {
		return nil, err
	}
	gpus.Version.CUDA = "" // unsupported for now
	for i := uint(0); i < numGpus; i++ {
		gpu := nvgputypes.GpuInfo{}
		gpu.ID = devices[i].UUID
		gpu.Model = *devices[i].Model
		gpu.Path = devices[i].Path
		gpu.Memory = nvgputypes.MemoryInfo{
			Global: int64(*devices[i].Memory) * int64(1024) * int64(1024), //MiB
		}
		gpu.PCI = nvgputypes.PciInfo{
			BusID:     devices[i].PCI.BusID,
			Bandwidth: int64(*devices[i].PCI.Bandwidth) * int64(1000) * int64(1000), // MB
		}
		var topos []nvgputypes.TopologyInfo
		for j := uint(0); j < numGpus; j++ {
			if i != j {
				topos = append(topos, nvgputypes.TopologyInfo{
					BusID: devices[i].Topology[j].BusID,
					Link:  int32(devices[i].Topology[j].Link),
				})
			}
		}
		gpu.Topology = topos
		gpus.Gpus = append(gpus.Gpus, gpu)
	}

	return gpus, nil
}

// GetDevicesJSON returns the device information as a JSON string
func GetDevicesJSON() []byte {
	gpus, err := GetDevices()
	if err != nil {
		return nil
	} else {
		str, err := json.Marshal(gpus)
		if err != nil {
			return nil
		}
		return str
	}
}
