package types

const (
	// NVIDIA GPU, in devices. Alpha, might change: although fractional and allowing values >1, only one whole device per node is assigned.
	ResourceNvidiaGPU ResourceName = "alpha.kubernetes.io/nvidia-gpu"
	// Namespace prefix for group resources (alpha).
	ResourceGroupPrefix = "alpha.kubernetes.io/group-resource"
)

type ResourceName string

// ResourceLocation is a set of (resource name, resource location on node) pairs.
type ResourceLocation map[ResourceName]ResourceName

// ResourceList is a set of resources
type ResourceList map[ResourceName]int64

// ResourceScorer is a set of (resource name, scorer) pairs.
type ResourceScorer map[ResourceName]int32

type ContainerInfo struct {
	Name         string
	Requests     ResourceList
	AllocateFrom ResourceLocation
	Scorer       ResourceScorer
}

func NewContainerInfo() *ContainerInfo {
	return &ContainerInfo{Requests: make(ResourceList), AllocateFrom: make(ResourceLocation), Scorer: make(ResourceScorer)}
}

type PodInfo struct {
	Name string
	// requests
	InitContainers    []ContainerInfo
	RunningContainers []ContainerInfo
}

func (p *PodInfo) GetContainerInPod(name string) *ContainerInfo {
	for _, c := range p.InitContainers {
		if c.Name == name {
			return &c
		}
	}
	for _, c := range p.RunningContainers {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

type NodeInfo struct {
	Name        string
	Capacity    ResourceList
	Allocatable ResourceList // capacity minus reserverd
	Used        ResourceList // being used by pods, must be less than allocatable
	Scorer      ResourceScorer
}

func NewNodeInfo() *NodeInfo {
	return &NodeInfo{Capacity: make(ResourceList), Allocatable: make(ResourceList),
		Used: make(ResourceList), Scorer: make(ResourceScorer)}
}

func NewNodeInfoWithName(name string) *NodeInfo {
	node := &NodeInfo{Capacity: make(ResourceList), Allocatable: make(ResourceList),
		Used: make(ResourceList), Scorer: make(ResourceScorer)}
	node.Name = name
	return node
}

type Volume struct {
	Name   string
	Driver string
}

// Device is a device to use
type Device interface {
	// New creates the device and initializes it
	New() error
	// Start logically initializes the device
	Start() error
	// UpdateNodeInfo - updates a node info structure by writing capacity, allocatable, used, scorer
	UpdateNodeInfo(*NodeInfo) error
	// Allocate attempst to allocate the devices
	// Returns list of (VolumeName, VolumeDriver), and list of Devices to use
	// Returns an error on failure.
	Allocate(*PodInfo, *ContainerInfo) ([]Volume, []string, error)
	// GetName returns the name of a device
	GetName() string
}

const (
	DefaultScorer = iota // 0
	LeftOverScorer
	EnumLeftOverScorer
)
