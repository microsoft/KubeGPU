package kubeadvertise

import (
	"fmt"
	"net"
	"time"

	"github.com/Microsoft/KubeGPU/crishim/pkg/device"
	"github.com/Microsoft/KubeGPU/kubeinterface"
	"github.com/Microsoft/KubeGPU/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
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

func GetHostName(f *options.KubeletFlags) (string, string, error) {
	// 1) Use nodeIP if set
	// 2) If the user has specified an IP to HostnameOverride, use it
	// 3) Lookup the IP from node name by DNS and use the first non-loopback ipv4 address
	// 4) Try to get the IP from the network interface used as default gateway
	ipName := ""
	nodeName := nodeutil.GetHostname(f.HostnameOverride)
	if f.NodeIP != "" {
		ipName = f.NodeIP
	} else {
		var addr net.IP
		if addr = net.ParseIP(nodeName); addr == nil {
			var err error
			addr, err = utilnet.ChooseHostInterface()
			if err != nil {
				return "", nodeName, err
			}
		}
		ipName = addr.String()
	}
	return ipName, nodeName, nil
}

func StartDeviceAdvertiser(s *options.KubeletServer, done chan bool) (*DeviceAdvertiser, error) {
	_, nodeName, err := GetHostName(&s.KubeletFlags) // nodeName is name of machine
	if err != nil {
		return nil, err
	}
	da, err := NewDeviceAdvertiser(s, device.DeviceManager, nodeName)
	if err != nil {
		return nil, err
	}
	// start the advertisement loop
	go da.AdvertiseLoop(20000, 5000, done)

	return da, nil
}
