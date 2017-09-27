package types

import "k8s.io/kubernetes/pkg/api/v1"

type ResourceName string

// ResourceLocation is a set of (resource name, resource location on node) pairs.
type ResourceLocation map[ResourceName]ResourceName
type ResourceList map[ResourceName]int64

const (
	// NVIDIA GPU, in devices. Alpha, might change: although fractional and allowing values >1, only one whole device per node is assigned.
	ResourceNvidiaGPU ResourceName = "alpha.kubernetes.io/nvidia-gpu"
	// Namespace prefix for group resources (alpha).
	ResourceGroupPrefix = "alpha.kubernetes.io/group-resource"
)

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
	AllocateDevices(*v1.Pod, *v1.Container) ([]Volume, []string, error)
}
