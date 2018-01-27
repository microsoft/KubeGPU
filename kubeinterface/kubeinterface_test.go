package kubeinterface

import (
	"reflect"
	"testing"
	"github.com/KubeGPU/types"
	kubev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvert(t *testing.T) {
	// test node conversion
	nodeMeta := &metav1.ObjectMeta{Annotations: map[string]string{"OtherAnnotation" : "OtherAnnotationValue"}}
	nodeInfo := &types.NodeInfo{
		Name: "Node0",
		Capacity: ResourceList{"A": 245, "B": 300},
		Allocatable: ResourceList{"A": 200, "B": 100},
		Used: ResourceList{"A": 0, "B": 0},
		Scorer: ResourceScorer{"A": 4}, // no scorer for resource "B" is provided
	}
	NodeInfoToAnnotation(nodeMeta, nodeInfo)
	annotationExpect := map[string]string{
		"OtherAnnotation": "OtherAnnotationValue",
		"NodeInfo/Name": "Node0",
		"NodeInfo/Capacity/A": "245",
		"NodeInfo/Capacity/B": "300",
		"NodeInfo/Allocatable/A": "200",
		"NodeInfo/Allocatable/B": "100",
		"NodeInfo/Used/A": "0",
		"NodeInfo/Used/B": "0",
		"NodeInfo/Scorer/A": "4", 
	}
	nodeInfoGet, err := AnnotationToNodeInfo(nodeMeta)
	if err != nil {
		t.Errorf("Error encountered when converting annotation to node info: %v", err)
	}
	if !reflect.DeepEqual(nodeInfo, nodeInfoGet) {
		t.Errorf("Get node is not same, expect: %v, get: %v", nodeInfo, nodeInfoGet)
	}

	// test pod conversion
	kubePod := &kubev1.Pod{
		metav1.ObjectMeta{
			Name: "Pod0",
			Annotations: map[string]string{
				"ABCD": "EFGH",
				"PodInfo/InitContainer/Init0/Requests/alpha/devresource/gpu/0/cards": 1,
				"PodInfo/InitContainer/Init0/Requests/alpha/devresource/gpu/0/memory": 100000,
				"PodInfo/RunningContainer/Run0/Requests/alpha/devresource/gpu/A/cards": 4,
				"PodInfo/RunningContainer/Run0/AllocateFrom/alpha/devresource/gpu/0/cards": "CARD1",
				"PodInfo/RunningContainer/Run0/DevRequests/alpha/devresource/gpugrp1/A/gpu/0/cards": 90,
				"PodInfo/RunningContainer/Run1/Requests/alpha/devresource/gpu/A/cards": 6,
				"PodInfo/RunningContainer/Run1/Scorer/alpha/devresource/gpu/A/cards": 10,
				"PodInfo/ValidForNode": "NodeB",
			},
		},
		kubev1.PodSpec{
			InitContainers: []kubev1.Container{
				{
					Name: "Init0",
					Image: "BCDE",
					Resources: kubev1.ResourceRequirements{
						Requests: kubev1.ResourceList{"CPU": 4, "Memory": 100000, "Other": 20},
						Limits: kubev1.ResourceList{"CPU": 10},
					},
				},
			},
			Containers: []kubev1.Container{
				{
					Name: "Run0",
					Image: "RunBCDE",
					Resources: kubev1.ResourceRequirements{
						Requests: kubev1.ResourceList{"CPU": 8, "Memory": 200000},
					},
				},
				{
					Name: "Run1",
					Image: "RunBCDE",
					Resources: kubev1.ResourceRequirements{
						Requests: kubev1.ResourceList{"CPU": 4, "Memory": 300000, "alpha.kubernetes.io/nvidia-gpu": 2},
					},
				},
			},
		},
	}

	// convert to pod info and clear
	podInfo, err := kubeinterface.KubePodInfoToPodInfo(kubePod, true)
	expectedPodInfo := &{
		Name: "Pod0",
		NodeName: "",
		InitContainers: []ContainerInfo{
			{
				Name: "Init0",
				KubeRequests: types.ResourceList{"CPU": 4, "Memory": 100000, "Other": 20},
				Requests: types.ResourceList{"alpha/devresource/gpu/0/cards": 1, "devresource/gpu/0/memory": 100000},
				DevRequests: types.ResourceList{"alpha/devresource/gpu/0/cards": 1, "devresource/gpu/0/memory": 100000},
				AllocateFrom: types.ResourceLocation{},
				Scorer: types.ResourceScorer{},
			},
		},
		RunningContainers: []ContainerInfo{
			{
				Name: "Run0",
				KubeRequests: types.ResourceList{"CPU": 8, "Memory": 200000},
				Requests: types.ResourceList{"devresource/gpu/A/cards": 4},
				DevRequests: types.ResourceList{"devresource/gpu/A/cards": 4},
				AllocateFrom: types.ResourceList{},
				Scorer: types.ResourceScorer{},
			},
			{
				Name: "Run1",
				KubeRequests: types.ResourceList{"CPU": 4, "Memory": 300000, "alpha.kubernetes.io/nvidia-gpu": 2},
				Requests: types.ResourceList{"alpha/devresource/gpu/A/cards": 6},
				DevRequests: types.ResourceList{"alpha/devresource/gpu/A/cards": 6},
				AllocateFrom: types.ResourceList{},
				Scorer: types.ResourceScorer{"alpha/devresource/gpu/A/cards": 10},
			},
		},
	}
	if !reflect.DeepEqual(podInfo, expectedPodInfo) {
		t.Errorf("PodInfo is not what is expected, expect: %v, have: %v", expectedPodInfo, podInfo)
	}

	// set allocate from and devrequests
	podInfo.RunningContainers[0].DevRequests = types.ResourceList{"alpha/devresource/gpugrp/A/gpu/0/cards": 4}
	podInfo.InitContainers[0].DevRequests = types.ResourceList{"alpha/devresource/gpugrp/0/gpu/0/cards": 1, "alpha/devresource/gpugrp/0/gpu/0/memory": 200000}
	podInfo.RunningContainers[0].AllocateFrom = typpes.ResourceLocation{"alpha/devresource/gpugrp/A/gpu/0/cards", "alpha/devresource/gpugrp/0/gpu/43-21/cards"}
	// convert to annotations
	kubeinterface.PodInfoToAnnotation(&kubePod.ObjectMeta, podInfo)
	expectedAnnotations := map[string]string{
		"ABCD": "EFGH",
		"PodInfo/InitContainer/Init0/Requests/alpha/devresource/gpu/0/cards": 1,
		"PodInfo/InitContainer/Init0/Requests/alpha/devresource/gpu/0/memory": 100000,
		"PodInfo/RunningContainer/Run0/Requests/alpha/devresource/gpu/A/cards": 4,
		"PodInfo/RunningContainer/Run0/AllocateFrom/alpha/devresource/gpu/0/cards": "CARD1",
		"PodInfo/RunningContainer/Run0/DevRequests/alpha/devresource/gpugrp1/A/gpu/0/cards": 90,
		"PodInfo/RunningContainer/Run1/Requests/alpha/devresource/gpu/A/cards": 6,
		"PodInfo/RunningContainer/Run1/Scorer/alpha/devresource/gpu/A/cards": 10,
		"PodInfo/ValidForNode": "NodeB",
		
	}
}

func NodeInfoToAnnotation(meta *metav1.ObjectMeta, nodeInfo *types.NodeInfo) {
	unc AnnotationToNodeInfo(meta *metav1.ObjectMeta) (*types.NodeInfo, error)

	func KubePodInfoToPodInfo(kubePodInfo *kubev1.Pod, invalidateExistingAnnotations bool)
	func PodInfoToAnnotation(meta *metav1.ObjectMeta, podInfo *types.PodInfo, nodeName string) {