package gpuschedulerplugin

import (
	"fmt"

	"github.com/Microsoft/KubeGPU/device-scheduler/grpalloc"
	sctypes "github.com/Microsoft/KubeGPU/device-scheduler/types"
	gputypes "github.com/Microsoft/KubeGPU/plugins/gpuplugintypes"
	"github.com/Microsoft/KubeGPU/types"
	"github.com/golang/glog"
)

const (
	// auto topology generation "0" means default (everything in its own group)
	GPUTopologyGeneration types.ResourceName = "alpha.gpu/gpu-generate-topology"
)

type NvidiaGPUScheduler struct {
}

func TranslateGPUContainerResources(alloc types.ResourceList, cont types.ContainerInfo) types.ResourceList {
	numGPUs := cont.Requests[gputypes.ResourceGPU] // get from annotation, don't use default KubeRequests as this must be set to zero
	return TranslateGPUResources(numGPUs, alloc, cont.DevRequests)
}

func TranslateGPUResorces(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) (error, bool) {
	if podInfo.Requests[GPUTopologyGeneration] == int64(0) { // zero implies no topology, or topology explictly given
		for contName, contCopy := range podInfo.InitContainers {
			contCopy.DevRequests = TranslateGPUContainerResources(nodeInfo.Allocatable, contCopy)
			podInfo.InitContainers[contName] = contCopy
		}
		for contName, contCopy := range podInfo.RunningContainers {
			contCopy.DevRequests = TranslateGPUContainerResources(nodeInfo.Allocatable, contCopy)
			podInfo.RunningContainers[contName] = contCopy
		}
		return nil, true
	} else if podInfo.Requests[GPUTopologyGeneration] == int64(1) {
		found := ConvertToBestGPURequests(podInfo) // found a tree
		return nil, found
	} else {
		glog.Errorf("Invalid topology generation request %v", podInfo.Requests[GPUTopologyGeneration])
		return fmt.Errorf("Invalid topology generation request"), false
	}
}

func (ns *NvidiaGPUScheduler) AddNode(nodeName string, nodeInfo *types.NodeInfo) {
	AddResourcesToNodeTreeCache(nodeName, nodeInfo.Allocatable)
}

func (ns *NvidiaGPUScheduler) RemoveNode(nodeName string) {
	RemoveNodeFromNodeTreeCache(nodeName)
}

func (ns *NvidiaGPUScheduler) PodFitsDevice(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, fillAllocateFrom bool, runGrpScheduler bool) (bool, []sctypes.PredicateFailureReason, float64) {
	err, found := TranslateGPUResorces(nodeInfo, podInfo)
	if err != nil {
		//panic("Unexpected error")
		return false, nil, 0.0
	}
	if !found {
		return false, nil, 0.0
	}
	if runGrpScheduler {
		glog.V(5).Infof("Running group scheduler on device requests %+v", podInfo)
		return grpalloc.PodFitsGroupConstraints(nodeInfo, podInfo, fillAllocateFrom)
	}
	return true, nil, 0.0
}

func (ns *NvidiaGPUScheduler) PodAllocate(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, runGrpScheduler bool) error {
	err, found := TranslateGPUResorces(nodeInfo, podInfo)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("TranslateGPUResorces fails as no translation is found")
	}
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
