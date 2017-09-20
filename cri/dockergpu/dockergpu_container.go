package dockergpucri

// import (
// 	"k8s.io/kubernetes/pkg/kubelet/apis/cri";
// 	"k8s.io/kubernetes/pkg/kubelet"
// )

import (
	"k8s.io/kubernetes/pkg/kubelet/dockershim"
)

type dockerGPUService struct {
	dockerService dockershim.DockerService
}

func (dockerGPUService *d) Version(apiVersion string) (*runtimeapi.VersionResponse, error) {
	return d.dockerService.Version(apiVersion)
}

func (dockerGPUService *d) CreateContainer(podSandboxID string, config *runtimeapi.ContainerConfig, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	return d.dockerService.CreateContainer(podSandboxID, config, sandboxConfig)
}

func (dockerGPUService *d) StartContainer(containerID string) error {
	return d.dockerService.StartContainer(containerID)
}

func (dockerGPUService *d) StopContainer(containerID string, timeout int64) error {
	return 
}
	// RemoveContainer removes the container.
	RemoveContainer(containerID string) error
	// ListContainers lists all containers by filters.
	ListContainers(filter *runtimeapi.ContainerFilter) ([]*runtimeapi.Container, error)
	// ContainerStatus returns the status of the container.
	ContainerStatus(containerID string) (*runtimeapi.ContainerStatus, error)
	// ExecSync executes a command in the container, and returns the stdout output.
	// If command exits with a non-zero exit code, an error is returned.
	ExecSync(containerID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, err error)
	// Exec prepares a streaming endpoint to execute a command in the container, and returns the address.
	Exec(*runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error)
	// Attach prepares a streaming endpoint to attach to a running container, and returns the address.
	Attach(req *runtimeapi.AttachRequest) (*runtimeapi.AttachResponse, error)
}

func (dockerGPUService *d) Create