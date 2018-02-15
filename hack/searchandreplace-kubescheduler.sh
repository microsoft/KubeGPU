find . -name '*.go' -exec sed -i 's?k8s.io/kubernetes/plugin/cmd/kube-scheduler?github.com/KubeGPU/kube-scheduler/cmd?g' {} +
