/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This package re-written by Sanjeev Mehrotra to use nvidia-docker-plugin
package nvidia

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/golang/glog"

	"strconv"

	v1 "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/kubelet/dockertools"
	"k8s.io/kubernetes/pkg/kubelet/gpu"
)

type memoryInfo struct {
	Global int64 `json:"Global"`
}

type pciInfo struct {
	BusID     string `json:"BusID"`
	Bandwidth int64  `json:"Bandwidth"`
}

type topologyInfo struct {
	BusID string `json:"BusID"`
	Link  int32  `json:"Link"`
}

type gpuInfo struct {
	ID       string         `json:"UUID"`
	Model    string         `json:"Model"`
	Path     string         `json:"Path"`
	Memory   memoryInfo     `json:"Memory"`
	PCI      pciInfo        `json:"PCI"`
	Topology []topologyInfo `json:"Topology"`
	Found    bool           `json:"-"`
	Index    int            `json:"-"`
	InUse    bool           `json:"-"`
	TopoDone bool           `json:"-"`
	Name     string         `json:"-"`
}

type versionInfo struct {
	Driver string `json:"Driver"`
	CUDA   string `json:"CUDA"`
}
type gpusInfo struct {
	Version versionInfo `json:"Version"`
	Gpus    []gpuInfo   `json:"Devices"`
}

// nvidiaGPUManager manages nvidia gpu devices.
type nvidiaGPUManager struct {
	sync.Mutex
	np        NvidiaPlugin
	gpus      map[string]gpuInfo
	pathToID  map[string]string
	busIDToID map[string]string
	indexToID []string
	numGpus   int
}

// NewNvidiaGPUManager returns a GPUManager that manages local Nvidia GPUs.
// TODO: Migrate to use pod level cgroups and make it generic to all runtimes.
func NewNvidiaGPUManager(dockerClient dockertools.DockerInterface) (gpu.GPUManager, error) {
	if dockerClient == nil {
		return nil, fmt.Errorf("invalid docker client specified")
	}
	plugin := &NvidiaDockerPlugin{}
	return &nvidiaGPUManager{gpus: make(map[string]gpuInfo), np: plugin}, nil
}

func arrayContains(arr []int32, val int32) bool {
	for _, elem := range arr {
		if val == elem {
			return true
		}
	}
	return false
}

// topology discovery
func (ngm *nvidiaGPUManager) topologyDiscovery(links []int32, level int32) {
	for id, copy := range ngm.gpus {
		copy.TopoDone = false
		ngm.gpus[id] = copy
	}
	linkID := 0
	for _, id := range ngm.indexToID {
		copy := ngm.gpus[id]
		if !ngm.gpus[id].Found || ngm.gpus[id].TopoDone {
			continue
		}
		prefix := "gpugrp" + strconv.Itoa(int(level)) + "/" + strconv.Itoa(int(linkID))
		linkID++
		copy.Name = prefix + "/" + ngm.gpus[id].Name
		copy.TopoDone = true
		ngm.gpus[id] = copy
		for _, topolink := range ngm.gpus[id].Topology {
			if arrayContains(links, topolink.Link) {
				idOnLink := ngm.busIDToID[topolink.BusID]
				gpuOnLink := ngm.gpus[idOnLink]
				if gpuOnLink.Found {
					gpuOnLink.Name = prefix + "/" + gpuOnLink.Name
					gpuOnLink.TopoDone = true
					ngm.gpus[idOnLink] = gpuOnLink
				}
			}
		}
	}
}

// Initialize the GPU devices
func (ngm *nvidiaGPUManager) UpdateGPUInfo() error {
	ngm.Lock()
	defer ngm.Unlock()

	np := ngm.np
	body, err := np.GetGPUInfo()
	if err != nil {
		return err
	}
	var gpus gpusInfo
	if err := json.Unmarshal(body, &gpus); err != nil {
		return err
	}
	// convert certain resources to correct units, such as memory and Bandwidth
	for i := range gpus.Gpus {
		gpus.Gpus[i].Memory.Global *= int64(1024) * int64(1024) // in units of MiB
		gpus.Gpus[i].PCI.Bandwidth *= int64(1000) * int64(1000) // in units of MB
	}

	for key := range ngm.gpus {
		copy := ngm.gpus[key]
		copy.Found = false
		ngm.gpus[key] = copy
	}
	// go over found GPUs and reassign
	ngm.pathToID = make(map[string]string)
	ngm.busIDToID = make(map[string]string)
	ngm.indexToID = make([]string, len(gpus.Gpus))
	for index, gpuFound := range gpus.Gpus {
		gpu, available := ngm.gpus[gpuFound.ID]
		if available {
			gpuFound.InUse = gpu.InUse
		}
		gpuFound.Found = true
		gpuFound.Index = index
		gpuFound.Name = "gpu/" + gpuFound.ID
		ngm.gpus[gpuFound.ID] = gpuFound
		ngm.pathToID[gpuFound.Path] = gpuFound.ID
		ngm.busIDToID[gpuFound.PCI.BusID] = gpuFound.ID
		ngm.indexToID[index] = gpuFound.ID
	}
	// set numGpus to number found -- not to len(ngm.gpus)
	ngm.numGpus = len(gpus.Gpus) // if ngm.numGpus <> len(ngm.gpus), then some gpus have gone missing

	// perform topology discovery to reassign name
	// more information regarding various "link types" can be found in https://github.com/nvidia/nvidia-docker/blob/master/src/nvml/nvml.go
	// const (
	// 	P2PLinkUnknown P2PLinkType = iota
	// 	P2PLinkCrossCPU
	// 	P2PLinkSameCPU
	// 	P2PLinkHostBridge
	// 	P2PLinkMultiSwitch
	// 	P2PLinkSingleSwitch
	// 	P2PLinkSameBoard
	// )
	// For topology levels, see https://docs.nvidia.com/deploy/pdf/NVML_API_Reference_Guide.pdf
	// NVML_TOPOLOGY_INTERNAL = 0 (translate to level 6)
	// NVML_TOPOLOGY_SINGLE = 10 (level 5)
	// NVML_TOPOLOGY_MULTIPLE = 20 (level 4)
	// NVML_TOPOLOGY_HOSTBRIDGE = 30 (level 3)
	// NVML_TOPOLOGY_CPU = 40 (level 2)
	// NVML_TOPOLOGY_SYSTEM = 50 (level 1)
	//
	// can have more levels if desired, but perhaps two levels are sufficient
	// link "5" discovery - put 6, 5, 4 in first group
	ngm.topologyDiscovery([]int32{6, 5, 4}, 0)
	// link "5, 3"" discovery - put all in higher group
	ngm.topologyDiscovery([]int32{6, 5, 4, 3, 2, 1}, 1)

	return nil
}

func (ngm *nvidiaGPUManager) Start() error {
	_ = ngm.UpdateGPUInfo() // ignore error in updating, gpus stay at zero
	return nil
}

// Get how many GPU cards we have.
func (ngm *nvidiaGPUManager) Capacity() v1.ResourceList {
	err := ngm.UpdateGPUInfo() // don't care about error, ignore it
	resourceList := make(v1.ResourceList)
	if err != nil {
		ngm.numGpus = 0
		return resourceList // empty resource list
	}
	// first add # of gpus to resource list
	gpus := resource.NewQuantity(int64(ngm.numGpus), resource.DecimalSI)
	resourceList[v1.ResourceNvidiaGPU] = *gpus
	for _, val := range ngm.gpus {
		if val.Found { // if currently discovered
			v1.AddGroupResource(resourceList, val.Name+"/memory", val.Memory.Global)
			v1.AddGroupResource(resourceList, val.Name+"/cards", int64(1))
		}
	}
	return resourceList
}

// AllocateGPU returns VolumeName, VolumeDriver, and list of Devices to use
func (ngm *nvidiaGPUManager) AllocateGPU(pod *v1.Pod, container *v1.Container) (string, string, []string, error) {
	gpuList := []string{}
	volumeDriver := ""
	volumeName := ""
	ngm.Lock()
	defer ngm.Unlock()

	//re := regexp.MustCompile(v1.ResourceGroupPrefix + "/gpu/" + `(.*?)/cards`)
	re := regexp.MustCompile(v1.ResourceGroupPrefix + "/gpugrp1/.*/gpugrp0/.*/gpu/" + `(.*?)/cards`)

	devices := []int{}
	for _, res := range container.Resources.AllocateFrom {
		glog.V(4).Infof("PodName: %v -- searching for device UID: %v", pod.Name, res)
		matches := re.FindStringSubmatch(string(res))
		if len(matches) >= 2 {
			id := matches[1]
			devices = append(devices, ngm.gpus[id].Index)
			glog.V(4).Infof("PodName: %v -- device index: %v", pod.Name, ngm.gpus[id].Index)
			if ngm.gpus[id].Found {
				gpuList = append(gpuList, ngm.gpus[id].Path)
				glog.V(3).Infof("PodName: %v -- UID: %v device path: %v", pod.Name, res, ngm.gpus[id].Path)
			}
		}
	}
	np := ngm.np
	body, err := np.GetGPUCommandLine(devices)
	glog.V(3).Infof("PodName: %v Command line from plugin: %v", pod.Name, string(body))
	if err != nil {
		return "", "", nil, err
	}

	re = regexp.MustCompile(`(.*?)=(.*)`)
	//fmt.Println("body:", body)
	tokens := strings.Split(string(body), " ")
	//fmt.Println("tokens:", tokens)
	for _, token := range tokens {
		matches := re.FindStringSubmatch(token)
		if len(matches) == 3 {
			key := matches[1]
			val := matches[2]
			//fmt.Printf("Token %v Match key %v Val %v\n", token, key, val)
			if key == `--device` {
				_, available := ngm.pathToID[val] // val is path in case of device
				if !available {
					gpuList = append(gpuList, val) // for other devices, e.g. /dev/nvidiactl, /dev/nvidia-uvm, /dev/nvidia-uvm-tools
				}
			} else if key == `--volume-driver` {
				volumeDriver = val
			} else if key == `--volume` {
				volumeName = val
			}
		}
	}

	return volumeName, volumeDriver, gpuList, nil
}
