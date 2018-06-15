package types

import (
	"github.com/Microsoft/KubeGPU/types"
)

const (
	// auto topology generation "0" means default (everything in its own group)
	GPUTopologyGeneration types.ResourceName = "alpha.gpu/gpu-generate-topology"
)

type PredicateFailureReason interface {
	GetReason() string
	GetInfo() (types.ResourceName, int64, int64, int64)
}

// used by scheduler
type DeviceScheduler interface {
	// see if pod fits on node & return device score
	PodFitsDevice(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, fillAllocateFrom bool, runGrpScheduler bool) (bool, []PredicateFailureReason, float64)
	// allocate resources
	PodAllocate(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, runGrpScheduler bool) error
	// take resources from node
	TakePodResources(*types.NodeInfo, *types.PodInfo, bool) error
	// return resources to node
	ReturnPodResources(*types.NodeInfo, *types.PodInfo, bool) error
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
