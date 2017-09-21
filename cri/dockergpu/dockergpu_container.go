package dockergpucri

import (
	"time"

	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/dockershim"
)

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
