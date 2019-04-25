
# Kubernetes GPU Project (KubeGPU)

The KubeGPU consists of two parts, one are the core extensions to Kubernetes (a CRI shim and a custom scheduler), and the other are the device-specific
implementations implemented using Golang plugins for further extensibility.

The project has been started and being worked on by the Cloud Computing and Storage (CCS) team at Microsoft Research Lab in Redmond, USA.

1. **Core Kubernetes Extension Components**: As part of the core extensions, the following two binaries are built.  Common components used by both are provided in the `types`, `utils`, and `kubeinterface` directories. The code in `kubeinterface` provides an interface between core Kubernetes data structures and those used by the extensions.

    a. *Custom CRI shim and device advertiser*: This binary serves two purposes. The first purpose is to advertise devices and other information to be used by the scheduler. The advertisement is done by patching the node annotation on the API server. The second purpose is to serve as a CRI shim for container creation. The shim modifies the container configuration by using pod annotations. As an example, these annotations could be provided by the scheduler which specify which devices are being used.  However, the actual modifications made to the container configuration are done inside the `plugins`.

    Code for the kubecri is inside the `kubecri` directory.

    b. *Custom scheduler*: The purpose of the custom scheduler is to schedule a pod on a node using arbitrary constraints that are specified by the pod using the device scheduler plugins. The scheduler allows for finding the node for the pod to run on as well as *schedule devices to use on the node*. The second part is why a custom scheduler is needed. Arbitrary constraints can already be specified to a certain extent in default Kubernetes by using scheduler extender or additional remote predicates. However, the devices to use are not scheduled in a default Kubernetes scheduler. In our custom scheduler, nodes are first evaluated for fit by using an additional device predicate. Then, the devices needed to meet the pod constraints are allocated on the chosen node. Finally, the chosen devices are written as pod annotations to be consumed by the custom CRI shim.

    Code for the custom scheduling is inside the `device-scheduler` directory. A fork of the default Kuberenetes scheduler with minor modifications to connect with our code is in the `kube-scheduler` directory.

2. **Plugins**: Plugins are device-specific code to be used by the CRI shim/device advertiser and the custom device scheduler. They are compiled using `--buildmode=plugin` as shown in the `Makefile`. All device-specific code resides inside the plugins as opposed to the core extensions.

A plugin for NVidia GPU scheduling is provided here and can be used as an example. This plugin can provide scheduling of constraints such as minimum GPU memory as well as other hardware topology constraints, e.g. multi-GPU connectivity, using NVLink or other P2P or fast connections. 

# Adding other devices

You can add other devices by forking and adding code directly into the plugins directory.

To add other devices is fairly easy. For the CRI shim and device advertiser, you simply need to create a structure type which supports the `Device` interface in `kubecri/pkg/types/types.go`.  You can use the `NvidiaGPUManager` class in `plugins/nvidiagpuplugin/gpu/nvidia/nvidia_gpu_manager.go` as an example. Then, you need to create the plugin by creating a constructor function, `CreateDevicePlugin()`, which the extension code will search for to create the `Device`, as done in `plugins/nvidiagpuplugin/plugin/nvidiagpu.go`. The `Device` interface is given by the following.

    type Device interface {
        // New creates the device and initializes it
        New() error
        // Start logically initializes the device
        Start() error
        // UpdateNodeInfo - updates a node info structure by writing capacity, allocatable, used, scorer
        UpdateNodeInfo(*types.NodeInfo) error
        // Allocate attempst to allocate the devices
        // Returns list of (VolumeName, VolumeDriver), and list of Devices to use
        // Returns an error on failure.
        Allocate(*types.PodInfo, *types.ContainerInfo) ([]Volume, []string, error)
        // GetName returns the name of a device
        GetName() string
    }

To add device scheduling capability, you need to create a structure which implements the the `DeviceScheduler` interface defined in `device-scheduler/types/types.go` and create a function which creates an object of this type called, `CreateDeviceSchedulerPlugin()`. An example of a device scheduler plugin is shown by the `NvidiaGPUScheduler` class in `plugins/gpuschedulerplugin/gpu_scheduler.go` and plugin creation is shown in `plugins/gpuschedulerplugin/plugin/gpuscheduler.go`.  The `DeviceScheduler` interface is given by the following.

    type DeviceScheduler interface {
        // add node and resources
        AddNode(nodeName string, nodeInfo *types.NodeInfo)
        // remove node
        RemoveNode(nodeName string)
        // see if pod fits on node & return device score
        PodFitsDevice(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, fillAllocateFrom bool, runGrpScheduler bool) (bool, []PredicateFailureReason, float64)
        // allocate resources
        PodAllocate(nodeInfo *types.NodeInfo, podInfo *types.PodInfo, runGrpScheduler bool) error
        // take resources from node
        TakePodResources(*types.NodeInfo, *types.PodInfo, bool) error
        // return resources to node
        ReturnPodResources(*types.NodeInfo, *types.PodInfo, bool) error
        // GetName returns the name of a device
        GetName() string
        // Tells whether group scheduler is being used?
        UsingGroupScheduler() bool
    }

# Installing

Clone this repo to $GOPATH/src/github.com/Microsoft/KubeGPU to get it to compile. Alternatively, you can use `go get github.com/Microsoft/KubeGPU`. The easiest way to compile the binaries is to use the provided Makefile. The binaries will be available in the `_output` folder. 

The scheduler can be be used directly in place of the default scheduler and supports all the same options.
The CRI shim changes the way in which the kubelet is launched. First the CRI shim should be launched, followed by launching of the kubelet.
The argument `--container-runtime=remote` should be used in place of the default `--container-runtime=docker`.
The rest of the arguments should be identical to those being used before.

An easy way to install and use the work here is by installing a Kubernetes cluster using the DLWorkspace project,
http://github.com/Microsoft/DLWorkspace. Please use the master branch as opposed to the default alpha.v1.5 branch of this project.
The DLWorkspace project provides a turnkey AI cluster deployment solution by installing a Kubernetes cluster on various
on-prem and cloud providers as well as additional setup, AI job launching, and monitoring capabilities 
using shells scripts and Kubernetes pods.

The following additional setup is needed in order to utilize the custom GPU scheduler in a DLWorkspace deployment. Please launch these
steps prior to running the rest of DLWorkspace deployment.
1. **Modify the configuration file**:  The following lines need to be added to the configuration file (config.yaml) prior to launching setup:  
\# For deploying custom Kubernetes  
kube\_custom\_cri : True  
kube\_custom\_scheduler: True  
  
2. **Build the custom Kubernetes components**: Prior to launching rest of DLWorkspace deployment, build custom kubernetes components using the following:  
./deploy.py build_kube

# Design

More information about the current design and reasons for doing it in this way is provided [here.](docs/kubegpu.md)

# Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
