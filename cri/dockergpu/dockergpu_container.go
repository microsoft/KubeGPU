package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

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
	"k8s.io/kubernetes/pkg/version/verflag"
)

// implementation of runtime service -- have to implement entire docker service
type dockerGPUService struct {
	dockerService dockershim.DockerService
}

// DockerService
func (d *dockerGPUService) Start() error {
	return d.dockerService.Start()
}

// DockerService => RuntimeService => RuntimeVersioner
func (d *dockerGPUService) Version(apiVersion string) (*runtimeapi.VersionResponse, error) {
	return d.dockerService.Version(apiVersion)
}

// DockerService => RuntimeService => ContainerManager
func (d *dockerGPUService) CreateContainer(podSandboxID string, config *runtimeapi.ContainerConfig, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	return d.dockerService.CreateContainer(podSandboxID, config, sandboxConfig)
}

func (d *dockerGPUService) StartContainer(containerID string) error {
	return d.dockerService.StartContainer(containerID)
}

func (d *dockerGPUService) StopContainer(containerID string, timeout int64) error {
	return d.dockerService.StopContainer(containerID, timeout)
}

func (d *dockerGPUService) RemoveContainer(containerID string) error {
	return d.dockerService.RemoveContainer(containerID)
}

func (d *dockerGPUService) ListContainers(filter *runtimeapi.ContainerFilter) ([]*runtimeapi.Container, error) {
	return d.dockerService.ListContainers(filter)
}

func (d *dockerGPUService) ContainerStatus(containerID string) (*runtimeapi.ContainerStatus, error) {
	return d.dockerService.ContainerStatus(containerID)
}

// func (d *dockerGPUService) UpdateContainerResources(containerID string, resources *runtimeapi.LinuxContainerResources) error {
// 	return d.dockerService.UpdateContainerResources(containerID, resources)
// }

func (d *dockerGPUService) ExecSync(containerID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, err error) {
	return d.dockerService.ExecSync(containerID, cmd, timeout)
}

func (d *dockerGPUService) Exec(request *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
	return d.dockerService.Exec(request)
}

func (d *dockerGPUService) Attach(req *runtimeapi.AttachRequest) (*runtimeapi.AttachResponse, error) {
	return d.dockerService.Attach(req)
}

// DockerService => RuntimeService => PodSandboxManager
func (d *dockerGPUService) RunPodSandbox(config *runtimeapi.PodSandboxConfig) (string, error) {
	return d.dockerService.RunPodSandbox(config)
}

func (d *dockerGPUService) StopPodSandbox(podSandboxID string) error {
	return d.dockerService.StopPodSandbox(podSandboxID)
}

func (d *dockerGPUService) RemovePodSandbox(podSandboxID string) error {
	return d.dockerService.RemovePodSandbox(podSandboxID)
}

func (d *dockerGPUService) PodSandboxStatus(podSandboxID string) (*runtimeapi.PodSandboxStatus, error) {
	return d.dockerService.PodSandboxStatus(podSandboxID)
}

func (d *dockerGPUService) ListPodSandbox(filter *runtimeapi.PodSandboxFilter) ([]*runtimeapi.PodSandbox, error) {
	return d.dockerService.ListPodSandbox(filter)
}

func (d *dockerGPUService) PortForward(req *runtimeapi.PortForwardRequest) (*runtimeapi.PortForwardResponse, error) {
	return d.dockerService.PortForward(req)
}

// DockerService => RuntimeService => ContainerStatsManager
// func (d *dockerGPUService) ContainerStats(containerID string) (*runtimeapi.ContainerStats, error) {
// 	return d.dockerService.ContainerStats(containerID)
// }
func (d *dockerGPUService) ContainerStats(req *runtimeapi.ContainerStatsRequest) (*runtimeapi.ContainerStatsResponse, error) {
	return d.dockerService.ContainerStats(req)
}

func (d *dockerGPUService) ListContainerStats(req *runtimeapi.ListContainerStatsRequest) (*runtimeapi.ListContainerStatsResponse, error) {
	return d.dockerService.ListContainerStats(req)
}

// DockerService => RuntimeService
func (d *dockerGPUService) UpdateRuntimeConfig(runtimeConfig *runtimeapi.RuntimeConfig) error {
	return d.dockerService.UpdateRuntimeConfig(runtimeConfig)
}

func (d *dockerGPUService) Status() (*runtimeapi.RuntimeStatus, error) {
	return d.dockerService.Status()
}

// DockerService => ImageManagerService
func (d *dockerGPUService) ListImages(filter *runtimeapi.ImageFilter) ([]*runtimeapi.Image, error) {
	return d.dockerService.ListImages(filter)
}

func (d *dockerGPUService) ImageStatus(image *runtimeapi.ImageSpec) (*runtimeapi.Image, error) {
	return d.dockerService.ImageStatus(image)
}

func (d *dockerGPUService) PullImage(image *runtimeapi.ImageSpec, auth *runtimeapi.AuthConfig) (string, error) {
	return d.dockerService.PullImage(image, auth)
}

func (d *dockerGPUService) RemoveImage(image *runtimeapi.ImageSpec) error {
	return d.dockerService.RemoveImage(image)
}

// func (d *dockerGPUService) ImageFsInfo() ([]*runtimeapi.FilesystemUsage, error) {
// 	return d.dockerService.ImageFsInfo()
// }
func (d *dockerGPUService) ImageFsInfo(req *runtimeapi.ImageFsInfoRequest) (*runtimeapi.ImageFsInfoResponse, error) {
	return d.dockerService.ImageFsInfo(req)
}

// DockerService => http.Handler
func (d *dockerGPUService) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	d.dockerService.ServeHTTP(writer, req)
}

// =====================
// Start the shim
func DockerGPUInit(f *options.KubeletFlags, c *componentconfig.KubeletConfiguration, r *options.ContainerRuntimeOptions) error {
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

	// Initialize streaming configuration. (Not using TLS now)
	streamingConfig := &streaming.Config{
		// Use a relative redirect (no scheme or host).
		BaseURL:                         &url.URL{Path: "/cri/"},
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

	dsGPU := &dockerGPUService{dockerService: ds}

	glog.V(2).Infof("Starting the GRPC server for the docker CRI shim.")
	server := dockerremote.NewDockerServer(c.RemoteRuntimeEndpoint, dsGPU)
	if err := server.Start(); err != nil {
		return err
	}

	// Start the streaming server
	//addr := net.JoinHostPort(c.Address, strconv.Itoa(int(c.Port)))
	//return http.ListenAndServe(addr, ds)

	// wait forever
	done := make(chan bool)
	<-done
	return nil
}

// Gets the streaming server configuration to use with in-process CRI shims.
// func getStreamingConfig(kubeCfg *componentconfig.KubeletConfiguration, tlsOptions *server.TLSOptions) *streaming.Config {
// 	config := &streaming.Config{
// 		// Use a relative redirect (no scheme or host).
// 		BaseURL: &url.URL{
// 			Path: "/cri/",
// 		},
// 		StreamIdleTimeout:               kubeCfg.StreamingConnectionIdleTimeout.Duration,
// 		StreamCreationTimeout:           streaming.DefaultConfig.StreamCreationTimeout,
// 		SupportedRemoteCommandProtocols: streaming.DefaultConfig.SupportedRemoteCommandProtocols,
// 		SupportedPortForwardProtocols:   streaming.DefaultConfig.SupportedPortForwardProtocols,
// 	}
// 	if tlsOptions != nil {
// 		config.TLSConfig = tlsOptions.Config
// 	}
// 	return config
// }

// Basically RunDockerShim
// func (d *dockerGPUService) Init(s *options.KubeletServer, crOptions *options.ContainerRuntimeOptions) {
// 	kubeCfg = s.KubeletConfiguration

// 	tlsOptions, err := app.InitializeTLS(&s.KubeletFlags, &s.KubeletConfiguration)
// 	if err != nil {
// 		return err
// 	}
// 	dockerClient := libdocker.ConnectToDockerOrDie(s.DockerEndpoint, s.RuntimeRequestTimeout.Duration, s.ImagePullProgressDeadline.Duration)

// 	// Create and start the CRI shim running as a grpc server.
// 	streamingConfig := getStreamingConfig(kubeCfg, kubeDeps)
// 	ds, err := dockershim.NewDockerService(kubeDeps.DockerClient, kubeCfg.SeccompProfileRoot, crOptions.PodSandboxImage,
// 		streamingConfig, &pluginSettings, kubeCfg.RuntimeCgroups, kubeCfg.CgroupDriver, crOptions.DockerExecHandlerName,
// 		crOptions.DockershimRootDirectory, crOptions.DockerDisableSharedPID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if err := ds.Start(); err != nil {
// 		return nil, err
// 	}

// 	// The unix socket for kubelet <-> dockershim communication.
// 	glog.V(5).Infof("RemoteRuntimeEndpoint: %q, RemoteImageEndpoint: %q",
// 		kubeCfg.RemoteRuntimeEndpoint,
// 		kubeCfg.RemoteImageEndpoint)
// 	glog.V(2).Infof("Starting the GRPC server for the docker CRI shim.")
// 	server := dockerremote.NewDockerServer(kubeCfg.RemoteRuntimeEndpoint, ds)
// 	if err := server.Start(); err != nil {
// 		return nil, err
// 	}
// }
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
	if err := DockerGPUInit(&s.KubeletFlags, &s.KubeletConfiguration, &s.ContainerRuntimeOptions); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	// if s.ExperimentalDockershim {
	// 	if err := app.RunDockershim(&s.KubeletConfiguration, &s.ContainerRuntimeOptions); err != nil {
	// 		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	// 		os.Exit(1)
	// 	}
	// }
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
