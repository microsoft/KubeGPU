package nvgputypes

import (
	"encoding/json"
	"os/exec"
)

type MemoryInfo struct {
	Global int64 `json:"Global"`
}

type PciInfo struct {
	BusID     string `json:"BusID"`
	Bandwidth int64  `json:"Bandwidth"`
}

type TopologyInfo struct {
	BusID string `json:"BusID"`
	Link  int32  `json:"Link"`
}

type GpuInfo struct {
	ID       string         `json:"UUID"`
	Model    string         `json:"Model"`
	Path     string         `json:"Path"`
	Memory   MemoryInfo     `json:"Memory"`
	PCI      PciInfo        `json:"PCI"`
	Topology []TopologyInfo `json:"Topology"`
	Found    bool           `json:"-"`
	Index    int            `json:"-"`
	InUse    bool           `json:"-"`
	TopoDone bool           `json:"-"`
	Name     string         `json:"-"`
}

type VersionInfo struct {
	Driver string `json:"Driver"`
	CUDA   string `json:"CUDA"`
}
type GpusInfo struct {
	Version VersionInfo `json:"Version"`
	Gpus    []GpuInfo   `json:"Devices"`
}

func GetDevices() (*GpusInfo, error) {
	output, err := exec.Command("/usr/local/bin/nvmlinfo", "json").Output()
	if err != nil {
		return nil, err
	}
	gpus := &GpusInfo{}
	err = json.Unmarshal(output, &gpus)
	if err != nil {
		return nil, err
	}
	return gpus, nil
}
