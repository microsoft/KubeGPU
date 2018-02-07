package kubeinterface

import (
	"encoding/json"
	"reflect"
	"testing"
	"fmt"
	"github.com/Microsoft/KubeGPU/types"
	"github.com/Microsoft/KubeGPU/utils"
	kubev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func rq (i int64) resource.Quantity {
	return *resource.NewQuantity(i, resource.DecimalSI)
}

func compareContainer(cont0 *types.ContainerInfo, cont1 *types.ContainerInfo) {
	if (true) {
		if !reflect.DeepEqual(cont0.KubeRequests, cont1.KubeRequests) {
			fmt.Printf("KubeReqs don't match\n0:\n%v\n1:\n%v\n", cont0.KubeRequests, cont1.KubeRequests)
		}
		if !reflect.DeepEqual(cont0.Requests, cont1.Requests) {
			fmt.Printf("Reqs don't match\n0:\n%v\n1:\n%v\n", cont0.Requests, cont1.Requests)
		}
		if !reflect.DeepEqual(cont0.DevRequests, cont1.DevRequests) {
			fmt.Printf("DevReqs don't match\n0:\n%v\n1:\n%v\n", cont0.DevRequests, cont1.DevRequests)
		}
		if !reflect.DeepEqual(cont0.AllocateFrom, cont1.AllocateFrom) {
			fmt.Printf("AllocateFrom don't match\n0:\n%v\n1:\n%v\n", cont0.AllocateFrom, cont1.AllocateFrom)
		}
		if !reflect.DeepEqual(cont0.Scorer, cont1.Scorer) {
			fmt.Printf("Scorer don't match\n0:\n%v\n1:\n%v\n", cont0.Scorer, cont1.Scorer)
		}
	}
}

func compareContainers(conts0 map[string]types.ContainerInfo, conts1 map[string]types.ContainerInfo) {
	for contName0, cont0 := range conts0 {
		cont1, ok := conts1[contName0]
		if !ok {
			fmt.Printf("1 does not have container %s\n", contName0)
		} else {
			fmt.Printf("Compare container %s\n", contName0)
			compareContainer(&cont0, &cont1)
		}
	}
	for contName1, _ := range conts1 {
		_, ok := conts0[contName1]
		if !ok {
			fmt.Printf("0 does not have container %s\n", contName1)
		}
	}
}

func comparePod(pod0 *types.PodInfo, pod1 *types.PodInfo) {
	if pod0.Name != pod1.Name {
		fmt.Printf("Name does not match %s %s\n", pod0.Name, pod1.Name)
	}
	if pod0.NodeName != pod1.NodeName {
		fmt.Printf("Nodename does not match %s %s\n", pod0.NodeName, pod1.NodeName)
	}
	compareContainers(pod0.InitContainers, pod1.InitContainers)
	compareContainers(pod0.RunningContainers, pod1.RunningContainers)
}

func TestConvert(t *testing.T) {
	// test node conversion
	nodeMeta := &metav1.ObjectMeta{Annotations: map[string]string{"OtherAnnotation" : "OtherAnnotationValue"}}
	nodeInfo := &types.NodeInfo{
		Name: "Node0",
		Capacity: types.ResourceList{"A": 245, "B": 300},
		Allocatable: types.ResourceList{"A": 200, "B": 100},
		Used: types.ResourceList{"A": 0, "B": 0},
		Scorer: types.ResourceScorer{"A": 4}, // no scorer for resource "B" is provided
	}
	NodeInfoToAnnotation(nodeMeta, nodeInfo)
	jsonNode, _ := json.Marshal(nodeInfo)
	annotationExpect := map[string]string{
		"OtherAnnotation": "OtherAnnotationValue",
		"node.alpha/DeviceInformation" : string(jsonNode),
		// "NodeInfo/Name": "Node0",
		// "NodeInfo/Capacity/A": "245",
		// "NodeInfo/Capacity/B": "300",
		// "NodeInfo/Allocatable/A": "200",
		// "NodeInfo/Allocatable/B": "100",
		// "NodeInfo/Used/A": "0",
		// "NodeInfo/Used/B": "0",
		// "NodeInfo/Scorer/A": "4", 
	}
	if !reflect.DeepEqual(annotationExpect, nodeMeta.Annotations) {
		t.Errorf("Node info annotations not what is expected, expected: %+v, have: %+v", annotationExpect, nodeMeta.Annotations)
	}
	nodeInfoGet, err := AnnotationToNodeInfo(nodeMeta)
	if err != nil {
		t.Errorf("Error encountered when converting annotation to node info: %v", err)
	}
	if !reflect.DeepEqual(nodeInfo, nodeInfoGet) {
		t.Errorf("Get node is not same, expect: %+v, get: %+v", nodeInfo, nodeInfoGet)
	}

	// test pod conversion
	init0 := types.ContainerInfo{
		Requests : types.ResourceList{"alpha/grpresource/gpu/0/cards" : 1, "alpha/grpresource/gpu/0/memory" : 100000},
	}
	run0 := types.ContainerInfo{
		Requests : types.ResourceList{"alpha/grpresource/gpu/A/cards" : 4},
		AllocateFrom : types.ResourceLocation{"alpha/grpresource/gpu/0/cards": "CARD1"},
		DevRequests : types.ResourceList{"alpha/grpresource/gpugrp1/A/gpu/0/cards": 90},
	}
	run1 := types.ContainerInfo{
		Requests : types.ResourceList{"alpha/grpresource/gpu/A/cards": 6},
		Scorer : types.ResourceScorer{"alpha/grpresource/gpu/A/cards": 10},
	}
	pod0 := types.PodInfo{
		NodeName : "NodeB",
		InitContainers : map[string]types.ContainerInfo{"Init0" : init0},
		RunningContainers : map[string]types.ContainerInfo{"Run0" : run0, "Run1" : run1},
	}
	jsonStr, _ := json.Marshal(pod0)
	kubePod := &kubev1.Pod{
		ObjectMeta : metav1.ObjectMeta{
			Name: "Pod0",
			Annotations: map[string]string{
				"ABCD": "EFGH",
				"pod.alpha/DeviceInformation" : string(jsonStr),
				// "PodInfo/InitContainer/Init0/Requests/alpha/grpresource/gpu/0/cards": "1",
				// "PodInfo/InitContainer/Init0/Requests/alpha/grpresource/gpu/0/memory": "100000",
				// "PodInfo/RunningContainer/Run0/Requests/alpha/grpresource/gpu/A/cards": "4",
				// "PodInfo/RunningContainer/Run0/AllocateFrom/alpha/grpresource/gpu/0/cards": "CARD1",
				// "PodInfo/RunningContainer/Run0/DevRequests/alpha/grpresource/gpugrp1/A/gpu/0/cards": "90",
				// "PodInfo/RunningContainer/Run1/Requests/alpha/grpresource/gpu/A/cards": "6",
				// "PodInfo/RunningContainer/Run1/Scorer/alpha/grpresource/gpu/A/cards": "10",
				// "PodInfo/ValidForNode": "NodeB",
			},
		},
		Spec : kubev1.PodSpec{
			InitContainers: []kubev1.Container{
				{
					Name: "Init0",
					Image: "BCDE",
					Resources: kubev1.ResourceRequirements{
						Requests: kubev1.ResourceList{"CPU": rq(4), "Memory": rq(100000), "Other": rq(20)},
						Limits: kubev1.ResourceList{"CPU": rq(10)},
					},
				},
			},
			Containers: []kubev1.Container{
				{
					Name: "Run0",
					Image: "RunBCDE",
					Resources: kubev1.ResourceRequirements{
						Requests: kubev1.ResourceList{"CPU": rq(8), "Memory": rq(200000)},
					},
				},
				{
					Name: "Run1",
					Image: "RunBCDE",
					Resources: kubev1.ResourceRequirements{
						Requests: kubev1.ResourceList{"CPU": rq(4), "Memory": rq(300000), "alpha.kubernetes.io/nvidia-gpu": rq(2)},
					},
				},
			},
		},
	}

	// convert to pod info and clear some annotations
	podInfo, err := KubePodInfoToPodInfo(kubePod, true)
	if err != nil {
		t.Errorf("encounter error %v", err)
	}
	expectedPodInfo := &types.PodInfo{
		Name: "Pod0",
		NodeName: "",
		InitContainers: map[string]types.ContainerInfo{
			"Init0" :
			{
				KubeRequests: types.ResourceList{"CPU": 4, "Memory": 100000, "Other": 20},
				Requests: types.ResourceList{"alpha/grpresource/gpu/0/cards": 1, "alpha/grpresource/gpu/0/memory": 100000},
				DevRequests: types.ResourceList{"alpha/grpresource/gpu/0/cards": 1, "alpha/grpresource/gpu/0/memory": 100000},
				AllocateFrom: types.ResourceLocation{},
				Scorer: types.ResourceScorer{},
			},
		},
		RunningContainers: map[string]types.ContainerInfo{
			"Run0" :
			{
				KubeRequests: types.ResourceList{"CPU": 8, "Memory": 200000},
				Requests: types.ResourceList{"alpha/grpresource/gpu/A/cards": 4},
				DevRequests: types.ResourceList{"alpha/grpresource/gpu/A/cards": 4},
				AllocateFrom: types.ResourceLocation{},
				Scorer: types.ResourceScorer{},
			},
			"Run1" :
			{
				KubeRequests: types.ResourceList{"CPU": 4, "Memory": 300000, "alpha.kubernetes.io/nvidia-gpu": 2},
				Requests: types.ResourceList{"alpha/grpresource/gpu/A/cards": 6},
				DevRequests: types.ResourceList{"alpha/grpresource/gpu/A/cards": 6},
				AllocateFrom: types.ResourceLocation{},
				Scorer: types.ResourceScorer{"alpha/grpresource/gpu/A/cards": 10},
			},
		},
	}
	if !reflect.DeepEqual(podInfo, expectedPodInfo) {
		t.Errorf("PodInfo is not what is expected\n expect:\n%+v\n have:\n%+v", expectedPodInfo, podInfo)
		comparePod(podInfo, expectedPodInfo)
	}

	// set allocate from and devrequests after translation and allocation
	contCopy := podInfo.InitContainers["Init0"]
	contCopy.DevRequests = types.ResourceList{"alpha/grpresource/gpugrp/0/gpu/0/cards": 1, "alpha/grpresource/gpugrp/0/gpu/0/memory": 200000}
	contCopy.AllocateFrom = types.ResourceLocation{
		"alpha/grpresource/gpugrp/0/gpu/0/cards": "alpha/grpresource/gpugrp/A/gpu/12/cards",
		"alpha/grpresource/gpugrp/0/gpu/0/memory": "alpha/grpresource/gpugrp/A/gpu/12/memory",
	}
	podInfo.InitContainers["Init0"] = contCopy

	contCopy = podInfo.RunningContainers["Run0"]
	contCopy.DevRequests = types.ResourceList{"alpha/grpresource/gpugrp/A/gpu/0/cards": 4}
	contCopy.AllocateFrom = types.ResourceLocation{
		"alpha/grpresource/gpugrp/A/gpu/0/cards": "alpha/grpresource/gpugrp/0/gpu/43-21/cards",
	}
	podInfo.RunningContainers["Run0"] = contCopy

	contCopy = podInfo.RunningContainers["Run1"]
	contCopy.DevRequests = types.ResourceList{}
	podInfo.RunningContainers["Run1"] = contCopy

	podInfo.NodeName = "NodeNewD"

	// clear existing annotations
	ClearPodInfoAnnotations(&kubePod.ObjectMeta)
	// convert to annotations
	PodInfoToAnnotation(&kubePod.ObjectMeta, podInfo)

	jsonStr, _ = json.Marshal(podInfo)
	expectedAnnotations := map[string]string{
		"ABCD": "EFGH", // existing
		"pod.alpha/DeviceInformation" : string(jsonStr),
		// "PodInfo/InitContainer/Init0/Requests/alpha/grpresource/gpu/0/cards": "1",
		// "PodInfo/InitContainer/Init0/Requests/alpha/grpresource/gpu/0/memory": "100000",
		// "PodInfo/RunningContainer/Run0/Requests/alpha/grpresource/gpu/A/cards": "4",
		// "PodInfo/RunningContainer/Run1/Requests/alpha/grpresource/gpu/A/cards": "6",
		// "PodInfo/RunningContainer/Run1/Scorer/alpha/grpresource/gpu/A/cards": "10",
		// "PodInfo/RunningContainer/Run0/DevRequests/alpha/grpresource/gpugrp/A/gpu/0/cards": "4",
		// "PodInfo/RunningContainer/Run0/AllocateFrom/alpha/grpresource/gpugrp/A/gpu/0/cards": "alpha/grpresource/gpugrp/0/gpu/43-21/cards",
		// "PodInfo/InitContainer/Init0/DevRequests/alpha/grpresource/gpugrp/0/gpu/0/cards": "1",
		// "PodInfo/InitContainer/Init0/DevRequests/alpha/grpresource/gpugrp/0/gpu/0/memory": "200000",
		// "PodInfo/InitContainer/Init0/AllocateFrom/alpha/grpresource/gpugrp/0/gpu/0/cards": "alpha/grpresource/gpugrp/A/gpu/12/cards",
		// "PodInfo/InitContainer/Init0/AllocateFrom/alpha/grpresource/gpugrp/0/gpu/0/memory": "alpha/grpresource/gpugrp/A/gpu/12/memory",
		// "PodInfo/ValidForNode": "NodeNewD",
	}
	if !reflect.DeepEqual(kubePod.ObjectMeta.Annotations, expectedAnnotations)  {
		t.Errorf("Pod annotations are not what is expected\nexpect:\n%v\nhave:\n%v", expectedAnnotations, kubePod.ObjectMeta.Annotations)
		utils.CompareMapStringString(expectedAnnotations, kubePod.ObjectMeta.Annotations)
	}

	// convert back and check podinfo
	podInfo2, err := KubePodInfoToPodInfo(kubePod, false)
	if err != nil {
		t.Errorf("encounter error %v", err)
	}
	if !reflect.DeepEqual(podInfo, podInfo2) {
		t.Errorf("Get back Pod info is not correct\nexpect:\n%v\nhave:\n%v", podInfo, podInfo2)
		comparePod(podInfo, podInfo2)
	}
}

