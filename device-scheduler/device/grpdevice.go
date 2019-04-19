package device

import (
	"fmt"

	"github.com/Microsoft/KubeGPU/device-scheduler/grpalloc"
	sctypes "github.com/Microsoft/KubeGPU/device-scheduler/types"
	gputypes "github.com/Microsoft/KubeGPU/plugins/gpuplugintypes"
	"github.com/Microsoft/KubeGPU/types"
	"github.com/golang/glog"
)

type GrpDevice struct {
}

func (d *GrpDevice) AddNode(nodeName string, nodeInfo *types.NodeInfo) {
}

func ((d *GrpDevice) RemoveNode(nodeName string) {
}

func (d *GrpDevice) PodFitsDevice(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, fillAllocateFrom bool) (bool, []sctypes.PredicateFailureReason, float64) {
	glog.V(5).Infof("Running group scheduler on device requests %+v", podInfo)
	return grpalloc.PodFitsGroupConstraints(nodeInfo, podInfo, fillAllocateFrom)
}

func (d *GrpDevice) PodAllocate(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) error {
	fits, reasons, _ := grpalloc.PodFitsGroupConstraints(nodeInfo, podInfo, true)
	if !fits {
		return fmt.Errorf("Scheduler unable to allocate pod %s as pod no longer fits: %v", podInfo.Name, reasons)
	}
}

func (d *GrpDevice) TakePodResources(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) error {
	grpalloc.TakePodGroupResource(nodeInfo, podInfo)
	return nil
}

func (d *GrpDevice) ReturnPodResources(nodeInfo *types.NodeInfo, podInfo *types.PodInfo) error {
	grpalloc.ReturnPodGroupResource(nodeInfo, podInfo)
	return nil
}

func (d *GrpDevice) GetName() string {
	return "grpdevice"
}

func (d *GrpDevice) UsingGroupScheduler() bool {
	return true
}
