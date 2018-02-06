
# Kubernetes GPU Project

This project aims to provide extensible support for devices such as GPU inside Kubernetes.
It utilizes a group scheduler and group based hierarchical constraints to support GPU topology.
It consists of two parts.

1. Custom CRI to take care of GPU constraints
2. Custom scheduler to allocate and schedule pods

This project is undergoing active development and does not yet have any releases.

# Installing

Clone this repo to $GOPATH/src/github.com/Microsoft/KubeGPU to get it to compile

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
