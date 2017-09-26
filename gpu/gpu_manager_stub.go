package gpu

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/golang/glog"

	v1 "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

type gpuManagerStub struct{}

func (gms *gpuManagerStub) Start() error {
	return nil
}

func (gms *gpuManagerStub) Capacity() v1.ResourceList {
	return nil
}

// AllocateGPU Returns volumename, volumedriver, devices
func (gms *gpuManagerStub) AllocateGPU(_ *v1.Pod, _ *v1.Container) (volumeName string, volumeDriver string, devices []string, err error) {
	devices = nil
	err = fmt.Errorf("GPUs are not supported")
	return volumeName, volumeDriver, devices, err
}

func NewGPUManagerStub() GPUManager {
	return &gpuManagerStub{}
}

// TranslateGPUResources translates GPU resources to max level
func TranslateGPUResources(nodeInfo *schedulercache.NodeInfo, container *v1.Container) error {
	requests := container.Resources.Requests

	// First stage translation, translate # of cards to simple GPU resources - extra stage
	re := regexp.MustCompile(v1.ResourceGroupPrefix + `.*/gpu/(.*?)/cards`)

	neededGPUQ := requests[v1.ResourceNvidiaGPU]
	neededGPUs := neededGPUQ.Value()
	haveGPUs := 0
	maxGPUIndex := -1
	for res := range container.Resources.Requests {
		matches := re.FindStringSubmatch(string(res))
		if len(matches) >= 2 {
			haveGPUs++
			gpuIndex, err := strconv.Atoi(matches[1])
			if err == nil {
				if gpuIndex > maxGPUIndex {
					maxGPUIndex = gpuIndex
				}
			}
		}
	}
	resourceModified := false
	diffGPU := int(neededGPUs - int64(haveGPUs))
	for i := 0; i < diffGPU; i++ {
		gpuIndex := maxGPUIndex + i + 1
		v1.AddGroupResource(requests, "gpu/"+strconv.Itoa(gpuIndex)+"/cards", 1)
		resourceModified = true
	}

	// perform 2nd stage translation if needed
	resourceModified = resourceModified ||
		v1.TranslateResource(nodeInfo.AllocatableResource().OpaqueIntResources, container, "gpugrp0", "gpu")
	// perform 3rd stage translation if needed
	resourceModified = resourceModified ||
		v1.TranslateResource(nodeInfo.AllocatableResource().OpaqueIntResources, container, "gpugrp1", "gpugrp0")

	if resourceModified {
		glog.V(3).Infoln("New Resources", container.Resources.Requests)
	}

	return nil
}
