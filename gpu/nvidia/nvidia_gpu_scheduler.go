package nvidia

import (
	"github.com/KubeGPU/gpu"
	"github.com/KubeGPU/types"
)

type NvidiaGPUScheduler struct {
}

func (ns *NvidiaGPUScheduler) TranslateGPUResources(alloc types.ResourceList, req types.ResourceList) types.ResourceList {
	numGPUs := req[types.ResourceNvidiaGPU]
	return gpu.TranslateGPUResources(numGPUs, alloc, req)
}

func (ns *NvidiaGPUScheduler) GetName() string {
	return "nvidiagpu"
}
