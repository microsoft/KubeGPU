package gpuschedulerplugin

import (
	"fmt"

	"github.com/Microsoft/KubeDevice-API/pkg/devicescheduler"
	types "github.com/Microsoft/KubeDevice-API/pkg/types"
	gtype "github.com/Microsoft/KubeGPU/gpuplugintypes"
)

const (
	// auto topology generation "0" means default (everything in its own group)
	GPUTopologyGeneration types.ResourceName = "gpu/gpu-generate-topology"
)

type NvidiaGPUScheduler struct {
}

// force translation to two levels
func (ns *NvidiaGPUScheduler) AddNode(nodeName string, nodeInfo *types.NodeInfo) {
	modReq := TranslateGPUResources(nodeInfo.KubeAlloc[gtype.ResourceGPU], types.ResourceList{
		types.DeviceGroupPrefix + "/gpugrp1/A/gpugrp0/B/gpu/GPU0/cards": int64(1),
	}, nodeInfo.Allocatable)
	nodeInfo.Allocatable = modReq
	AddResourcesToNodeTreeCache(nodeName, nodeInfo.Allocatable)
}

func (ns *NvidiaGPUScheduler) RemoveNode(nodeName string) {
	RemoveNodeFromNodeTreeCache(nodeName)
}

func (ns *NvidiaGPUScheduler) PodFitsDevice(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, fillAllocateFrom bool) (bool, []devicescheduler.PredicateFailureReason, float64) {
	err, found := TranslatePodGPUResources(nodeInfo, podInfo)
	if err != nil {
		//panic("Unexpected error")
		return false, nil, 0.0
	}
	if !found {
		return false, nil, 0.0
	}
	return true, nil, 0.0
}

func (ns *NvidiaGPUScheduler) PodAllocate(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) error {
	err, found := TranslatePodGPUResources(nodeInfo, podInfo)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("TranslatePodGPUResources fails as no translation is found")
	}
	return nil
}

func (ns *NvidiaGPUScheduler) TakePodResources(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) error {
	return nil
}

func (ns *NvidiaGPUScheduler) ReturnPodResources(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) error {
	return nil
}

func (ns *NvidiaGPUScheduler) GetName() string {
	return "nvidiagpu"
}

func (ns *NvidiaGPUScheduler) UsingGroupScheduler() bool {
	return true
}
