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
	return d.dockerService(apiVersion)
}

func 