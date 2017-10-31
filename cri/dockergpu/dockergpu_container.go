package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/apiserver/pkg/util/logs"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/kubelet"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	kubeletconfiginternal "k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig"
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

func (d *dockerGPUService) UpdateContainerResources(ctx context.Context, r *runtimeapi.UpdateContainerResourcesRequest) (*runtimeapi.UpdateContainerResourcesResponse, error) {
	return d.dockerService.UpdateContainerResources(ctx, r)
}

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
func (d *dockerGPUService) ContainerStats(containerID string) (*runtimeapi.ContainerStats, error) {
	return d.dockerService.ContainerStats(containerID)
}

func (d *dockerGPUService) ListContainerStats(filter *runtimeapi.ContainerStatsFilter) ([]*runtimeapi.ContainerStats, error) {
	return d.dockerService.ListContainerStats(filter)
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

func (d *dockerGPUService) ImageFsInfo() ([]*runtimeapi.FilesystemUsage, error) {
	return d.dockerService.ImageFsInfo()
}

// DockerService => http.Handler
func (d *dockerGPUService) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	d.dockerService.ServeHTTP(writer, req)
}

// =====================
// Start the shim
func DockerGPUInit(c *kubeletconfiginternal.KubeletConfiguration, r *options.ContainerRuntimeOptions) error {
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
		HairpinMode:       kubeletconfiginternal.HairpinMode(c.HairpinMode),
		NonMasqueradeCIDR: c.NonMasqueradeCIDR,
		PluginName:        r.NetworkPluginName,
		PluginConfDir:     r.CNIConfDir,
		PluginBinDir:      binDir,
		MTU:               int(r.NetworkPluginMTU),
		LegacyRuntimeHost: nh,
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

	ds, err := dockershim.NewDockerService(dockerClient, c.SeccompProfileRoot, r.PodSandboxImage,
		streamingConfig, &pluginSettings, c.RuntimeCgroups, c.CgroupDriver, r.DockerExecHandlerName, r.DockershimRootDirectory,
		r.DockerDisableSharedPID)

	dsGpu := &dockerGPUService{dockerService: ds}

	if err != nil {
		return err
	}
	if err := dsGpu.Start(); err != nil {
		return err
	}

	glog.V(2).Infof("Starting the GRPC server for the docker CRI shim.")
	server := dockerremote.NewDockerServer(c.RemoteRuntimeEndpoint, dsGpu)
	if err := server.Start(); err != nil {
		return err
	}

	// Start the streaming server - blocks?
	addr := net.JoinHostPort(c.Address, strconv.Itoa(int(c.Port)))
	return http.ListenAndServe(addr, dsGpu)
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

// WordSepNormalizeFunc changes all flags that contain "_" separators
func WordSepNormalizeFunc(f *pflag.FlagSet, name string) pflag.NormalizedName {
	if strings.Contains(name, "_") {
		return pflag.NormalizedName(strings.Replace(name, "_", "-", -1))
	}
	return pflag.NormalizedName(name)
}

// WarnWordSepNormalizeFunc changes and warns for flags that contain "_" separators
func WarnWordSepNormalizeFunc(f *pflag.FlagSet, name string) pflag.NormalizedName {
	if strings.Contains(name, "_") {
		nname := strings.Replace(name, "_", "-", -1)
		glog.Warningf("%s is DEPRECATED and will be removed in a future version. Use %s instead.", name, nname)

		return pflag.NormalizedName(nname)
	}
	return pflag.NormalizedName(name)
}

// InitFlags normalizes, parses, then logs the command line flags
func InitFlags() {
	pflag.CommandLine.SetNormalizeFunc(WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	pflag.Parse()
	pflag.VisitAll(func(flag *pflag.Flag) {
		glog.V(4).Infof("FLAG: --%s=%q", flag.Name, flag.Value)
	})
}

// ====================
// Main
func main() {
	s := options.NewKubeletServer()
	AddFlags(s, pflag.CommandLine)

	InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	verflag.PrintAndExitIfRequested()

	// run the gpushim
	if err := DockerGPUInit(&s.KubeletConfiguration, &s.ContainerRuntimeOptions); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
