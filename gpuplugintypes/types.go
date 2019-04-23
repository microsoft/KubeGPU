package gpuplugintypes

import "github.com/Microsoft/KubeDevice-API/pkg/types"

const (
	ResourceGPU types.ResourceName = "nvidia.com/gpu"
)

type SortedTreeNode struct {
	Val   int
	Score float64 // used for tie breaker
	Child []*SortedTreeNode
}
