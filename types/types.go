package types

import "k8s.io/kubernetes/pkg/api/v1"

type ResourceList map[string]int64

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
	AllocateGPU(*v1.Pod, *v1.Container) ([]Volume, []string, error)
}

