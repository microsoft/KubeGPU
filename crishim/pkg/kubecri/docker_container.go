package kubecri

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"

	"github.com/Microsoft/KubeGPU/crishim/pkg/device"
	"github.com/Microsoft/KubeGPU/crishim/pkg/kubeadvertise"
	"github.com/Microsoft/KubeGPU/kubeinterface"

	"github.com/Microsoft/KubeGPU/types"
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/kubelet/dockershim"
	dockerremote "k8s.io/kubernetes/pkg/kubelet/dockershim/remote"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
)

// implementation of runtime service -- have to implement entire docker service
type dockerExtService struct {
	dockershim.DockerService
	kubeclient *clientset.Clientset
	devmgr     *device.DevicesManager
}

func (d *dockerExtService) modifyContainerConfig(pod *types.PodInfo, cont *types.ContainerInfo, config *runtimeapi.ContainerConfig) error {
	numAllocateFrom := len(cont.AllocateFrom) // may be zero from old scheduler
	nvidiaFullpathRE := regexp.MustCompile(`^/dev/nvidia[0-9]*$`)
	var newDevices []*runtimeapi.Device
	// first remove any existing nvidia devices
	numRequestedGPU := 0
	for _, oldDevice := range config.Devices {
		isNvidiaDevice := false
		if oldDevice.HostPath == "/dev/nvidiactl" ||
			oldDevice.HostPath == "/dev/nvidia-uvm" ||
			oldDevice.HostPath == "/dev/nvidia-uvm-tools" {
			isNvidiaDevice = true
		}
		if nvidiaFullpathRE.MatchString(oldDevice.HostPath) {
			isNvidiaDevice = true
			numRequestedGPU++
		}
		if !isNvidiaDevice || 0 == numAllocateFrom {
			newDevices = append(newDevices, oldDevice)
		}
	}
	if (numAllocateFrom > 0) && (numRequestedGPU > 0) && (numAllocateFrom != numRequestedGPU) {
		return fmt.Errorf("Number of AllocateFrom is different than number of requested GPUs")
	}
	glog.V(3).Infof("Modified devices: %v", newDevices)
	// allocate devices for container
	_, devices, err := d.devmgr.AllocateDevices(pod, cont)
	if err != nil {
		return err
	}
	glog.V(3).Infof("New devices to add: %v", devices)
	// now add devices returned -- skip volumes for now
	for _, device := range devices {
		newDevices = append(newDevices, &runtimeapi.Device{HostPath: device, ContainerPath: device, Permissions: "mrw"})
	}
	config.Devices = newDevices
	return nil
}

// DockerService => RuntimeService => ContainerManager
func (d *dockerExtService) CreateContainer(ctx context.Context, r *runtimeapi.CreateContainerRequest) (*runtimeapi.CreateContainerResponse, error) {
	// overwrite config.Devices here & then call CreateContainer ...
	config := r.Config
	podName := config.Labels[kubelettypes.KubernetesPodNameLabel]
	podNameSpace := config.Labels[kubelettypes.KubernetesPodNamespaceLabel]
	containerName := config.Labels[kubelettypes.KubernetesContainerNameLabel]
	glog.V(3).Infof("Creating container for pod %v container %v", podName, containerName)
	opts := metav1.GetOptions{}
	pod, err := d.kubeclient.CoreV1().Pods(podNameSpace).Get(podName, opts)
	if err != nil {
		glog.Errorf("Retrieving pod %v gives error %v", podName, err)
	}
	glog.V(3).Infof("Pod Spec: %v", pod.Spec)
	// convert to local podInfo structure using annotations available
	podInfo, err := kubeinterface.KubePodInfoToPodInfo(pod, false)
	if err != nil {
		return nil, err
	}
	// modify the container config
	err = d.modifyContainerConfig(podInfo, podInfo.GetContainerInPod(containerName), config)
	if err != nil {
		return nil, err
	}
	return d.DockerService.CreateContainer(ctx, r)
}

// func (d *dockerExtService) ExecSync(containerID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, err error) {
// 	glog.V(5).Infof("Exec sync called %v Cmd %v", containerID, cmd)
// 	return d.DockerService.ExecSync(containerID, cmd, timeout)
// }

// func (d *dockerExtService) Exec(request *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
// 	response, err := d.DockerService.Exec(request)
// 	glog.V(5).Infof("Exec called %v\n Response %v", request, response)
// 	return response, err
// }

// =====================
// Start the shim
func DockerExtInit(f *options.KubeletFlags, c *kubeletconfig.KubeletConfiguration, client *clientset.Clientset, dev *device.DevicesManager) error {
	r := &f.ContainerRuntimeOptions

	// Initialize docker client configuration.
	dockerClientConfig := &dockershim.ClientConfig{
		DockerEndpoint:            r.DockerEndpoint,
		RuntimeRequestTimeout:     c.RuntimeRequestTimeout.Duration,
		ImagePullProgressDeadline: r.ImagePullProgressDeadline.Duration,
	}

	// Initialize network plugin settings.
	pluginSettings := dockershim.NetworkPluginSettings{
		HairpinMode:        kubeletconfig.HairpinMode(c.HairpinMode),
		NonMasqueradeCIDR:  f.NonMasqueradeCIDR,
		PluginName:         r.NetworkPluginName,
		PluginConfDir:      r.CNIConfDir,
		PluginBinDirString: r.CNIBinDir,
		MTU:                int(r.NetworkPluginMTU),
	}

	// Initialize streaming configuration.
	// Initialize TLS
	tlsOptions, err := kubeletapp.InitializeTLS(f, c)
	if err != nil {
		return err
	}
	ipName, nodeName, err := kubeadvertise.GetHostName(f)
	glog.V(2).Infof("Using ipname %v nodeName %v", ipName, nodeName)
	if err != nil {
		return err
	}
	streamingConfig := &streaming.Config{
		Addr:                            fmt.Sprintf("%s:%d", ipName, c.Port),
		StreamIdleTimeout:               c.StreamingConnectionIdleTimeout.Duration,
		StreamCreationTimeout:           streaming.DefaultConfig.StreamCreationTimeout,
		SupportedRemoteCommandProtocols: streaming.DefaultConfig.SupportedRemoteCommandProtocols,
		SupportedPortForwardProtocols:   streaming.DefaultConfig.SupportedPortForwardProtocols,
	}
	if tlsOptions != nil {
		streamingConfig.TLSConfig = tlsOptions.Config
	}

	// if !r.RedirectContainerStreaming, then proxy commands to docker service
	//      client->APIServer->kubelet->crishim_shim->crishim(dockerservice)
	// client->APIServer->kubelet is already TLS (i.e. secure), but overhead (traversing many components)
	// else if r.ReirectContainerStreaming, then upon connection,
	//      client->APIServer->kublet->crishim_shim->crishim(dockerservice) gives redirect
	// client->crishim(dockerservice) - go directly to streaming server, streaming server should use TLS, then it is secure
	// client->APIServer is with TLS, APIServer->kubelet is TLS, kubelet->crishim_shim is localhost REST, crishim_shim->crishim is linux socket
	ds, err := dockershim.NewDockerService(dockerClientConfig, r.PodSandboxImage, streamingConfig, &pluginSettings,
		f.RuntimeCgroups, c.CgroupDriver, r.DockershimRootDirectory, !r.RedirectContainerStreaming)

	if err != nil {
		return err
	}

	dsExt := &dockerExtService{DockerService: ds, kubeclient: client, devmgr: dev}

	if err := dsExt.Start(); err != nil {
		return err
	}

	glog.V(2).Infof("Starting the GRPC server for the docker CRI shim.")
	server := dockerremote.NewDockerServer(f.RemoteRuntimeEndpoint, dsExt)
	if err := server.Start(); err != nil {
		return err
	}

	// Start the streaming server
	if r.RedirectContainerStreaming {
		s := &http.Server{
			Addr:           net.JoinHostPort(c.Address, strconv.Itoa(int(c.Port))),
			Handler:        dsExt,
			TLSConfig:      tlsOptions.Config,
			MaxHeaderBytes: 1 << 20,
		}
		if tlsOptions != nil {
			// this will listen forever
			return s.ListenAndServeTLS(tlsOptions.CertFile, tlsOptions.KeyFile)
		} else {
			return s.ListenAndServe()
		}
	} else {
		var stop = make(chan struct{})
		<-stop // wait forever
		close(stop)
		return nil
	}
}

// func RunDockershim(f *options.KubeletFlags, c *kubeletconfiginternal.KubeletConfiguration, stopCh <-chan struct{}) error {
// 	r := &f.ContainerRuntimeOptions

// 	// Initialize docker client configuration.
// 	dockerClientConfig := &dockershim.ClientConfig{
// 		DockerEndpoint:            r.DockerEndpoint,
// 		RuntimeRequestTimeout:     c.RuntimeRequestTimeout.Duration,
// 		ImagePullProgressDeadline: r.ImagePullProgressDeadline.Duration,
// 	}

// 	// Initialize network plugin settings.
// 	pluginSettings := dockershim.NetworkPluginSettings{
// 		HairpinMode:        kubeletconfiginternal.HairpinMode(c.HairpinMode),
// 		NonMasqueradeCIDR:  f.NonMasqueradeCIDR,
// 		PluginName:         r.NetworkPluginName,
// 		PluginConfDir:      r.CNIConfDir,
// 		PluginBinDirString: r.CNIBinDir,
// 		MTU:                int(r.NetworkPluginMTU),
// 	}

// 	// Initialize streaming configuration. (Not using TLS now)
// 	streamingConfig := &streaming.Config{
// 		// Use a relative redirect (no scheme or host).
// 		BaseURL:                         &url.URL{Path: "/cri/"},
// 		StreamIdleTimeout:               c.StreamingConnectionIdleTimeout.Duration,
// 		StreamCreationTimeout:           streaming.DefaultConfig.StreamCreationTimeout,
// 		SupportedRemoteCommandProtocols: streaming.DefaultConfig.SupportedRemoteCommandProtocols,
// 		SupportedPortForwardProtocols:   streaming.DefaultConfig.SupportedPortForwardProtocols,
// 	}

// 	// Standalone dockershim will always start the local streaming server.
// 	ds, err := dockershim.NewDockerService(dockerClientConfig, r.PodSandboxImage, streamingConfig, &pluginSettings,
// 		f.RuntimeCgroups, c.CgroupDriver, r.DockershimRootDirectory, true /*startLocalStreamingServer*/)
// 	if err != nil {
// 		return err
// 	}
// 	klog.V(2).Infof("Starting the GRPC server for the docker CRI shim.")
// 	server := dockerremote.NewDockerServer(f.RemoteRuntimeEndpoint, ds)
// 	if err := server.Start(); err != nil {
// 		return err
// 	}
// 	<-stopCh
// 	return nil
// }
