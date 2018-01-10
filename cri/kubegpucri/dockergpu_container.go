package main

import (
	"github.com/KubeGPU/devicemanager"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/KubeGPU/cri/kubeadvertise"
	"github.com/golang/glog"
	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/kubelet"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/dockershim"
	"k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	dockerremote "k8s.io/kubernetes/pkg/kubelet/dockershim/remote"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubernetes/pkg/version/verflag"
)

// implementation of runtime service -- have to implement entire docker service
type dockerGPUService struct {
	dockershim.DockerService
	advertiser *kubeadvertise.DeviceAdvertiser
}

// DockerService => RuntimeService => ContainerManager
func (d *dockerGPUService) CreateContainer(podSandboxID string, config *runtimeapi.ContainerConfig, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	// overwrite config.Devices here & then call CreateContainer ...
	podName := config.Labels[kubelettypes.KubernetesPodNameLabel]
	podNameSpace := config.Labels[kubelettypes.KubernetesPodNamespaceLabel]
	containerName := config.Labels[kubelettypes.KubernetesContainerNameLabel]
	glog.V(3).Infof("Creating container for pod %v container %v", podName, containerName)
	opts := metav1.GetOptions{}
	pod, err := d.advertiser.KubeClient.Core().Pods(podNameSpace).Get(podName, opts)
	if err != nil {
		glog.Errorf("Retrieving pod %v gives error %v", podName, err)
	}
	glog.V(3).Infof("Pod Spec: %v", pod.Spec)
	return d.DockerService.CreateContainer(podSandboxID, config, sandboxConfig)
}

func (d *dockerGPUService) ExecSync(containerID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, err error) {
	glog.V(5).Infof("Exec sync called %v Cmd %v", containerID, cmd)
	return d.DockerService.ExecSync(containerID, cmd, timeout)
}

func (d *dockerGPUService) Exec(request *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
	response, err := d.DockerService.Exec(request)
	glog.V(5).Infof("Exec called %v\n Response %v", request, response)
	return response, err
}

// =====================
// Start the shim
func GetHostName(f *options.KubeletFlags) (string, string, error) {
	// 1) Use nodeIP if set
	// 2) If the user has specified an IP to HostnameOverride, use it
	// 3) Lookup the IP from node name by DNS and use the first non-loopback ipv4 address
	// 4) Try to get the IP from the network interface used as default gateway
	name := ""
	hostName := nodeutil.GetHostname(f.HostnameOverride)
	if f.NodeIP != "" {
		name = f.NodeIP
	} else {
		var addr net.IP
		if addr = net.ParseIP(hostName); addr == nil {
			var err error
			addr, err = utilnet.ChooseHostInterface()
			if err != nil {
				return "", hostName, err
			}
		}
		name = addr.String()
	}
	return name, hostName, nil
}

func DockerGPUInit(s *options.KubeletServer, f *options.KubeletFlags, c *componentconfig.KubeletConfiguration, r *options.ContainerRuntimeOptions) error {
	// create channel to notify we are finished
	done := make(chan bool)
	// Create docker client.
	dockerClient := libdocker.ConnectToDockerOrDie(r.DockerEndpoint, c.RuntimeRequestTimeout.Duration,
		r.ImagePullProgressDeadline.Duration)

	// Initialize network plugin settings.
	binDir := r.CNIBinDir
	if binDir == "" {
		binDir = r.NetworkPluginDir
	}
	nh := &kubelet.NoOpLegacyHost{}
	pluginSettings := dockershim.NetworkPluginSettings{
		HairpinMode:       componentconfig.HairpinMode(c.HairpinMode),
		NonMasqueradeCIDR: c.NonMasqueradeCIDR,
		PluginName:        r.NetworkPluginName,
		PluginConfDir:     r.CNIConfDir,
		PluginBinDir:      binDir,
		MTU:               int(r.NetworkPluginMTU),
		LegacyRuntimeHost: nh,
	}

	// initialize TLS
	tlsOptions, err := kubeletapp.InitializeTLS(f, c)
	if err != nil {
		return err
	}

	// Initialize streaming configuration.
	hostName, nodeName, err := GetHostName(f)
	glog.V(2).Infof("Using hostname %v nodeName %v", hostName, nodeName)
	if err != nil {
		return err
	}
	streamingConfig := &streaming.Config{
		Addr:                            fmt.Sprintf("%s:%d", hostName, c.Port),
		StreamIdleTimeout:               c.StreamingConnectionIdleTimeout.Duration,
		StreamCreationTimeout:           streaming.DefaultConfig.StreamCreationTimeout,
		SupportedRemoteCommandProtocols: streaming.DefaultConfig.SupportedRemoteCommandProtocols,
		SupportedPortForwardProtocols:   streaming.DefaultConfig.SupportedPortForwardProtocols,
	}
	if tlsOptions != nil {
		streamingConfig.TLSConfig = tlsOptions.Config
	}

	// ds, err := dockershim.NewDockerService(dockerClient, r.PodSandboxImage, streamingConfig, &pluginSettings,
	// 	c.RuntimeCgroups, c.CgroupDriver, r.DockerExecHandlerName, r.DockershimRootDirectory, r.DockerDisableSharedPID)
	ds, err := dockershim.NewDockerService(dockerClient, c.SeccompProfileRoot, r.PodSandboxImage,
		streamingConfig, &pluginSettings, c.RuntimeCgroups, c.CgroupDriver, r.DockerExecHandlerName, r.DockershimRootDirectory,
		r.DockerDisableSharedPID)

	if err != nil {
		return err
	}

	if err := ds.Start(); err != nil {
		return err
	}

	// create a device manager using nvidiagpu as the only device
	dm := &devicemanager.DevicesManager{}
	if err := dm.CreateAndAddDevice("nvidiagpu"); err != nil {
		return err
	}
	// start the device manager
	dm.Start()

	da, err := kubeadvertise.NewDeviceAdvertiser(s, dm, nodeName)
	if err != nil {
		return err
	}
	// start the advertisement loop
	go da.AdvertiseLoop(20000, 1000, done)

	// create the GPU service
	dsGPU := &dockerGPUService{DockerService: ds, advertiser: da}

	glog.V(2).Infof("Starting the GRPC server for the docker CRI shim.")
	server := dockerremote.NewDockerServer(c.RemoteRuntimeEndpoint, dsGPU)
	if err := server.Start(); err != nil {
		return err
	}

	if c.EnableServer {
		// Start the streaming server
		s := &http.Server{
			Addr:           net.JoinHostPort(c.Address, strconv.Itoa(int(c.Port))),
			Handler:        dsGPU,
			TLSConfig:      tlsOptions.Config,
			MaxHeaderBytes: 1 << 20,
		}
		if tlsOptions != nil {
			// this will listen forever
			ret := s.ListenAndServeTLS(tlsOptions.CertFile, tlsOptions.KeyFile)
			done <- true
			return ret
		} else {
			ret := s.ListenAndServe()
			done <- true
			return ret
		}
	} else {
		// wait forever
		<-done
		return nil
	}
}

// ====================
// Main
func die(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func main() {
	s := options.NewKubeletServer()
	s.AddFlags(pflag.CommandLine)

	flag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	verflag.PrintAndExitIfRequested()

	// run the gpushim
	if err := DockerGPUInit(s, &s.KubeletFlags, &s.KubeletConfiguration, &s.ContainerRuntimeOptions); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// From 1.8
// func main() {
// 	// construct KubeletFlags object and register command line flags mapping
// 	kubeletFlags := options.NewKubeletFlags()
// 	kubeletFlags.AddFlags(pflag.CommandLine)

// 	// construct KubeletConfiguration object and register command line flags mapping
// 	defaultConfig, err := options.NewKubeletConfiguration()
// 	if err != nil {
// 		die(err)
// 	}
// 	options.AddKubeletConfigFlags(pflag.CommandLine, defaultConfig)

// 	// parse the command line flags into the respective objects
// 	flag.InitFlags()

// 	// initialize logging and defer flush
// 	logs.InitLogs()
// 	defer logs.FlushLogs()

// 	// short-circuit on verflag
// 	verflag.PrintAndExitIfRequested()

// 	// validate the initial KubeletFlags, to make sure the dynamic-config-related flags aren't used unless the feature gate is on
// 	if err := options.ValidateKubeletFlags(kubeletFlags); err != nil {
// 		die(err)
// 	}
// 	// bootstrap the kubelet config controller, app.BootstrapKubeletConfigController will check
// 	// feature gates and only turn on relevant parts of the controller
// 	kubeletConfig, kubeletConfigController, err := app.BootstrapKubeletConfigController(
// 		defaultConfig, kubeletFlags.InitConfigDir, kubeletFlags.DynamicConfigDir)
// 	if err != nil {
// 		die(err)
// 	}

// 	// construct a KubeletServer from kubeletFlags and kubeletConfig
// 	kubeletServer := &options.KubeletServer{
// 		KubeletFlags:         *kubeletFlags,
// 		KubeletConfiguration: *kubeletConfig,
// 	}

// 	// use kubeletServer to construct the default KubeletDeps
// 	kubeletDeps, err := app.UnsecuredDependencies(kubeletServer)
// 	if err != nil {
// 		die(err)
// 	}

// 	// add the kubelet config controller to kubeletDeps
// 	kubeletDeps.KubeletConfigController = kubeletConfigController

// 	// run the gpushim
// 	if err := DockerGPUInit(kubeletConfig, &kubeletFlags.ContainerRuntimeOptions); err != nil {
// 		fmt.Fprintf(os.Stderr, "error: %v\n", err)
// 		os.Exit(1)
// 	}
// }
