#!/bin/bash

go get github.com/golang/glog@v0.0.0-20160126235308-23def4e6c14b
go get github.com/spf13/pflag@v1.0.1

ver=1.14.0
go get k8s.io/kubernetes@v$ver

go get k8s.io/api@kubernetes-$ver
go get k8s.io/apimachinery@kubernetes-$ver
go get k8s.io/apiserver@kubernetes-$ver
go get k8s.io/client-go@kubernetes-$ver
go get k8s.io/component-base@kubernetes-$ver
go get k8s.io/apiextensions-apiserver@kubernetes-$ver
go get k8s.io/cli-runtime@kubernetes-$ver
go get k8s.io/cloud-provider@kubernetes-$ver
go get k8s.io/cluster-bootstrap@kubernetes-$ver
go get k8s.io/code-generator@kubernetes-$ver
go get k8s.io/cri-api@kubernetes-$ver
go get k8s.io/csi-translation-lib@kubernetes-$ver
go get k8s.io/kube-aggregator@kubernetes-$ver
go get k8s.io/kube-controller-manager@kubernetes-$ver
go get k8s.io/kube-proxy@kubernetes-$ver
go get k8s.io/kube-scheduler@kubernetes-$ver
go get k8s.io/kubelet@kubernetes-$ver
go get k8s.io/metrics@kubernetes-$ver
go get k8s.io/node-api@kubernetes-$ver
go get k8s.io/sample-apiserver@kubernetes-$ver
go get k8s.io/sample-cli-plugin@kubernetes-$ver
go get k8s.io/sample-controller@kubernetes-$ver
