package nvidia

import (
	"fmt"

	"github.com/Microsoft/KubeGPU/gpuextension/gpu"
	"github.com/Microsoft/KubeGPU/gpuextension/grpalloc"
	"github.com/Microsoft/KubeGPU/types"
	"github.com/golang/glog"
)

type NvidiaGPUScheduler struct {
}

func TranslateGPUContainerResources(alloc types.ResourceList, cont types.ContainerInfo) types.ResourceList {
	numGPUs := cont.Requests[types.ResourceGPU] // get from annotation, don't use default KubeRequests as this must be set to zero
	return gpu.TranslateGPUResources(numGPUs, alloc, cont.DevRequests)
}

func TranslateGPUResorces(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) {
	autoGenerateTopology := 0 // zero implies no topology desired, or it is explictly given
	for contName, contCopy := range podInfo.InitContainers {
		if contCopy.Requests[types.GPUTopologyGeneration] != int64(0) {
			autoGenerateTopology = int(contCopy.Requests[types.GPUTopologyGeneration])
			break
		}
		contCopy.DevRequests = TranslateGPUContainerResources(nodeInfo.Allocatable, contCopy)
		podInfo.InitContainers[contName] = contCopy
	}
	if autoGenerateTopology == 0 {
		for contName, contCopy := range podInfo.RunningContainers {
			if contCopy.Requests[types.GPUTopologyGeneration] != int64(0) {
				autoGenerateTopology = int(contCopy.Requests[types.GPUTopologyGeneration])
				break
			}
			contCopy.DevRequests = TranslateGPUContainerResources(nodeInfo.Allocatable, contCopy)
			podInfo.RunningContainers[contName] = contCopy
		}
	}
	if autoGenerateTopology == 0 {
		// nothing
	} else if autoGenerateTopology == 1 {
		gpu.ConvertToBestGPURequests(podInfo)
	}
}

func (ns *NvidiaGPUScheduler) PodFitsDevice(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, fillAllocateFrom bool, runGrpScheduler bool) (bool, []types.PredicateFailureReason, float64) {
	TranslateGPUResorces(nodeInfo, podInfo)
	if runGrpScheduler {
		glog.V(5).Infof("Running group scheduler on device requests %+v", podInfo)
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
