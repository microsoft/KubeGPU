package types

const (
	// NVIDIA GPU, in devices. Alpha, might change: although fractional and allowing values >1, only one whole device per node is assigned.
	ResourceGPU ResourceName = "alpha.gpu/numgpu"
	// auto topology generation "0" means default (everything in its own group)
	GPUTopologyGeneration ResourceName = "alpha.gpu/gpu-generate-topology"
	// Namespace prefix for group resources.
	DeviceGroupPrefix = "alpha/grpresource"
)

type ResourceName string

// ResourceLocation is a set of (resource name, resource location on node) pairs.
type ResourceLocation map[ResourceName]ResourceName

// ResourceList is a set of resources
type ResourceList map[ResourceName]int64

// ResourceScorer is a set of (resource name, scorer) pairs.
type ResourceScorer map[ResourceName]int32

type ContainerInfo struct {
	KubeRequests ResourceList     `json:"-"`                      // requests being handled by kubernetes core - only needed here for resource translation
	Requests     ResourceList     `json:"requests,omitempty"`     // requests specified in annotations in the pod spec
	DevRequests  ResourceList     `json:"devrequests,omitempty"`  // requests after translation - these are used by scheduler to schedule
	AllocateFrom ResourceLocation `json:"allocatefrom,omitempty"` // only valid for extended resources being advertised here
	Scorer       ResourceScorer   `json:"scorer,omitempty"`       // scorer function specified in pod specificiation annotations
}

func NewContainerInfo() *ContainerInfo {
	return &ContainerInfo{KubeRequests: make(ResourceList), Requests: make(ResourceList), AllocateFrom: make(ResourceLocation), Scorer: make(ResourceScorer), DevRequests: make(ResourceList)}
}

func FillContainerInfo(fill *ContainerInfo) *ContainerInfo {
	info := NewContainerInfo()
	if fill.KubeRequests != nil {
		info.KubeRequests = fill.KubeRequests
	}
	if fill.Requests != nil {
		info.Requests = fill.Requests
	}
	if fill.DevRequests != nil {
		info.DevRequests = fill.DevRequests
	}
	if fill.AllocateFrom != nil {
		info.AllocateFrom = fill.AllocateFrom
	}
	if fill.Scorer != nil {
		info.Scorer = fill.Scorer
	}
	return info
}

type PodInfo struct {
	Name              string                   `json:"podname,omitempty"`
	NodeName          string                   `json:"nodename,omitempty"` // the node for which DevRequests and AllocateFrom on ContainerInfo are valid, the node for which PodInfo has been customized
	Requests          ResourceList             `json:"requests,omitempty"` // pod level requests
	InitContainers    map[string]ContainerInfo `json:"initcontainer,omitempty"`
	RunningContainers map[string]ContainerInfo `json:"runningcontainer,omitempty"`
}

func NewPodInfo() *PodInfo {
	return &PodInfo{Requests: make(ResourceList), InitContainers: make(map[string]ContainerInfo), RunningContainers: make(map[string]ContainerInfo)}
}

func (p *PodInfo) GetContainerInPod(name string) *ContainerInfo {
	cont, ok := p.InitContainers[name]
	if ok {
		return &cont
	}
	cont, ok = p.RunningContainers[name]
	if ok {
		return &cont
	}
	return nil
}

// NodeInfo only holds resources being advertised by the device advertisers through annotations
type NodeInfo struct {
	Name        string         `json:"name,omitempty"`
	Capacity    ResourceList   `json:"capacity,omitempty"`
	Allocatable ResourceList   `json:"allocatable,omitempty"` // capacity minus reserverd
	Used        ResourceList   `json:"used,omitempty"`        // being used by pods, must be less than allocatable
	Scorer      ResourceScorer `json:"scorer,omitempty"`
}

func NewNodeInfo() *NodeInfo {
	return &NodeInfo{Capacity: make(ResourceList), Allocatable: make(ResourceList),
		Used: make(ResourceList), Scorer: make(ResourceScorer)}
}

func (ni *NodeInfo) Clone() *NodeInfo {
	newNode := NewNodeInfo()
	newNode.Name = ni.Name
	for key, val := range ni.Capacity {
		newNode.Capacity[key] = val
	}
	for key, val := range ni.Allocatable {
		newNode.Allocatable[key] = val
	}
	for key, val := range ni.Used {
		newNode.Used[key] = val
	}
	for key, val := range ni.Scorer {
		newNode.Scorer[key] = val
	}
	return newNode
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

type PredicateFailureReason interface {
	GetReason() string
	GetInfo() (ResourceName, int64, int64, int64)
}

// used by scheduler
type DeviceScheduler interface {
	// see if pod fits on node & return device score
	PodFitsDevice(nodeInfo *NodeInfo, podInfo *PodInfo, fillAllocateFrom bool, runGrpScheduler bool) (bool, []PredicateFailureReason, float64)
	// allocate resources
	PodAllocate(nodeInfo *NodeInfo, podInfo *PodInfo, runGrpScheduler bool) error
	// take resources from node
	TakePodResources(*NodeInfo, *PodInfo, bool) error
	// return resources to node
	ReturnPodResources(*NodeInfo, *PodInfo, bool) error
	// GetName returns the name of a device
	GetName() string
	// Tells whether group scheduler is being used?
	UsingGroupScheduler() bool
}

const (
	DefaultScorer = iota // 0
	LeftOverScorer
	EnumLeftOverScorer
)

type SortedTreeNode struct {
	Val   int
	Score float64 // used for tie breaker
	Child []*SortedTreeNode
}
