package gpu

import (
	"regexp"
	"strconv"

	"github.com/Microsoft/KubeGPU/gpuextension/grpalloc/resource"
	"github.com/Microsoft/KubeGPU/types"
	"github.com/golang/glog"
)

// TranslateGPUResources translates GPU resources to max level advertised by the node
func TranslateGPUResources(neededGPUs int64, nodeResources types.ResourceList, containerRequests types.ResourceList) types.ResourceList {
	// First stage translation, translate # of cards to simple GPU resources - extra stage
	re := regexp.MustCompile(types.DeviceGroupPrefix + `.*/gpu/(.*?)/cards`)

	needTranslation := false
	for res := range nodeResources {
		matches := re.FindStringSubmatch(string(res))
		if len(matches) >= 2 {
			needTranslation = true
			break
		}
	}
	if !needTranslation {
		return containerRequests
	}

	haveGPUs := 0
	maxGPUIndex := -1
	for res := range containerRequests {
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
		resource.AddGroupResource(containerRequests, "gpu/"+strconv.Itoa(gpuIndex)+"/cards", 1)
		resourceModified = true
	}

	// perform 2nd stage translation if needed
	resourceModified1, containerRequests := resource.TranslateResource(nodeResources, containerRequests, "gpugrp0", "gpu")
	resourceModified = resourceModified || resourceModified1
	// perform 3rd stage translation if needed
	resourceModified1, containerRequests = resource.TranslateResource(nodeResources, containerRequests, "gpugrp1", "gpugrp0")
	resourceModified = resourceModified || resourceModified1

	if resourceModified {
		glog.V(3).Infoln("New Resources", containerRequests)
	}

	return containerRequests
}
