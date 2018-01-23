package nvidia

import (
	"github.com/KubeGPU/gpu"
	"github.com/KubeGPU/types"
	"github.com/KubeGPU/grpalloc"
)

type NvidiaGPUScheduler struct {
}

func TranslateGPUContainerResources(alloc types.ResourceList, cont types.ContainerInfo) types.ResourceList {
	numGPUs := cont.KubeRequests[types.ResourceNvidiaGPU]
	return gpu.TranslateGPUResources(numGPUs, alloc, cont.Requests)
}

func TranslateGPUResorces(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) {
	for index := range podInfo.InitContainers {
		podInfo.InitContainers[index].Requests = TranslateGPUContainerResources(nodeInfo.Allocatable, podInfo.InitContainers[index])
	}
	for index := range podInfo.RunningContainers {
		podInfo.RunningContainers[index].Requests = TranslateGPUContainerResources(nodeInfo.Allocatable, podInfo.RunningContainers[index])
	}
}

func (ns *NvidiaGPUManager) PodFitsDevice(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, runGrpScheduler bool) {
	TranslateGPUResorces(nodeInfo, podInfo)
	if runGrpScheduler {
		return grpalloc.PodFitsGroupConstraints(nodeInfo, podInfo, false)
	}
}

func (ns *NvidiaGPUScheduler) PodAllocate(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, runGrpScheduler bool) {
	TranslateGPUResorces(nodeInfo, podInfo)
	if runGrpScheduler {
		return grpalloc.PodFitsGroupConstraints(nodeInfo, podInfo, true)
	}
}

func (ns *NvidiaGPUScheduler) GetName() string {
	return "nvidiagpu"
}

func (ns *NvidiaGPUScheduler) UsingGroupScheduler bool {
	return true
}

