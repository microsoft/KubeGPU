package resource

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Microsoft/KubeGPU/types"
	"github.com/Microsoft/KubeGPU/utils"
	"github.com/golang/glog"
)

// IsGroupResourceName returns true if the resource name has the group resource prefix
func IsGroupResourceName(name types.ResourceName) bool {
	return strings.HasPrefix(string(name), types.DeviceGroupPrefix)
}

// IsEnumResource returns true if resource name is an "enum" resource
func IsEnumResource(res types.ResourceName) bool {
	re := regexp.MustCompile(`\S*/(\S*)`)
	matches := re.FindStringSubmatch(string(res))
	if len(matches) >= 2 {
		return strings.HasPrefix(strings.ToLower(matches[1]), "enum")
	}
	return false
}

func AddGroupResource(list types.ResourceList, key string, val int64) {
	list[types.ResourceName(types.DeviceGroupPrefix+"/"+key)] = val
}

// Resource translation to max level specified in nodeInfo
// TranslateResource translates resources to next level
func TranslateResource(nodeResources types.ResourceList, containerRequests types.ResourceList,
	thisStage string, nextStage string) (bool, types.ResourceList) {

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
		return false, containerRequests
	}

	// find max existing index
	maxGroupIndex := -1
	for res := range containerRequests {
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
	newList := make(types.ResourceList)
	groupMap := make(map[string]string)
	// ordered addition to make sure groupIndex is deterministic based on order
	reqKeys := utils.SortedStringKeys(containerRequests)
	resourceModified := false
	for _, resKey := range reqKeys {
		val := containerRequests[types.ResourceName(resKey)]
		matches := re.FindStringSubmatch(string(resKey))
		newResKey := types.ResourceName(resKey)
		if len(matches) == 0 { // does not qualify as thisStage resource
			matches = re2.FindStringSubmatch(string(resKey))
			if len(matches) >= 5 { // does qualify as next stage resource
				mapGrp, available := groupMap[matches[3]]
				if !available {
					groupMap[matches[3]] = strconv.Itoa(groupIndex)
					groupIndex++
					mapGrp = groupMap[matches[3]]
				}
				newResKey = types.ResourceName(matches[1] + thisStage + "/" + mapGrp + "/" + nextStage + "/" + matches[2])
				glog.V(7).Infof("Writing new resource %v - old %v", newResKey, resKey)
				resourceModified = true
			}
		}
		newList[newResKey] = val
	}

	return resourceModified, newList
}

func PrecheckedResource(constraint types.ResourceName) bool {
	return !IsGroupResourceName(constraint)
}

// InsufficientResourceError is an error type that indicates what kind of resource limit is
// hit and caused the unfitting failure.
type InsufficientResourceError struct {
	// resourceName is the name of the resource that is insufficient
	ResourceName types.ResourceName
	requested    int64
	used         int64
	capacity     int64
}

func NewInsufficientResourceError(resourceName types.ResourceName, requested, used, capacity int64) *InsufficientResourceError {
	return &InsufficientResourceError{
		ResourceName: resourceName,
		requested:    requested,
		used:         used,
		capacity:     capacity,
	}
}

func (e *InsufficientResourceError) GetReason() string {
	return fmt.Sprintf("Insufficient %v", e.ResourceName)
}

func (e *InsufficientResourceError) GetInfo() (types.ResourceName, int64, int64, int64) {
	return e.ResourceName, e.requested, e.used, e.capacity
}
