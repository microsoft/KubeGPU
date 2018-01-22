package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/KubeGPU/device"
	"github.com/KubeGPU/kubeinterface"

	"github.com/KubeGPU/cri/kubeadvertise"
	"github.com/KubeGPU/types"
	"github.com/golang/glog"
	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"
	clientset "k8s.io/client-go/kubernetes"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/kubelet"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig"
	"k8s.io/kubernetes/pkg/kubelet/dockershim"
	dockerremote "k8s.io/kubernetes/pkg/kubelet/dockershim/remote"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubernetes/pkg/version/verflag"
)

// implementation of runtime service -- have to implement entire docker service
type dockerGPUService struct {
	dockershim.DockerService
	kubeclient *clientset.Clientset
	devmgr     *device.DevicesManager
}

func (d *dockerGPUService) modifyContainerConfig(pod *types.PodInfo, cont *types.ContainerInfo, config *runtimeapi.ContainerConfig) error {
	nvidiaFullpathRE := regexp.MustCompile(`^/dev/nvidia[0-9]*$`)
	var newDevices []*runtimeapi.Device
	// first remove any existing nvidia devices
	for _, oldDevice := range config.Devices {
		isNvidiaDevice := false
		if oldDevice.HostPath == "/dev/nvidiactl" ||
			oldDevice.HostPath == "/dev/nvidia-uvm" ||
			oldDevice.HostPath == "/dev/nvidia-uvm-tools" {
			isNvidiaDevice = true
		}
		if nvidiaFullpathRE.MatchString(oldDevice.HostPath) {
			isNvidiaDevice = true
		}
		if !isNvidiaDevice {
			newDevices = append(newDevices, oldDevice)
		}
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
func (d *dockerGPUService) CreateContainer(podSandboxID string, config *runtimeapi.ContainerConfig, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	// overwrite config.Devices here & then call CreateContainer ...
	podName := config.Labels[kubelettypes.KubernetesPodNameLabel]
	podNameSpace := config.Labels[kubelettypes.KubernetesPodNamespaceLabel]
	containerName := config.Labels[kubelettypes.KubernetesContainerNameLabel]
	glog.V(3).Infof("Creating container for pod %v container %v", podName, containerName)
	opts := metav1.GetOptions{}
	pod, err := d.kubeclient.Core().Pods(podNameSpace).Get(podName, opts)
	if err != nil {
		glog.Errorf("Retrieving pod %v gives error %v", podName, err)
	}
	glog.V(3).Infof("Pod Spec: %v", pod.Spec)
	// convert to local podInfo structure
	podInfo := kubeinterface.KubePodInfoToPodInfo(&pod.Spec)
	// use annotations to add fields to podInfo
	kubeinterface.AnnotationToPodInfo(&pod.ObjectMeta, podInfo)
	// modify the container config
	err = d.modifyContainerConfig(podInfo, podInfo.GetContainerInPod(containerName), config)
	if err != nil {
		return "", err
	}
	return d.DockerService.CreateContainer(podSandboxID, config, sandboxConfig)
}

// func (d *dockerGPUService) ExecSync(containerID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, err error) {
// 	glog.V(5).Infof("Exec sync called %v Cmd %v", containerID, cmd)
// 	return d.DockerService.ExecSync(containerID, cmd, timeout)
// }

// func (d *dockerGPUService) Exec(request *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
// 	response, err := d.DockerService.Exec(request)
// 	glog.V(5).Infof("Exec called %v\n Response %v", request, response)
// 	return response, err
// }

// =====================
// Start the shim
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

func StartDeviceManager(s *options.KubeletServer, done chan bool) (*kubeadvertise.DeviceAdvertiser, error) {
	// create a device manager using nvidiagpu as the only device
	dm := &device.DevicesManager{}
	if err := dm.CreateAndAddDevice("nvidiagpu"); err != nil {
		return nil, err
	}
	// start the device manager
	dm.Start()

	_, nodeName, err := GetHostName(&s.KubeletFlags) // nodeName is name of machine
	if err != nil {
		return nil, err
	}
	da, err := kubeadvertise.NewDeviceAdvertiser(s, dm, nodeName)
	if err != nil {
		return nil, err
	}
	// start the advertisement loop
	go da.AdvertiseLoop(20000, 1000, done)

	return da, nil
}

func DockerGPUInit(f *options.KubeletFlags, c *kubeletconfig.KubeletConfiguration, client *clientset.Clientset, dev *device.DevicesManager) error {
	r := &f.ContainerRuntimeOptions

	// Initialize docker client configuration.
	dockerClientConfig := &dockershim.ClientConfig{
		DockerEndpoint:            r.DockerEndpoint,
		RuntimeRequestTimeout:     c.RuntimeRequestTimeout.Duration,
		ImagePullProgressDeadline: r.ImagePullProgressDeadline.Duration,
	}

	// Initialize network plugin settings.
	nh := &kubelet.NoOpLegacyHost{}
	pluginSettings := dockershim.NetworkPluginSettings{
		HairpinMode:       kubeletconfig.HairpinMode(c.HairpinMode),
		NonMasqueradeCIDR: f.NonMasqueradeCIDR,
		PluginName:        r.NetworkPluginName,
		PluginConfDir:     r.CNIConfDir,
		PluginBinDir:      r.CNIBinDir,
		MTU:               int(r.NetworkPluginMTU),
		LegacyRuntimeHost: nh,
	}

	// Initialize streaming configuration.
	// Initialize TLS
	tlsOptions, err := kubeletapp.InitializeTLS(f, c)
	if err != nil {
		return err
	}
	ipName, nodeName, err := GetHostName(f)
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

	ds, err := dockershim.NewDockerService(dockerClientConfig, r.PodSandboxImage, streamingConfig, &pluginSettings,
		f.RuntimeCgroups, c.CgroupDriver, r.DockershimRootDirectory, r.DockerDisableSharedPID)

	if err != nil {
		return err
	}

	dsGPU := &dockerGPUService{DockerService: ds, kubeclient: client, devmgr: dev}

	if err := dsGPU.Start(); err != nil {
		return err
	}

	glog.V(2).Infof("Starting the GRPC server for the docker CRI shim.")
	server := dockerremote.NewDockerServer(f.RemoteRuntimeEndpoint, dsGPU)
	if err := server.Start(); err != nil {
		return err
	}

	// Start the streaming server
	s := &http.Server{
		Addr:           net.JoinHostPort(c.Address, strconv.Itoa(int(c.Port))),
		Handler:        dsGPU,
		TLSConfig:      tlsOptions.Config,
		MaxHeaderBytes: 1 << 20,
	}
	if tlsOptions != nil {
		// this will listen forever
		return s.ListenAndServeTLS(tlsOptions.CertFile, tlsOptions.KeyFile)
	} else {
		return s.ListenAndServe()
	}
}

// ====================
// Main
func die(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func main() {
	// construct KubeletFlags object and register command line flags mapping
	kubeletFlags := options.NewKubeletFlags()
	kubeletFlags.AddFlags(pflag.CommandLine)

	// construct KubeletConfiguration object and register command line flags mapping
	defaultConfig, err := options.NewKubeletConfiguration()
	if err != nil {
		die(err)
	}
	options.AddKubeletConfigFlags(pflag.CommandLine, defaultConfig)

	// parse the command line flags into the respective objects
	flag.InitFlags()

	// initialize logging and defer flush
	logs.InitLogs()
	defer logs.FlushLogs()

	// short-circuit on verflag
	verflag.PrintAndExitIfRequested()

	// TODO(mtaufen): won't need this this once dynamic config is GA
	// set feature gates so we can check if dynamic config is enabled
	if err := utilfeature.DefaultFeatureGate.SetFromMap(defaultConfig.FeatureGates); err != nil {
		die(err)
	}
	// validate the initial KubeletFlags, to make sure the dynamic-config-related flags aren't used unless the feature gate is on
	if err := options.ValidateKubeletFlags(kubeletFlags); err != nil {
		die(err)
	}
	// bootstrap the kubelet config controller, app.BootstrapKubeletConfigController will check
	// feature gates and only turn on relevant parts of the controller
	kubeletConfig, _, err := kubeletapp.BootstrapKubeletConfigController(
		defaultConfig, kubeletFlags.InitConfigDir, kubeletFlags.DynamicConfigDir)
	if err != nil {
		die(err)
	}

	// construct a KubeletServer from kubeletFlags and kubeletConfig
	kubeletServer := &options.KubeletServer{
		KubeletFlags:         *kubeletFlags,
		KubeletConfiguration: *kubeletConfig,
	}

	done := make(chan bool)
	// start the device manager
	da, err := StartDeviceManager(kubeletServer, done)
	if err != nil {
		die(err)
	}
	// run the gpushim
	if err := DockerGPUInit(kubeletFlags, kubeletConfig, da.KubeClient, da.DevMgr); err != nil {
		die(err)
	}
	<-done // wait forever
	done <- true
}
