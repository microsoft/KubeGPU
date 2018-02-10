package kubeadvertise

import (
	"fmt"
	"time"

	"github.com/Microsoft/KubeGPU/gpuextension/device"
	"github.com/Microsoft/KubeGPU/kubeinterface"
	"github.com/Microsoft/KubeGPU/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	//kubetypes "k8s.io/apimachinery/pkg/types"
	//nodeutil "k8s.io/kubernetes/pkg/util/node"
)

type DeviceAdvertiser struct {
	KubeClient *clientset.Clientset
	DevMgr     *device.DevicesManager
	nodeName   string
}

func NewDeviceAdvertiser(s *options.KubeletServer, dm *device.DevicesManager, thisNodeName string) (*DeviceAdvertiser, error) {
	clientConfig, err := app.CreateAPIServerClientConfig(s)
	if err != nil {
		return nil, err
	}
	kubeClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	da := &DeviceAdvertiser{KubeClient: kubeClient, DevMgr: dm, nodeName: thisNodeName}
	return da, nil
}

func (da *DeviceAdvertiser) patchResources() error {
	// Get current node status
	opts := metav1.GetOptions{}
	node, err := da.KubeClient.Core().Nodes().Get(da.nodeName, opts)
	if err != nil {
		return fmt.Errorf("error getting current node %q: %v", da.nodeName, err)
	}

	newNode := node.DeepCopy()

	// update the node status here with device resources ...
	nodeInfo := types.NewNodeInfoWithName(da.nodeName)
	da.DevMgr.UpdateNodeInfo(nodeInfo)
	// write node info into annotations
	kubeinterface.NodeInfoToAnnotation(&newNode.ObjectMeta, nodeInfo)

	// Patch the current status on the API server
	_, err = kubeinterface.PatchNodeMetadata(da.KubeClient.CoreV1(), da.nodeName, node, newNode)
	if err != nil {
		return err
	}
	return nil
}

func (da *DeviceAdvertiser) AdvertiseLoop(intervalMs int, tryAgainIntervalMs int, done chan bool) {
	intervalDuration := time.Duration(intervalMs) * time.Millisecond
	tickChan := time.NewTicker(intervalDuration)
	lastSuccessfulPatch := time.Now()
	for {
		select {
		case <-tickChan.C:
			if time.Since(lastSuccessfulPatch) > intervalDuration {
				err := da.patchResources()
				if err != nil {
					tickChanOnErr := time.NewTicker(time.Duration(tryAgainIntervalMs) * time.Millisecond)
					for {
						select {
						case <-tickChanOnErr.C:
							err = da.patchResources()
						case <-done:
							return
						}
						if err == nil {
							tickChanOnErr.Stop()
							//close(tickChanOnErr.C)
							break // back to original timer
						}
					}
				} else {
					lastSuccessfulPatch = time.Now()
				}
			}
		case <-done:
			return
		}
	}
}
