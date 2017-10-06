package grpalloc

// to test: run
// go test --args -log_dir=/home/sanjeevm/logs -v=10
// "--args" separate arguments to binary (after compiling)
// -log_dir=/home/sanjeevm/logs -v=10 are arguments to main program for logging

import (
	"bytes"
	"encoding/gob"
	"flag"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/MSRCCS/grpalloc/devicemanager"

	"github.com/MSRCCS/grpalloc/types"
	"github.com/golang/glog"

	"regexp"
)

type cont struct {
	name           string
	res            map[string]int64
	grpres         map[string]int64
	expectedGrpLoc map[string]string
}

type PodEx struct {
	pod           *types.PodInfo
	podOrig       *types.PodInfo
	expectedScore float64
	icont         []cont
	rcont         []cont
}

func printContainerAllocation(cont *types.ContainerInfo) {
	//glog.V(5).Infoln("Allocated", cont.Resources.Allocated)
	sortedKeys := types.SortedStringKeys(cont.Requests)
	for _, resKey := range sortedKeys {
		resVal := cont.Requests[types.ResourceName(resKey)]
		fmt.Println("Resource", cont.Name+"/"+string(resKey),
			"TakenFrom", cont.AllocateFrom[types.ResourceName(resKey)],
			"Amt", resVal)
	}
}

func printPodAllocation(spec *types.PodInfo) {
	fmt.Printf("\nRunningContainers\n\n")
	for _, cont := range spec.RunningContainers {
		printContainerAllocation(&cont)
		fmt.Printf("\n")
	}
	fmt.Printf("\nInitContainers\n\n")
	for _, cont := range spec.InitContainers {
		printContainerAllocation(&cont)
		fmt.Printf("\n")
	}
}

func setRes(res types.ResourceList, name string, amt int64) {
	res[types.ResourceName(name)] = amt
}

func setGrpRes(res types.ResourceList, name string, amt int64) {
	fullName := types.ResourceName(types.ResourceGroupPrefix + "/" + name)
	res[fullName] = amt
}

func addContainer(cont *[]types.ContainerInfo, name string) types.ResourceList {
	c := types.NewContainerInfo()
	c.Name = name
	*cont = append(*cont, *c)
	return c.Requests
}

// ResourceList is a map, no need for pointer
func setResource(alloc types.ResourceList, res map[string]int64, grpres map[string]int64) {
	// set resource
	for key, val := range res {
		setRes(alloc, key, val)
	}
	// set group resource
	for key, val := range grpres {
		setGrpRes(alloc, key, val)
	}
}

type nodeArgs struct {
	name   string
	res    map[string]int64
	grpres map[string]int64
}

func createNode(name string, res map[string]int64, grpres map[string]int64) (*types.NodeInfo, nodeArgs) {
	alloc := types.ResourceList{}
	setResource(alloc, res, grpres)
	node := types.NewNodeInfo()
	node.Name = name
	node.Capacity = alloc
	node.Allocatable = alloc

	glog.V(7).Infoln("AllocatableResource", len(node.Allocatable), node.Allocatable)

	return node, nodeArgs{name: name, res: res, grpres: grpres}
}

func createNodeArgs(args *nodeArgs) *types.NodeInfo {
	info, _ := createNode(args.name, args.res, args.grpres)
	return info
}

func setExpectedResources(c *cont) {
	expectedGrpLoc := make(map[string]string)
	prefix := make(map[string]string)
	suffix := make(map[string]string)
	re := regexp.MustCompile(`(.*)/(.*)`) // take max in prefix
	for keyRes := range c.grpres {
		matches := re.FindStringSubmatch(keyRes)
		if len(matches) == 3 {
			prefix[keyRes] = matches[1]
			suffix[keyRes] = matches[2]
		} else {
			prefix[keyRes] = ""
			suffix[keyRes] = matches[2]
		}
	}
	for key, val := range c.expectedGrpLoc {
		//re := regexp.MustCompile(key + `/(.*)`)
		if c.grpres != nil {
			for keyRes := range c.grpres {
				//matches := re.FindStringSubmatch(keyRes)
				if strings.HasSuffix(key, prefix[keyRes]) {
					newKey := types.ResourceGroupPrefix + "/" + key + "/" + suffix[keyRes]
					newVal := types.ResourceGroupPrefix + "/" + val + "/" + suffix[keyRes]
					expectedGrpLoc[newKey] = newVal
				}
				// if len(matches) >= 2 {
				// 	newKey := types.ResourceGroupPrefix + "/" + key + "/" + matches[1]
				// 	newVal := types.ResourceGroupPrefix + "/" + val + "/" + matches[1]
				// 	expectedGrpLoc[newKey] = newVal
				// }
			}
		} else {
			newKey := types.ResourceGroupPrefix + "/" + key + "/cards"
			newVal := types.ResourceGroupPrefix + "/" + val + "/cards"
			expectedGrpLoc[newKey] = newVal
		}
	}
	c.expectedGrpLoc = expectedGrpLoc
}

func createPod(name string, expScore float64, iconts []cont, rconts []cont) (*types.PodInfo, *PodEx) {
	pod := types.PodInfo{Name: name}

	glog.V(2).Infof("Working on pod %s", pod.Name)

	var contR types.ResourceList

	for index, icont := range iconts {
		setExpectedResources(&iconts[index])
		contR = addContainer(&pod.InitContainers, icont.name)
		setResource(contR, icont.res, icont.grpres)
		glog.V(7).Infoln(icont.name, pod.InitContainers[index].Requests)
	}
	for index, rcont := range rconts {
		setExpectedResources(&rconts[index])
		contR = addContainer(&pod.RunningContainers, rcont.name)
		setResource(contR, rcont.res, rcont.grpres)
		glog.V(7).Infoln(rcont.name, pod.RunningContainers[index].Requests)
	}

	podEx := PodEx{podOrig: nil, pod: &pod, icont: iconts, rcont: rconts, expectedScore: expScore}

	return &pod, &podEx
}

func translatePod(node *types.NodeInfo, podEx *PodEx) {
	buffer := &bytes.Buffer{}
	enc := gob.NewEncoder(buffer)
	dec := gob.NewDecoder(buffer)
	if podEx.podOrig == nil {
		//deep copy
		enc.Encode(podEx.pod)
		dec.Decode(&podEx.podOrig)
	} else {
		enc.Encode(podEx.podOrig)
		dec.Decode(&podEx.pod)
	}
	devicemanager.TranslateResources(node, podEx.pod)
}

func sampleTest(pod *types.PodInfo, podEx *PodEx, nodeInfo *types.NodeInfo, testCnt int) {
	//fmt.Printf("Node: %v\n", nodeInfo)
	//fmt.Printf("Pod: %v\n", pod)
	// now perform allocation
	found, reasons, score := PodFitsGroupConstraints(nodeInfo, pod, true)
	//fmt.Println("AllocatedFromF", spec.InitContainers[0].Resources)
	fmt.Printf("Test %d\n", testCnt)
	fmt.Printf("Found: %t Score: %f\n", found, score)
	fmt.Printf("Reasons\n")
	for _, reason := range reasons {
		fmt.Println(reason.GetReason())
	}
	if found {
		printPodAllocation(pod)
		usedResources, _ := ComputePodGroupResources(nodeInfo, pod, false)
		TakePodGroupResource(nodeInfo, pod)

		for usedRes, usedAmt := range usedResources {
			fmt.Println("Resource", usedRes, "AmtUsed", usedAmt)
		}
		for usedRes, usedAmt := range nodeInfo.Used {
			fmt.Println("RequestedResource", usedRes, "Amt", usedAmt)
		}
	}
}

func testContainerAllocs(t *testing.T, conts []cont, podConts []types.ContainerInfo, testCnt int) {
	if len(conts) != len(podConts) {
		t.Errorf("Test %d Number of containers don't match - expected %v - have %v", testCnt, len(conts), len(podConts))
		return
	}
	for ci, c := range conts {
		if len(c.expectedGrpLoc) != len(podConts[ci].AllocateFrom) {
			t.Errorf("Test %d Container %s Number of resources don't match - expected %v %v - have %v %v",
				testCnt, c.name,
				len(c.expectedGrpLoc), c.expectedGrpLoc,
				len(podConts[ci].AllocateFrom), podConts[ci].AllocateFrom)
			return
		}
		for key, val := range c.expectedGrpLoc {
			valP, available := podConts[ci].AllocateFrom[types.ResourceName(key)]
			if !available {
				t.Errorf("Test %d Container %s Expected key %v not available", testCnt, c.name, key)
			} else if string(valP) != val {
				t.Errorf("Test %d Container %s Expected value for key %v not same - expected %v - have %v",
					testCnt, c.name, key, val, valP)
			}
		}
	}
}

func testPodResourceUsage(t *testing.T, pod *types.PodInfo, nodeInfo *types.NodeInfo, testCnt int) {
	usedResources, nodeResources := ComputePodGroupResources(nodeInfo, pod, false)
	TakePodGroupResource(nodeInfo, pod)
	if len(usedResources) == 0 {
		t.Errorf("Test %d no resources being used", testCnt)
	}
	for usedRes, usedAmt := range nodeInfo.Used {
		val, available := nodeResources[usedRes]
		if !available {
			t.Errorf("Test %d - expected used resource %v not found", testCnt, usedRes)
		} else {
			if val != usedAmt {
				t.Errorf("Test %d - expected used resource not match have %v - expected %v", testCnt, val, usedAmt)
			}
		}
	}
	// now return the resource and check
	usedResourcesReturn, usedResourcesNode := ComputePodGroupResources(nodeInfo, pod, true)
	if len(usedResources) != len(usedResourcesReturn) {
		t.Errorf("Test %d used resource lengths do not match - now %d - before %d",
			testCnt, len(usedResourcesReturn), len(usedResources))
	}
	for usedRes, usedAmt := range usedResourcesNode {
		if usedAmt != 0 {
			t.Errorf("Test %d resource %v not zero - still have %d", testCnt, usedRes, usedAmt)
		}
	}
}

func testPodAllocs(t *testing.T, pod *types.PodInfo, podEx *PodEx, nodeInfo *types.NodeInfo, testCnt int) {
	//fmt.Printf("Node: %v\n", nodeInfo)
	//fmt.Printf("Pod: %v\n", pod)
	found, _, score := PodFitsGroupConstraints(nodeInfo, pod, true)
	if found {
		if podEx.rcont[0].expectedGrpLoc == nil {
			t.Errorf("Test %d Group allocation found when it should not be found", testCnt)
		} else {
			if math.Abs(score-podEx.expectedScore)/podEx.expectedScore > 0.01 {
				t.Errorf("Test %d Score not correct - expected %v - have %v", testCnt, podEx.expectedScore, score)
			}
			testContainerAllocs(t, podEx.icont, pod.InitContainers, testCnt)
			testContainerAllocs(t, podEx.rcont, pod.RunningContainers, testCnt)
			// repeat - now should go through findScoreAndUpdate path
			found2, _, score2 := PodFitsGroupConstraints(nodeInfo, pod, true)
			if found2 != found || math.Abs(score-score2)/score > 0.01 {
				t.Errorf("Test %d Repeat Score does not match - expected %v %v - have %v %v",
					testCnt, found, score, found2, score2)
			}
			// now test update & release of resources
			testPodResourceUsage(t, pod, nodeInfo, testCnt)
		}
	} else {
		if podEx.rcont[0].expectedGrpLoc != nil {
			t.Errorf("Test %d Group allocation not found when it should be found", testCnt)
		}
	}
}

func TestGrpAllocate1(t *testing.T) {
	testCnt := 0
	flag.Parse()

	// allocatable resources
	nodeInfo, nodeArgs := createNode("node1",
		map[string]int64{"A1": 4000, "B1": 3000},
		map[string]int64{
			"gpu/dev0/memory": 100000, "gpu/dev0/cards": 1,
			"gpu/dev1/memory": 256000, "gpu/dev1/cards": 1, "gpu/dev1/enumType": int64(0x1),
			"gpu/dev2/memory": 257000, "gpu/dev2/cards": 1,
			"gpu/dev3/memory": 192000, "gpu/dev3/cards": 1, "gpu/dev3/enumType": int64(0x1),
			"gpu/dev4/memory": 178000, "gpu/dev4/cards": 1},
	)
	// required resources
	pod, podEx := createPod("pod1", 0.58214,
		[]cont{
			{name: "Init0",
				res:            map[string]int64{"A1": 2200, "B1": 2000},
				grpres:         map[string]int64{"gpu/0/memory": 100000, "gpu/0/cards": 1},
				expectedGrpLoc: map[string]string{"gpu/0": "gpu/dev4"}}},
		[]cont{
			{"Run0",
				map[string]int64{"A1": 3000, "B1": 1000},
				map[string]int64{
					"gpu/a/memory": 256000, "gpu/a/cards": 1,
					"gpu/b/memory": 178000, "gpu/b/cards": 1},
				map[string]string{
					"gpu/a": "gpu/dev2",
					"gpu/b": "gpu/dev4"},
			},
			{name: "Run1",
				res: map[string]int64{"A1": 1000, "B1": 2000},
				grpres: map[string]int64{
					"gpu/0/memory": 190000, "gpu/0/cards": 1, "gpu/0/enumType": int64(0x3)},
				expectedGrpLoc: map[string]string{"gpu/0": "gpu/dev3"},
			},
		},
	)
	translatePod(nodeInfo, podEx)
	testCnt++
	//sampleTest(pod, podEx, nodeInfo, testCnt)
	testPodAllocs(t, pod, podEx, nodeInfo, testCnt)

	// test with init resources more than running
	nodeInfo = createNodeArgs(&nodeArgs)
	// required resources
	pod, podEx = createPod("pod1", 0.58214,
		[]cont{
			{name: "Init0",
				res:            map[string]int64{"A1": 2200, "B1": 2000},
				grpres:         map[string]int64{"gpu/0/memory": 257000, "gpu/0/cards": 1},
				expectedGrpLoc: map[string]string{"gpu/0": "gpu/dev2"}}},
		[]cont{
			{"Run0",
				map[string]int64{"A1": 3000, "B1": 1000},
				map[string]int64{
					"gpu/a/memory": 256000, "gpu/a/cards": 1,
					"gpu/b/memory": 178000, "gpu/b/cards": 1},
				map[string]string{
					"gpu/a": "gpu/dev2",
					"gpu/b": "gpu/dev4"},
			},
			{name: "Run1",
				res: map[string]int64{"A1": 1000, "B1": 2000},
				grpres: map[string]int64{
					"gpu/0/memory": 190000, "gpu/0/cards": 1, "gpu/0/enumType": int64(0x3)},
				expectedGrpLoc: map[string]string{"gpu/0": "gpu/dev3"},
			},
		},
	)
	translatePod(nodeInfo, podEx)
	testCnt++
	//sampleTest(pod, podEx, nodeInfo, testCnt)
	testPodAllocs(t, pod, podEx, nodeInfo, testCnt)

	// test with just numgpu
	nodeInfo, nodeArgs = createNode("node1",
		map[string]int64{"A1": 4000, "B1": 3000},
		map[string]int64{
			"gpu/dev0/memory": 100000, "gpu/dev0/cards": 1,
			"gpu/dev1/memory": 256000, "gpu/dev1/cards": 1,
			"gpu/dev2/memory": 257000, "gpu/dev2/cards": 1,
			"gpu/dev3/memory": 192000, "gpu/dev3/cards": 1,
			"gpu/dev4/memory": 178000, "gpu/dev4/cards": 1},
	)
	pod, podEx = createPod("pod2", 0.3,
		[]cont{
			{name: "Init0",
				res:            map[string]int64{string(types.ResourceNvidiaGPU): 1},
				expectedGrpLoc: map[string]string{"gpu/0": "gpu/dev4"}}},
		[]cont{
			{name: "Run0",
				res: map[string]int64{string(types.ResourceNvidiaGPU): 2},
				expectedGrpLoc: map[string]string{
					"gpu/0": "gpu/dev4",
					"gpu/1": "gpu/dev3"},
			},
			{name: "Run1",
				res:            map[string]int64{string(types.ResourceNvidiaGPU): 1},
				expectedGrpLoc: map[string]string{"gpu/0": "gpu/dev2"},
			},
		},
	)
	translatePod(nodeInfo, podEx)
	testCnt++
	//sampleTest(pod, podEx, nodeInfo, testCnt)
	testPodAllocs(t, pod, podEx, nodeInfo, testCnt)

	// test gpu affinity group
	nodeInfo, _ = createNode("node1",
		map[string]int64{"A1": 4000, "B1": 3000},
		map[string]int64{
			"gpugrp0/group0/gpu/dev0/memory": 100000, "gpugrp0/group0/gpu/dev0/cards": 1,
			"gpugrp0/group0/gpu/dev1/memory": 256000, "gpugrp0/group0/gpu/dev1/cards": 1,
			"gpugrp0/group1/gpu/dev2/memory": 257000, "gpugrp0/group1/gpu/dev2/cards": 1,
			"gpugrp0/group2/gpu/dev3/memory": 192000, "gpugrp0/group2/gpu/dev3/cards": 1,
			"gpugrp0/group2/gpu/dev4/memory": 178000, "gpugrp0/group2/gpu/dev4/cards": 1},
	)

	// required resources
	pod, podEx = createPod("pod3", 0.9985692,
		[]cont{
			// this goes to dev4 since all gpus are in use in running state, which is fine
			{name: "Init0",
				grpres:         map[string]int64{"gpu/0/memory": 100000, "gpu/0/cards": 1},
				expectedGrpLoc: map[string]string{"gpugrp0/0/gpu/0": "gpugrp0/group0/gpu/dev1"}}},
		[]cont{
			{name: "Run0",
				grpres: map[string]int64{
					"gpugrp0/A/gpu/a/memory": 190000, "gpugrp0/A/gpu/a/cards": 1,
					"gpugrp0/A/gpu/b/memory": 178000, "gpugrp0/A/gpu/b/cards": 1},
				expectedGrpLoc: map[string]string{
					"gpugrp0/A/gpu/a": "gpugrp0/group2/gpu/dev3",
					"gpugrp0/A/gpu/b": "gpugrp0/group2/gpu/dev4"},
			},
			{name: "Run1",
				grpres: map[string]int64{
					"gpu/0/memory": 256000, "gpu/0/cards": 1},
				expectedGrpLoc: map[string]string{"gpugrp0/0/gpu/0": "gpugrp0/group1/gpu/dev2"},
			},
			{name: "Run2",
				grpres: map[string]int64{
					"gpu/0/memory": 256000, "gpu/0/cards": 1,
					"gpu/1/memory": 100000, "gpu/1/cards": 1},
				expectedGrpLoc: map[string]string{
					"gpugrp0/0/gpu/0": "gpugrp0/group0/gpu/dev1",
					"gpugrp0/1/gpu/1": "gpugrp0/group0/gpu/dev0",
				},
			},
		},
	)
	translatePod(nodeInfo, podEx)
	testCnt++
	//sampleTest(pod, podEx, nodeInfo)
	testPodAllocs(t, pod, podEx, nodeInfo, testCnt)

	// test gpu affinity group
	nodeInfo, nodeArgs = createNode("node1",
		map[string]int64{"A1": 4000, "B1": 3000},
		map[string]int64{
			"gpugrp1/0/gpugrp0/0/gpu/dev0/memory": 100000, "gpugrp1/0/gpugrp0/0/gpu/dev0/cards": 1,
			"gpugrp1/0/gpugrp0/0/gpu/dev1/memory": 256000, "gpugrp1/0/gpugrp0/0/gpu/dev1/cards": 1,
			"gpugrp1/0/gpugrp0/1/gpu/dev2/memory": 257000, "gpugrp1/0/gpugrp0/1/gpu/dev2/cards": 1,
			"gpugrp1/0/gpugrp0/1/gpu/dev3/memory": 192000, "gpugrp1/0/gpugrp0/1/gpu/dev3/cards": 1,
			"gpugrp1/1/gpugrp0/2/gpu/dev4/memory": 178000, "gpugrp1/1/gpugrp0/2/gpu/dev4/cards": 1,
			"gpugrp1/1/gpugrp0/2/gpu/dev5/memory": 100000, "gpugrp1/1/gpugrp0/2/gpu/dev5/cards": 1,
			"gpugrp1/1/gpugrp0/3/gpu/dev6/memory": 256000, "gpugrp1/1/gpugrp0/3/gpu/dev6/cards": 1,
			"gpugrp1/1/gpugrp0/3/gpu/dev7/memory": 257000, "gpugrp1/1/gpugrp0/3/gpu/dev7/cards": 1,
		},
	)
	pod, podEx = createPod("pod4", 0.125,
		[]cont{},
		[]cont{
			{name: "Run0",
				grpres: map[string]int64{
					"gpugrp0/A/gpu/a/cards": 1,
					"gpugrp0/A/gpu/b/cards": 1},
				expectedGrpLoc: map[string]string{
					"gpugrp1/0/gpugrp0/A/gpu/a": "gpugrp1/1/gpugrp0/3/gpu/dev7",
					"gpugrp1/0/gpugrp0/A/gpu/b": "gpugrp1/1/gpugrp0/3/gpu/dev6"},
			},
		},
	)
	translatePod(nodeInfo, podEx)
	testCnt++
	//sampleTest(pod, podEx, nodeInfo, testCnt)
	testPodAllocs(t, pod, podEx, nodeInfo, testCnt)

	// recreate node
	nodeInfo = createNodeArgs(&nodeArgs)
	pod, podEx = createPod("pod5", 0.375,
		[]cont{},
		[]cont{
			{name: "Run0",
				// try for 3 at lower level
				grpres: map[string]int64{
					"gpugrp1/0/gpugrp0/A/gpu/a/cards": 1,
					"gpugrp1/0/gpugrp0/B/gpu/b/cards": 1,
					"gpugrp1/0/gpugrp0/C/gpu/c/cards": 1,
					"gpugrp1/0/gpugrp0/D/gpu/d/cards": 1,
					"gpugrp0/A/gpu/a/cards":           1,
					"gpugrp0/A/gpu/b/cards":           1,
				},
				expectedGrpLoc: map[string]string{
					"gpugrp1/0/gpugrp0/A/gpu/a": "gpugrp1/1/gpugrp0/3/gpu/dev7",
					"gpugrp1/0/gpugrp0/B/gpu/b": "gpugrp1/1/gpugrp0/3/gpu/dev6",
					"gpugrp1/0/gpugrp0/C/gpu/c": "gpugrp1/1/gpugrp0/2/gpu/dev5",
					"gpugrp1/0/gpugrp0/D/gpu/d": "gpugrp1/1/gpugrp0/2/gpu/dev4",
					"gpugrp1/1/gpugrp0/A/gpu/a": "gpugrp1/0/gpugrp0/1/gpu/dev3",
					"gpugrp1/1/gpugrp0/A/gpu/b": "gpugrp1/0/gpugrp0/1/gpu/dev2",
				},
			},
		},
	)
	translatePod(nodeInfo, podEx)
	testCnt++
	//sampleTest(pod, podEx, nodeInfo, testCnt)
	testPodAllocs(t, pod, podEx, nodeInfo, testCnt)

	glog.Flush()
}
