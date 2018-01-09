package kubeadvertise

import (
	"fmt"
	"strconv"
	"time"

	"github.com/KubeGPU/devicemanager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	kubev1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
)

type DeviceAdvertiser struct {
	KubeClient *clientset.Clientset
	DevMgr     *devicemanager.DevicesManager
	nodeName   string
}

func NewDeviceAdvertiser(s *options.KubeletServer, nodeName string) (*DeviceAdvertiser, error) {
	clientConfig, err := app.CreateAPIServerClientConfig(s)
	if err != nil {
		return nil, err
	}
	kubeClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	da := &DeviceAdvertiser{KubeClient: kubeClient}
	return da, nil
}

func (da *DeviceAdvertiser) patchResources() error {
	// Get current node status
	opts := metav1.GetOptions{}
	node, err := da.KubeClient.Core().Nodes().Get(da.nodeName, opts)
	if err != nil {
		return fmt.Errorf("error getting current node %q: %v", da.nodeName, err)
	}

	clonedNode, err := conversion.NewCloner().DeepCopy(node)
	if err != nil {
		return fmt.Errorf("error clone node %q: %v", da.nodeName, err)
	}

	newNode, ok := clonedNode.(*kubev1.Node)
	if !ok || newNode == nil {
		return fmt.Errorf("failed to cast %q node object %#v to v1.Node", da.nodeName, clonedNode)
	}

	// update the node status here with device resources ...
	resources := da.DevMgr.Capacity()
	for resName, resVal := range resources {
		newNode.ObjectMeta.Annotations[string(resName)] = strconv.FormatInt(resVal, 10)
	}

	// Patch the current status on the API server
	_, err = nodeutil.PatchNodeStatus(da.KubeClient, types.NodeName(da.nodeName), node, newNode)
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
