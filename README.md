
# Kubernetes GPU Project (KubeGPU)

The KubeGPU consists of two parts, one are the core extensions to Kubernetes (a CRI shim and a custom scheduler), and the other are the device-specific
implementations implemented using Golang plugins for further extensibility.
The project has been started and being worked on by Microsoft Research Lab in Redmond, USA.

The main project has now been split into the core and the plugins.  For the core, please see:
[https://github.com/Microsoft/KubeDevice].
The NVIDIA GPU plugins (which can be used in place of the standard NVIDIA device plugin) is located here.

# Building the plugin

To build the plugin, make sure you have a Go installation. Then, run
```
go get github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml
go get github.com/Microsoft/KubeDevice-API
go get github.com/Microsoft/KubeGPU
cd $GOPATH/src/github.com/Microsoft/KubeGPU
make
```

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
