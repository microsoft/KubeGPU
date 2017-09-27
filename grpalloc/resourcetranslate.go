package grpalloc

import (
	"regexp"
	"strconv"

	"github.com/MSRCCS/grpalloc/types"
	"github.com/golang/glog"
)

func AddGroupResource(list types.ResourceList, key string, val int64) {
	list[ResourceName(ResourceGroupPrefix+"/"+key)] = val
}

// Resource translation to max level specified in nodeInfo
// TranslateResource translates resources to next level
func TranslateResource(nodeResources map[ResourceName]int64, container *Container,
	thisStage string, nextStage string) bool {

	// see if translation needed
	translationNeeded := false
	re := regexp.MustCompile(`.*/` + thisStage + `/(.*?)/` + nextStage + `(.*)`)
	for key := range nodeResources {
		matches := re.FindStringSubmatch(string(key))
		if (len(matches)) >= 2 {
			translationNeeded = true
			break
		}
	}
	if !translationNeeded {
		return false
	}

	// find max existing index
	maxGroupIndex := -1
	for res := range container.Resources.Requests {
		matches := re.FindStringSubmatch(string(res))
		if len(matches) >= 2 {
			groupIndex, err := strconv.Atoi(matches[1])
			if err == nil {
				if groupIndex > maxGroupIndex {
					maxGroupIndex = groupIndex
				}
			}
		}
	}

	groupIndex := maxGroupIndex + 1
	re2 := regexp.MustCompile(`(.*?/)` + nextStage + `/((.*?)/(.*))`)
	newList := make(ResourceList)
	groupMap := make(map[string]string)
	// ordered addition to make sure groupIndex is deterministic based on order
	reqKeys := SortedStringKeys(container.Resources.Requests)
	resourceModified := false
	for _, resKey := range reqKeys {
		val := container.Resources.Requests[ResourceName(resKey)]
		matches := re.FindStringSubmatch(string(resKey))
		newResKey := ResourceName(resKey)
		if len(matches) == 0 { // does not qualify as thisStage resource
			matches = re2.FindStringSubmatch(string(resKey))
			if len(matches) >= 5 { // does qualify as next stage resource
				mapGrp, available := groupMap[matches[3]]
				if !available {
					groupMap[matches[3]] = strconv.Itoa(groupIndex)
					groupIndex++
					mapGrp = groupMap[matches[3]]
				}
				newResKey = ResourceName(matches[1] + thisStage + "/" + mapGrp + "/" + nextStage + "/" + matches[2])
				glog.V(7).Infof("Writing new resource %v - old %v", newResKey, resKey)
				resourceModified = true
			}
		}
		newList[newResKey] = val
	}
	container.Resources.Requests = newList
	return resourceModified
}
