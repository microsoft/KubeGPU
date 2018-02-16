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

func addToNode(node *types.SortedTreeNode, nodeResources types.ResourceList, partitionPrefix string, suffix string, partitionLevel int) *types.SortedTreeNode {
	childMap := make(map[string]types.ResourceList)
	re := regexp.MustCompile(`.*/` + partitionPrefix + strconv.Itoa(partitionLevel) + `/(.*?)/.*/` + suffix)
	totalLen := 0
	for resourceKey, resourceVal := range nodeResources {
		matches := re.FindStringSubmatch(string(resourceKey))
		if len(matches) >= 2 {
			subGrpKey := matches[1]
			if childMap[subGrpKey] == nil {
				childMap[subGrpKey] = make(types.ResourceList)
			}
			childMap[subGrpKey][resourceKey] = resourceVal
			totalLen++
		}
	}
	if node == nil {
		node = &types.SortedTreeNode{Val: totalLen, Child: nil}
	}
	for _, subMaps := range childMap {
		childNode := types.AddToSortedTreeNode(node, len(subMaps))
		if partitionLevel > 0 {
			addToNode(childNode, subMaps, partitionPrefix, suffix, partitionLevel-1)
		}
	}
	return node
}

type treeInfo struct {
	ListOfNodes map[string]bool
	TreeScore   float64
}

var nodeCacheMap = make(map[*types.SortedTreeNode]treeInfo)
var nodeLocationMap = make(map[string]*types.SortedTreeNode)

func removeNodeFromCache(nodeName string, nodeLocation *types.SortedTreeNode) {
	if nodeLocation != nil {
		delete(nodeCacheMap[nodeLocation].ListOfNodes, nodeName)
		if len(nodeCacheMap[nodeLocation].ListOfNodes) == 0 {
			delete(nodeCacheMap, nodeLocation)
		}
	}
}

func computeTreeScoreAtLevel(node *types.SortedTreeNode, level int, numChild int) float64 {
	score := float64(node.Val * level * numChild)
	for _, child := range node.Child {
		score += computeTreeScoreAtLevel(child, level+1, len(node.Child))
	}
	return score
}

func computeTreeScore(node *types.SortedTreeNode) float64 {
	return computeTreeScoreAtLevel(node, 0, len(node.Child))
}

func AddResourcesToNodeTreeCache(nodeName string, nodeResources types.ResourceList) {
	// get tree representation of node gpu resources
	node := addToNode(nil, nodeResources, "gpugrp", "cards", 1) // gpugrp1 and gpugrp0
	// see if resource has changed
	nodeLocation := nodeLocationMap[nodeName]
	if types.CompareTreeNode(node, nodeLocation) {
		return
	}
	// remove node from current location
	removeNodeFromCache(nodeName, nodeLocation)
	// check if matches to some other node in cache
	found := false
	for cacheKey := range nodeCacheMap {
		if types.CompareTreeNode(node, cacheKey) {
			nodeCacheMap[cacheKey].ListOfNodes[nodeName] = true
			nodeLocation = cacheKey
			found = true
			break
		}
	}
	// if not found add new to cache
	if !found {
		treeScore := computeTreeScore(node)
		treeInfo := treeInfo{ListOfNodes: make(map[string]bool), TreeScore: treeScore}
		nodeLocation = node
		nodeCacheMap[node] = treeInfo
	}
	nodeLocationMap[nodeName] = nodeLocation
}

func RemoveNodeFromNodeTreeCache(nodeName string) {
	nodeLocation := nodeLocationMap[nodeName]
	removeNodeFromCache(nodeName, nodeLocation)
	delete(nodeLocationMap, nodeName)
}

func findBestTreeInCache(num int) *types.SortedTreeNode {
	var bestTree *types.SortedTreeNode
	bestScore := 0.0
	for tree, treeInfo := range nodeCacheMap {
		if tree.Val >= num {
			if treeInfo.TreeScore > bestScore {
				bestTree = tree
				bestScore = treeInfo.TreeScore
			}
		}
	}
	return bestTree
}

func assignGPUs(node *types.SortedTreeNode, prefix string, resourceGrp string, resource string, suffix string, level int, numLeft *int) types.ResourceList {
	resList := make(types.ResourceList)
	if level == 0 {
		toTake := node.Val
		if *numLeft <= node.Val {
			toTake = *numLeft
		}
		for i := 0; i < toTake; i++ {
			resList[types.ResourceName(prefix+"/"+resource+"/"+strconv.Itoa(i)+"/"+suffix)] = 1
		}
		*numLeft = *numLeft - toTake
	} else {
		for i, child := range node.Child {
			newPrefix := prefix + strconv.Itoa(level-1) + "/" + strconv.Itoa(i) + "/" + resourceGrp
			resListChild := assignGPUs(child, newPrefix, resourceGrp, resource, suffix, level-1, numLeft)
			for resKey, resVal := range resListChild {
				resList[resKey] = resVal
			}
		}
	}
	return resList
}

func translateToTree(node *types.SortedTreeNode, cont *types.ContainerInfo) {
	// remove all GPU topology requests
	re := regexp.MustCompile(`.*/gpu/.*`)
	newRequests := make(types.ResourceList)
	for reqKey, reqVal := range cont.DevRequests {
		matches := re.FindStringSubmatch(string(reqKey))
		if len(matches) == 0 {
			newRequests[reqKey] = reqVal
		}
	}
	cont.DevRequests = newRequests
	// append requests
	numGPUs := int(cont.Requests[types.ResourceGPU])
	assignGPUs(node, types.DeviceGroupPrefix+"/gpugrp", "gpugrp", "gpu", "cards", 2, &numGPUs)
}

// find total GPUs needed
func ConvertToBestGPURequests(podInfo *types.PodInfo) {
	numGPUs := int64(0)
	for _, cont := range podInfo.RunningContainers {
		numGPUs += cont.Requests[types.ResourceGPU]
	}
	for _, cont := range podInfo.RunningContainers {
		if cont.Requests[types.ResourceGPU] > numGPUs {
			numGPUs = cont.Requests[types.ResourceGPU]
		}
	}
	bestTree := findBestTreeInCache(int(numGPUs))
	// now translate requests to best tree
	for _, cont := range podInfo.RunningContainers {
		translateToTree(bestTree, &cont)
	}
	for _, cont := range podInfo.InitContainers {
		translateToTree(bestTree, &cont)
	}
}
