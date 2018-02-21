
# Kubernetes GPU Project

This project aims to provide extensible support for devices such as GPU inside Kubernetes. 
Although default Kubernetes can support a simple constraint on GPUs, such as a constraint on the number of GPUs needed,
it does not have any support for other constraints on GPUs such as minimum GPU memory or multi-GPU connectivity, e.g. NVLink or
other P2P or fast connections.
This project aims to provide a solution for that as well as develop a framework which others can use to add support for other devices
as well as allowing for arbitrary pod constraints for scheduling.

The project has been started and being worked on by the Cloud Computing and Storage (CCS) team at Microsoft Research Lab in Redmond, USA.

There are two binaries built from this project.
1. **Custom CRI shim and device advertiser**: This binary serves two purposes. The first purpose is to advertise devices and other information to 
be used by the scheduler. The advertisement is done by patching the node annotation on the API server. The second purpose is
to serve as a CRI shim for container creation. The shim modifies the container configuration by using pod annotations provided by the scheduler
which specify which devices are being used.

2. **Custom scheduler**: The purpose of the custom scheduler is to schedule a pod on a node using arbitrary constraints that are specified
on the pod as well as *schedule devices to use on the node*. The second part is why a custom scheduler is needed. Arbitrary constraints
can be specified to a certain extent in default Kubernetes by using scheduler extender or additional remote predicates.
However, the devices to use are not scheduled in default Kubernetes, rather they are determined by the kubelet. In our custom scheduler, nodes
are first evaluated for fit by using an additional device predicate. Then, the devices needed to meet the pod constraints are allocated
on the chosen node. Finally, the chosen devices are written as pod annotations to be consumed by the custom CRI shim.

# Installing

Clone this repo to $GOPATH/src/github.com/Microsoft/KubeGPU to get it to compile. The easiest way to compile the binaries is to use
the provided Makefile. The binaries will be available in the _output folder. 
The scheduler can be be used directly in place of the default scheduler and supports all the same options.
The CRI shim requires the way in which the kubelet is launched. First the CRI shim should be launched, followed by launching of the kubelet.
The argument "--container-runtime=remote" should be used in place of the default "--container-runtime=docker".
The rest of the arguments should be identical to those being used before.

An easy way to install and use the work here is by installing a Kubernetes cluster using the DLWorkspace project,
http://github.com/Microsoft/DLWorkspace. Please use the master branch as opposed to the default alpha.v1.5 branch of this project.
The DLWorkspace project provides a turnkey AI cluster deployment solution by installing a Kubernetes cluster on various
on-prem and cloud providers as well as additional setup, AI job launching, and monitoring capabilities 
using shells scripts and Kubernetes pods.

The following additional setup is needed in order to utilize the custom GPU scheduler in a DLWorkspace deployment. Please launch these
steps prior to running the rest of DLWorkspace deployment.
1. **Modify the configuration file**:  The following lines need to be added to the configuration file prior to launching (config.yaml):  
\# For building Kubernetes docker - specifies the code to use  
k8s-gitrepo : "kubernetes/kubernetes"  
k8s-gitbranch : "v1.9.1"  
k8scri-gitrepo : "Microsoft/KubeGPU"  
k8scri-gitbranch : "master"  
  
\# For deploying custom Kubernetes  
kube\_custom\_cri : True  
kube\_custom\_scheduler: True  
  
2. **Build the custom Kubernetes components**: Prior to launching rest of DLWorkspace deployment, build custom kubernetes components using:  
.\deploy.py build_kube

Please note the cluster status and number of GPUs being used by a job will show up as zero for now with the custom scheduler.  This is being fixed soon.

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
