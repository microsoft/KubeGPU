# Motivation

The motivation for the work was to provide a framework for scheduling of pods where the constraints on the pod are generic constraints that must be met
on the node where the pod is scheduled. 
The author of the constraints can easily express the constraints in code (i.e. written in Go) and nodes are able to advertise their capabilities to meet
these constraints with parameters that the scheduler can use. For example, the node advertises resources and other resource attributes as parameters.
By allowing infrastructure researchers and developers to be able to easily extend Kubernetes to do this is very powerful.
Although the solution presented here is work for meeting GPU topology constraints, the framework itself can be easily extended to support other
resources and constraints. 

The reason that this extension was written can be clearly explained by considering the problem of generic pod constraints. Suppose a node where a pod
is to be scheduled has resources given by:  
A,B,C,D,E,...  
Consider a case where two pods, Pod1 and Pod2, has constraints which can be met on the node by using the following resources:  
* Pod1's constraints can be met by the resource subset (A,B,C) or subset (C,D)
* Pod2's constraints can be met by the resource subset (D,E)  
Suppose the scheduler assigns Pod1 and Pod2 the given node and assumes that Pod1 is using resources (A,B,C) and Pod2 is using resources (D,E).
However, since there is no explicit information being sent from the scheduler to the node regarding which resources to use, it is possible
the Kubelet decides to satisfy Pod1's contraints with resources (C,D). Then when Pod2 is to be run on the node, it fails as resource D is no
longer available as it is being used by Pod1.
As long as Pod1 is running, Pod2 and other similar pods will be rejected by the node.

This problem can be solved by making sure the Kubelet uses the same resources as the scheduler assumes. This can be accomplished by:  
1. Make the scheduling decision deterministic. That is the Kubelet and scheduler come up with the same resource usage allocation.  However this can have its own
problems and issues which need to be addressed.  
   a. It is difficult to make allocation deterministic. Especially in the case of multiple schedulers, there may be race conditions. Other issues
   such as random ordering when iterating through maps can cause problems.  
   b. The allocation is repeatedly done at both ends.  
   c. The Kubelet needs to offer an option to perform this allocation.  
   d. The kubelet needs to maintain intelligence on resource allocation (for example which GPUs are in use)  
2. Perform the resource allocation decision once at the scheduler side and make the resource allocation decision known to the kubelet prior to pod launching/ container creation.

Option (2) is the best one and avoids a lot of issues. However, it is not easily supported in the existing kubelet / device plugin architecture.
A resource advertiser can easily be run as a daemon set to patch the node information with additional resource attributes by patching the node annotations.
A custom scheduler can then utilize the information along with additional pod constraints specified in the pod annotation to find which nodes
can fit a pod. The resource allocation decision can be communicated to the kubelet by writing back into pod annotations and patching the pod.
However, the kubelet cannot easily consume the resource allocation decision from the scheduler. Even the device plugin cannot currently consume this.
Therefore, the extension here utilizes a custom CRI based on the default CRI shim and simply modifies the container configuration prior to container creation.

# Design

The design used here is the following:
1. An advertiser loop to query the devices for parameters (resource quantities and other attributes) which are patched to the API server using node
annotations.
2. A scheduler which utilizes the node resources and attributes along with resource requests from the pod to:  
   a. Optional: Translate pod constraints/requests to device/resource requests  
   b. See which nodes satisfy all pod constraints  
   c. Allocate resources on the node to meet those constraints  
   d. Patch the pod annotation with resource allocation  
3. A CRI shim which uses the pod annotations to obtain resource allocation to assign devices to the container.  The pod annotations are obtained by using a
Kube Client.  

Currently since both steps (1) and (3) utilize a Kube client, both are built in a single binary, and (2) is built as another binary.

# Going forward

Although the current architecture uses extensions provided in Kubernetes, the use of a new CRI shim may not be needed provided the device plugin
architecture can somehow consume the scheduler's decision. Once that is done, then the resource advertiser may be a daemon set and the cri shim
replaced with a device plugin. On the scheduler side, although a custom scheduler is supported in Kubernetes, the default scheduler could potentially be used
by moving the additional predicate (step 2a and 2b in the Design) to a remote predicate in an extender and the resource allocation and pod patching (step 2c and 2d) being performed in a remote bind in an extender.

If the device plugin supports passing of pod annotations to the device, then item (3) in the Design will become a device plugin, item (1) will be a daemon set (or can be inside the device plugin also), and item (2) will remain as is unless an extender is used.

