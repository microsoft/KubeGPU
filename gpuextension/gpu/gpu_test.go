package gpu

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Microsoft/KubeGPU/types"
)

func TestTree(t *testing.T) {
	nodeRes1 := types.ResourceList{
		"alpha/grpresource/gpugrp1/A/gpugrp0/0/gpu/0/cards": 1,
		"alpha/grpresource/gpugrp1/A/gpugrp0/0/gpu/1/cards": 1,
		"alpha/grpresource/gpugrp1/A/gpugrp0/1/gpu/2/cards": 1,
		"alpha/grpresource/gpugrp1/A/gpugrp0/1/gpu/3/cards": 1,
		"alpha/grpresource/gpugrp1/B/gpugrp0/2/gpu/4/cards": 1,
		"alpha/grpresource/gpugrp1/B/gpugrp0/2/gpu/5/cards": 1,
		"alpha/grpresource/gpugrp1/B/gpugrp0/3/gpu/6/cards": 1,
		"alpha/grpresource/gpugrp1/B/gpugrp0/3/gpu/7/cards": 1,
	}
	nodeRes2 := types.ResourceList{
		"alpha/grpresource/gpugrp1/A/gpugrp0/0/gpu/0/cards": 1,
		"alpha/grpresource/gpugrp1/A/gpugrp0/0/gpu/1/cards": 1,
		"alpha/grpresource/gpugrp1/A/gpugrp0/1/gpu/2/cards": 1,
		"alpha/grpresource/gpugrp1/A/gpugrp0/1/gpu/3/cards": 1,
		"alpha/grpresource/gpugrp1/B/gpugrp0/2/gpu/4/cards": 1,
		"alpha/grpresource/gpugrp1/B/gpugrp0/2/gpu/5/cards": 1,
		"alpha/grpresource/gpugrp1/B/gpugrp0/2/gpu/6/cards": 1,
		"alpha/grpresource/gpugrp1/B/gpugrp0/2/gpu/7/cards": 1,
	}
	nodeRes3 := nodeRes1
	node := addToNode(nil, nodeRes1, "gpugrp", "cards", 1)
	nodeScore := computeTreeScore(node)
	types.PrintTreeNode(node)
	fmt.Printf("TreeScore: %v\n", nodeScore)
	node = addToNode(nil, nodeRes2, "gpugrp", "cards", 1)
	nodeScore = computeTreeScore(node)
	types.PrintTreeNode(node)
	fmt.Printf("TreeScore: %v\n", nodeScore)
	AddResourcesToNodeTreeCache("A", nodeRes1)
	AddResourcesToNodeTreeCache("B", nodeRes2)
	AddResourcesToNodeTreeCache("C", nodeRes3)
	for key, val := range nodeCacheMap {
		fmt.Printf("Key\n")
		types.PrintTreeNode(key)
		fmt.Printf("Val: %v\n", val)
	}
	RemoveNodeFromNodeTreeCache("A")
	fmt.Printf("After removal\n")
	for key, val := range nodeCacheMap {
		fmt.Printf("Key\n")
		types.PrintTreeNode(key)
		fmt.Printf("Val: %v\n", val)
	}
	//fmt.Printf("Add back\n")
	//AddResourcesToNodeTreeCache("B", nodeRes2)
	podInfo := &types.PodInfo{
		RunningContainers: map[string]types.ContainerInfo{
			"A": {
				Requests: types.ResourceList{types.ResourceGPU: 3},
				DevRequests: types.ResourceList{
					"alpha/grpresource/gpugrp1/B/gpugrp0/3/gpu/6/cards": 1,
					"alpha/grpresource/gpugrp1/B/gpugrp0/3/gpu/7/cards": 1,
				},
			},
		},
	}
	ConvertToBestGPURequests(podInfo)
	//fmt.Printf("New PodInfo: %+v", podInfo)
	expectedPodInfo := &types.PodInfo{
		RunningContainers: map[string]types.ContainerInfo{
			"A": {
				Requests: types.ResourceList{types.ResourceGPU: 3},
				DevRequests: types.ResourceList{
					"alpha/grpresource/gpugrp1/0/gpugrp0/0/gpu/0/cards": 1,
					"alpha/grpresource/gpugrp1/0/gpugrp0/0/gpu/1/cards": 1,
					"alpha/grpresource/gpugrp1/0/gpugrp0/0/gpu/2/cards": 1,
				},
			},
		},
	}
	if !reflect.DeepEqual(podInfo, expectedPodInfo) {
		t.Errorf("Pod A not equal\nHave:\n%+v\nExpect:\n%+v", podInfo, expectedPodInfo)
	}
	RemoveNodeFromNodeTreeCache("B")
	fmt.Printf("Now should have only one\n")
	for key, val := range nodeCacheMap {
		fmt.Printf("Key\n")
		types.PrintTreeNode(key)
		fmt.Printf("Val: %v\n", val)
	}
	fmt.Printf("LocationMap :%v\n", nodeLocationMap)
	ConvertToBestGPURequests(podInfo)
	expectedPodInfo = &types.PodInfo{
		RunningContainers: map[string]types.ContainerInfo{
			"A": {
				Requests: types.ResourceList{types.ResourceGPU: 3},
				DevRequests: types.ResourceList{
					"alpha/grpresource/gpugrp1/0/gpugrp0/0/gpu/0/cards": 1,
					"alpha/grpresource/gpugrp1/0/gpugrp0/0/gpu/1/cards": 1,
					"alpha/grpresource/gpugrp1/0/gpugrp0/1/gpu/0/cards": 1,
				},
			},
		},
	}
	if !reflect.DeepEqual(podInfo, expectedPodInfo) {
		t.Errorf("Pod B not equal\nHave:\n%+v\nExpect:\n%+v", podInfo, expectedPodInfo)
	}
}
