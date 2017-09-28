package types

import (
	"reflect"
	"sort"
)

const (
	// NVIDIA GPU, in devices. Alpha, might change: although fractional and allowing values >1, only one whole device per node is assigned.
	ResourceNvidiaGPU ResourceName = "alpha.kubernetes.io/nvidia-gpu"
	// Namespace prefix for group resources (alpha).
	ResourceGroupPrefix = "alpha.kubernetes.io/group-resource"
)

type ResourceName string

// ResourceLocation is a set of (resource name, resource location on node) pairs.
type ResourceLocation map[ResourceName]ResourceName
type ResourceList map[ResourceName]int64

type ContainerInfo struct {
	Requests     ResourceList
	AllocateFrom ResourceLocation
}

type PodInfo struct {
	Name string
	// requests
	InitContainer    []ContainerInfo
	RunningContainer []ContainerInfo
}

type NodeInfo struct {
	Name        string
	Capacity    []ResourceList
	Allocatable []ResourceList // capacity minus reserverd
	Used        []ResourceList // being used by pods, must be less than allocatable
}

type Volume struct {
	Name   string
	Driver string
}

// DeviceManager manages devices
type DeviceManager interface {
	// Start logically initializes the device
	Start() error
	// Capacity returns the capacity of resources
	Capacity() ResourceList
	// Allocate attempst to allocate the devices
	// Returns list of (VolumeName, VolumeDriver), and list of Devices to use
	// Returns an error on failure.
	AllocateDevices(*PodInfo, *ContainerInfo) ([]Volume, []string, error)
}

// sorted string keys
func SortedStringKeys(x interface{}) []string {
	t := reflect.TypeOf(x)
	keys := []string{}
	if t.Kind() == reflect.Map {
		mv := reflect.ValueOf(x)
		keysV := mv.MapKeys()
		for _, val := range keysV {
			keys = append(keys, val.String())
		}
		sort.Strings(keys)
		return keys
	}
	panic("Not a map")
}

const (
	DefaultScorer = iota // 0
	LeftOverScorer
	EnumLeftOverScorer
)
