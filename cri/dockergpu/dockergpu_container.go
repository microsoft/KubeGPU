package dockergpucri

import (
	"net/url"
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/pkg/apis/componentconfig"
	"k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/dockershim"
	"k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	"k8s.io/kubernetes/pkg/kubelet/server"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

// implementation of runtime service -- have to implement entire docker service
type dockerGPUService struct {
	dockerService dockershim.DockerService
}

func (d *dockerGPUService) Version(apiVersion string) (*runtimeapi.VersionResponse, error) {
	return d.dockerService.Version(apiVersion)
}

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

func (d *dockerGPUService) ExecSync(containerID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, err error) {
	return d.dockerService.ExecSync(containerID, cmd, timeout)
}

func (d *dockerGPUService) Exec(request *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
	return d.dockerService.Exec(request)
}

func (d *dockerGPUService) Attach(req *runtimeapi.AttachRequest) (*runtimeapi.AttachResponse, error) {
	return d.dockerService.Attach(req)
}

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

func (d *dockerGPUService) ContainerStats(req *runtimeapi.ContainerStatsRequest) (*runtimeapi.ContainerStatsResponse, error) {
	return d.dockerService.ContainerStats(req)
}

func (d *dockerGPUService) ListContainerStats(req *runtimeapi.ListContainerStatsRequest) (*runtimeapi.ListContainerStatsResponse, error) {
	return d.dockerService.ListContainerStats(req)
}

func (d *dockerGPUService) UpdateRuntimeConfig(runtimeConfig *runtimeapi.RuntimeConfig) error {
	return d.dockerService.UpdateRuntimeConfig(runtimeConfig)
}

func (d *dockerGPUService) Status() (*runtimeapi.RuntimeStatus, error) {
	return d.dockerService.Status()
}

// =====================

// Gets the streaming server configuration to use with in-process CRI shims.
func getStreamingConfig(kubeCfg *componentconfig.KubeletConfiguration, tlsOptions *server.TLSOptions) *streaming.Config {
	config := &streaming.Config{
		// Use a relative redirect (no scheme or host).
		BaseURL: &url.URL{
			Path: "/cri/",
		},
		StreamIdleTimeout:               kubeCfg.StreamingConnectionIdleTimeout.Duration,
		StreamCreationTimeout:           streaming.DefaultConfig.StreamCreationTimeout,
		SupportedRemoteCommandProtocols: streaming.DefaultConfig.SupportedRemoteCommandProtocols,
		SupportedPortForwardProtocols:   streaming.DefaultConfig.SupportedPortForwardProtocols,
	}
	if tlsOptions != nil {
		config.TLSConfig = tlsOptions.Config
	}
	return config
}

// Basically RunDockerShim
func (d *dockerGPUService) Init(s *options.KubeletServer, crOptions *options.ContainerRuntimeOptions) {
	kubeCfg = s.KubeletConfiguration

	tlsOptions, err := app.InitializeTLS(&s.KubeletFlags, &s.KubeletConfiguration)
	if err != nil {
		return err
	}
	dockerClient := libdocker.ConnectToDockerOrDie(s.DockerEndpoint, s.RuntimeRequestTimeout.Duration, s.ImagePullProgressDeadline.Duration)

	// Create and start the CRI shim running as a grpc server.
	streamingConfig := getStreamingConfig(kubeCfg, kubeDeps)
	ds, err := dockershim.NewDockerService(kubeDeps.DockerClient, kubeCfg.SeccompProfileRoot, crOptions.PodSandboxImage,
		streamingConfig, &pluginSettings, kubeCfg.RuntimeCgroups, kubeCfg.CgroupDriver, crOptions.DockerExecHandlerName,
		crOptions.DockershimRootDirectory, crOptions.DockerDisableSharedPID)
	if err != nil {
		return nil, err
	}
	if err := ds.Start(); err != nil {
		return nil, err
	}

	// The unix socket for kubelet <-> dockershim communication.
	glog.V(5).Infof("RemoteRuntimeEndpoint: %q, RemoteImageEndpoint: %q",
		kubeCfg.RemoteRuntimeEndpoint,
		kubeCfg.RemoteImageEndpoint)
	glog.V(2).Infof("Starting the GRPC server for the docker CRI shim.")
	server := dockerremote.NewDockerServer(kubeCfg.RemoteRuntimeEndpoint, ds)
	if err := server.Start(); err != nil {
		return nil, err
	}
}
