package gpuplugintypes

import "github.com/Microsoft/KubeDevice-API/pkg/types"

const (
	// NVIDIA GPU, in devices. Alpha, might change: although fractional and allowing values >1, only one whole device per node is assigned.
	ResourceGPU types.ResourceName = "gpu/numgpu"
)

type SortedTreeNode struct {
	Val   int
	Score float64 // used for tie breaker
	Child []*SortedTreeNode
}
