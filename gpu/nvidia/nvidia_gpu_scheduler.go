package nvidia

import (
	"fmt"

	"github.com/Microsoft/KubeGPU/gpu"
	"github.com/Microsoft/KubeGPU/types"
	"github.com/Microsoft/KubeGPU/grpalloc"
)

type NvidiaGPUScheduler struct {
}

func TranslateGPUContainerResources(alloc types.ResourceList, cont types.ContainerInfo) types.ResourceList {
	numGPUs := cont.KubeRequests[types.ResourceNvidiaGPU]
	return gpu.TranslateGPUResources(numGPUs, alloc, cont.DevRequests)
}

func TranslateGPUResorces(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) {
	for index := range podInfo.InitContainers {
		podInfo.InitContainers[index].DevRequests = TranslateGPUContainerResources(nodeInfo.Allocatable, podInfo.InitContainers[index])
	}
	for index := range podInfo.RunningContainers {
		podInfo.RunningContainers[index].DevRequests = TranslateGPUContainerResources(nodeInfo.Allocatable, podInfo.RunningContainers[index])
	}
}

func (ns *NvidiaGPUScheduler) PodFitsDevice(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, fillAllocateFrom bool, runGrpScheduler bool) (bool, []types.PredicateFailureReason, float64) {
	TranslateGPUResorces(nodeInfo, podInfo)
	if runGrpScheduler {
		return grpalloc.PodFitsGroupConstraints(nodeInfo, podInfo, fillAllocateFrom)
	}
	return true, nil, 0.0
}

func (ns *NvidiaGPUScheduler) PodAllocate(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, runGrpScheduler bool) error {
	TranslateGPUResorces(nodeInfo, podInfo)
	if runGrpScheduler {
		fits, reasons, _ := grpalloc.PodFitsGroupConstraints(nodeInfo, podInfo, true)
		if !fits {
			return fmt.Errorf("Scheduler unable to allocate pod %s as pod no longer fits: %v", podInfo.Name, reasons)
		}
	}
	return nil
}

func (ns *NvidiaGPUScheduler) TakePodResources(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, runGrpScheduler bool) error {
	if runGrpScheduler {
		grpalloc.TakePodGroupResource(nodeInfo, podInfo)
	}
	return nil
}

func (ns *NvidiaGPUScheduler) ReturnPodResources(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, runGrpScheduler bool) error {
	if runGrpScheduler {
		grpalloc.ReturnPodGroupResource(nodeInfo, podInfo)
	}
	return nil
}

func (ns *NvidiaGPUScheduler) GetName() string {
	return "nvidiagpu"
}

func (ns *NvidiaGPUScheduler) UsingGroupScheduler() bool {
	return true
}

