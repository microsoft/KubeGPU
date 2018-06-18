package app

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"
	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/version/verflag"

	"github.com/Microsoft/KubeGPU/crishim/pkg/kubecri"
)

// ====================
// Main
func Die(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func RunApp() {
	// construct KubeletFlags object and register command line flags mapping
	kubeletFlags := options.NewKubeletFlags()
	kubeletFlags.AddFlags(pflag.CommandLine)

	// construct KubeletConfiguration object and register command line flags mapping
	defaultConfig, err := options.NewKubeletConfiguration()
	if err != nil {
		Die(err)
	}
	options.AddKubeletConfigFlags(pflag.CommandLine, defaultConfig)

	// parse the command line flags into the respective objects
	flag.InitFlags()

	// initialize logging and defer flush
	logs.InitLogs()
	defer logs.FlushLogs()

	// short-circuit on verflag
	verflag.PrintAndExitIfRequested()

	// TODO(mtaufen): won't need this this once dynamic config is GA
	// set feature gates so we can check if dynamic config is enabled
	if err := utilfeature.DefaultFeatureGate.SetFromMap(defaultConfig.FeatureGates); err != nil {
		Die(err)
	}
	// validate the initial KubeletFlags, to make sure the dynamic-config-related flags aren't used unless the feature gate is on
	if err := options.ValidateKubeletFlags(kubeletFlags); err != nil {
		Die(err)
	}
	// bootstrap the kubelet config controller, app.BootstrapKubeletConfigController will check
	// feature gates and only turn on relevant parts of the controller
	kubeletConfig, _, err := kubeletapp.BootstrapKubeletConfigController(
		defaultConfig, kubeletFlags.InitConfigDir, kubeletFlags.DynamicConfigDir)
	if err != nil {
		Die(err)
	}

	// construct a KubeletServer from kubeletFlags and kubeletConfig
	kubeletServer := &options.KubeletServer{
		KubeletFlags:         *kubeletFlags,
		KubeletConfiguration: *kubeletConfig,
	}

	done := make(chan bool)
	// start the device manager
	da, err := kubecri.StartDeviceManager(kubeletServer, done)
	if err != nil {
		Die(err)
	}
	// run the gpushim
	if err := kubecri.DockerGPUInit(kubeletFlags, kubeletConfig, da.KubeClient, da.DevMgr); err != nil {
		Die(err)
	}
	<-done // wait forever
	done <- true
}
