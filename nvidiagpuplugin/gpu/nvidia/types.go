package nvidia

type memoryInfo struct {
	Global int64 `json:"Global"`
}

type pciInfo struct {
	BusID     string `json:"BusID"`
	Bandwidth int64  `json:"Bandwidth"`
}

type topologyInfo struct {
	BusID string `json:"BusID"`
	Link  int32  `json:"Link"`
}

type gpuInfo struct {
	ID       string         `json:"UUID"`
	Model    string         `json:"Model"`
	Path     string         `json:"Path"`
	Memory   memoryInfo     `json:"Memory"`
	PCI      pciInfo        `json:"PCI"`
	Topology []topologyInfo `json:"Topology"`
	Found    bool           `json:"-"`
	Index    int            `json:"-"`
	InUse    bool           `json:"-"`
	TopoDone bool           `json:"-"`
	Name     string         `json:"-"`
}

type versionInfo struct {
	Driver string `json:"Driver"`
	CUDA   string `json:"CUDA"`
}
type gpusInfo struct {
	Version versionInfo `json:"Version"`
	Gpus    []gpuInfo   `json:"Devices"`
}
