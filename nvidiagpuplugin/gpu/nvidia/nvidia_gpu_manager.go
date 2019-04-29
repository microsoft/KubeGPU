package nvidia

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	devtypes "github.com/Microsoft/KubeDevice-API/pkg/device"
	"github.com/Microsoft/KubeDevice-API/pkg/types"
	"github.com/Microsoft/KubeDevice-API/pkg/utils"
	gputypes "github.com/Microsoft/KubeGPU/gpuplugintypes"
	"github.com/Microsoft/KubeGPU/nvidiagpuplugin/gpu/nvgputypes"

	"strconv"
)

// NvidiaGPUManager manages nvidia gpu devices.
type NvidiaGPUManager struct {
	sync.Mutex
	np              NvidiaPlugin
	gpus            map[string]nvgputypes.GpuInfo
	pathToID        map[string]string
	busIDToID       map[string]string
	indexToID       []string
	numGpus         int
	useNVML         bool
	GpusInfo        *nvgputypes.GpusInfo
	nvmlLastGetTime time.Time
}

// NewNvidiaGPUManager returns a GPUManager that manages local Nvidia GPUs.
// TODO: Migrate to use pod level cgroups and make it generic to all runtimes.
func NewNvidiaGPUManager() (devtypes.Device, error) {
	ngm := &NvidiaGPUManager{useNVML: true}
	return ngm, ngm.New()
}

func (ngm *NvidiaGPUManager) New() error {
	ngm.gpus = make(map[string]nvgputypes.GpuInfo)
	if !ngm.useNVML {
		plugin := &NvidiaDockerPlugin{}
		ngm.np = plugin
	}
	return nil
}

func arrayContains(arr []int32, val int32) bool {
	for _, elem := range arr {
		if val == elem {
			return true
		}
	}
	return false
}

func (ngm *NvidiaGPUManager) GetName() string {
	return "nvidiagpu"
}

// topology discovery
func (ngm *NvidiaGPUManager) topologyDiscovery(links []int32, level int32) {
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
func (ngm *NvidiaGPUManager) UpdateGPUInfo() error {
	ngm.Lock()
	defer ngm.Unlock()

	var gpus nvgputypes.GpusInfo
	if !ngm.useNVML {
		np := ngm.np
		body, err := np.GetGPUInfo()
		if err != nil {
			return err
		}
		utils.Logf(5, "GetGPUInfo returns %s", string(body))
		if err := json.Unmarshal(body, &gpus); err != nil {
			return err
		}
	} else {
		timeElapsed := time.Now().Sub(ngm.nvmlLastGetTime)
		if ngm.GpusInfo == nil || timeElapsed.Seconds() > 5*60.0 {
			gpuPtr, err := nvgputypes.GetDevices()
			if err != nil {
				return err
			}
			gpus = *gpuPtr
			ngm.nvmlLastGetTime = time.Now()
			ngm.GpusInfo = gpuPtr
		} else {
			gpus = *ngm.GpusInfo
		}
	}
	utils.Logf(5, "GPUInfo: %+v", gpus)
	// convert certain resources to correct units, such as memory and Bandwidth
	if !ngm.useNVML {
		for i := range gpus.Gpus {
			gpus.Gpus[i].Memory.Global *= int64(1024) * int64(1024) // in units of MiB
			gpus.Gpus[i].PCI.Bandwidth *= int64(1000) * int64(1000) // in units of MB
		}
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

func (ngm *NvidiaGPUManager) Start() error {
	_ = ngm.UpdateGPUInfo() // ignore error in updating, gpus stay at zero
	return nil
}

// Get how many GPU cards we have.
func (ngm *NvidiaGPUManager) UpdateNodeInfo(nodeInfo *types.NodeInfo) error {
	err := ngm.UpdateGPUInfo() // don't care about error, ignore it
	if err != nil {
		utils.Logf(0, "UpdateGPUInfo encounters error %+v, setting GPUs to zero", err)
		ngm.numGpus = 0
		return err
	}
	utils.Logf(4, "NumGPUs found = %d", ngm.numGpus)
	numGpus := int64(len(ngm.gpus))
	nodeInfo.Capacity[gputypes.ResourceGPU] = numGpus
	nodeInfo.Allocatable[gputypes.ResourceGPU] = numGpus
	nodeInfo.KubeCap[gputypes.ResourceGPU] = numGpus
	nodeInfo.KubeAlloc[gputypes.ResourceGPU] = numGpus
	for _, val := range ngm.gpus {
		if val.Found { // if currently discovered
			types.AddGroupResource(nodeInfo.Capacity, val.Name+"/memory", val.Memory.Global)
			types.AddGroupResource(nodeInfo.Allocatable, val.Name+"/memory", val.Memory.Global)
			types.AddGroupResource(nodeInfo.Capacity, val.Name+"/cards", int64(1))
			types.AddGroupResource(nodeInfo.Allocatable, val.Name+"/cards", int64(1))
		}
	}
	return nil
}

// For use with nvidia runtime (nvidia docker2)
func (ngm *NvidiaGPUManager) Allocate(pod *types.PodInfo, container *types.ContainerInfo) ([]devtypes.Mount, []string, map[string]string, error) {
	gpuList := []string{}
	ngm.Lock()
	defer ngm.Unlock()

	if container.AllocateFrom == nil || 0 == len(container.AllocateFrom) {
		return nil, nil, nil, nil
	}

	re := regexp.MustCompile(types.DeviceGroupPrefix + "/gpugrp1/.*/gpugrp0/.*/gpu/" + `(.*?)/cards`)

	for _, res := range container.AllocateFrom {
		utils.Logf(4, "PodName: %v -- searching for device UID: %v", pod.Name, res)
		matches := re.FindStringSubmatch(string(res))
		if len(matches) >= 2 {
			id := matches[1]
			gpuList = append(gpuList, id)
		}
	}

	env := map[string]string{
		"NVIDIA_VISIBLE_DEVICES": strings.Join(gpuList, ","),
	}

	return nil, nil, env, nil
}

// AllocateGPU returns MountName, MountDriver, and list of Devices to use
func (ngm *NvidiaGPUManager) AllocateOld(pod *types.PodInfo, container *types.ContainerInfo) ([]devtypes.Mount, []string, map[string]string, error) {
	gpuList := []string{}
	// volumeDriver := ""
	// volumeName := ""
	ngm.Lock()
	defer ngm.Unlock()

	if container.AllocateFrom == nil || 0 == len(container.AllocateFrom) {
		return nil, nil, nil, nil
	}

	//re := regexp.MustCompile(types.DeviceGroupPrefix + "/gpu/" + `(.*?)/cards`)
	re := regexp.MustCompile(types.DeviceGroupPrefix + "/gpugrp1/.*/gpugrp0/.*/gpu/" + `(.*?)/cards`)

	devices := []int{}
	for _, res := range container.AllocateFrom {
		utils.Logf(4, "PodName: %v -- searching for device UID: %v", pod.Name, res)
		matches := re.FindStringSubmatch(string(res))
		if len(matches) >= 2 {
			id := matches[1]
			devices = append(devices, ngm.gpus[id].Index)
			utils.Logf(4, "PodName: %v -- device index: %v", pod.Name, ngm.gpus[id].Index)
			if ngm.gpus[id].Found {
				gpuList = append(gpuList, ngm.gpus[id].Path)
				utils.Logf(4, "PodName: %v -- UID: %v device path: %v", pod.Name, res, ngm.gpus[id].Path)
			}
		}
	}
	np := ngm.np
	body, err := np.GetGPUCommandLine(devices)
	utils.Logf(4, "PodName: %v Command line from plugin: %v", pod.Name, string(body))
	if err != nil {
		return nil, nil, nil, err
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
			}
			// } else if key == `--volume-driver` {
			// 	volumeDriver = val
			// } else if key == `--volume` {
			// 	volumeName = val
			// }
		}
	}

	return nil, gpuList, nil, nil
}

// numAllocateFrom := len(cont.AllocateFrom) // may be zero from old scheduler
// nvidiaFullpathRE := regexp.MustCompile(`^/dev/nvidia[0-9]*$`)
// var newDevices []*runtimeapi.Device
// // first remove any existing nvidia devices
// numRequestedGPU := 0
// for _, oldDevice := range config.Devices {
// 	isNvidiaDevice := false
// 	if oldDevice.HostPath == "/dev/nvidiactl" ||
// 		oldDevice.HostPath == "/dev/nvidia-uvm" ||
// 		oldDevice.HostPath == "/dev/nvidia-uvm-tools" {
// 		isNvidiaDevice = true
// 	}
// 	if nvidiaFullpathRE.MatchString(oldDevice.HostPath) {
// 		isNvidiaDevice = true
// 		numRequestedGPU++
// 	}
// 	if !isNvidiaDevice || 0 == numAllocateFrom {
// 		newDevices = append(newDevices, oldDevice)
// 	}
// }
// if (numAllocateFrom > 0) && (numRequestedGPU > 0) && (numAllocateFrom != numRequestedGPU) {
// 	return fmt.Errorf("Number of AllocateFrom is different than number of requested GPUs")
// }
// utils.Logf(3, "Modified devices: %v", newDevices)
