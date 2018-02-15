find . -name '*.go' -exec sed -i 's?k8s.io/kubernetes/plugin/pkg/scheduler?github.com/KubeGPU/kube-scheduler/pkg?g' {} +
