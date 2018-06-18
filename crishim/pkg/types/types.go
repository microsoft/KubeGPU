package types

import (
	"github.com/Microsoft/KubeGPU/types"
)

type Volume struct {
	Name   string
	Driver string
}

// Device is a device to use
type Device interface {
	// New creates the device and initializes it
	New() error
	// Start logically initializes the device
	Start() error
	// UpdateNodeInfo - updates a node info structure by writing capacity, allocatable, used, scorer
	UpdateNodeInfo(*types.NodeInfo) error
	// Allocate attempst to allocate the devices
	// Returns list of (VolumeName, VolumeDriver), and list of Devices to use
	// Returns an error on failure.
	Allocate(*types.PodInfo, *types.ContainerInfo) ([]Volume, []string, error)
	// GetName returns the name of a device
	GetName() string
}
