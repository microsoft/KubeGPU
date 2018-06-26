package gpuplugintypes

import "github.com/Microsoft/KubeGPU/types"

const (
	// NVIDIA GPU, in devices. Alpha, might change: although fractional and allowing values >1, only one whole device per node is assigned.
	ResourceGPU types.ResourceName = "alpha.gpu/numgpu"
)
