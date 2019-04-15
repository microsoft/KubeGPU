package app

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/golang/glog"

	"github.com/spf13/pflag"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/version/verflag"

	"github.com/Microsoft/KubeGPU/crishim/pkg/device"
	"github.com/Microsoft/KubeGPU/crishim/pkg/kubeadvertise"
	"github.com/Microsoft/KubeGPU/crishim/pkg/kubecri"
)

// ====================
// Main
func Die(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

type CriShimConfig struct {
	DevicePath string
}

func (cfg *CriShimConfig) New() {
	cfg.DevicePath = "/usr/local/KubeExt/devices"
}

func RunApp() {
	// construct KubeletFlags object and register command line flags mapping
	kubeletFlags := options.NewKubeletFlags()
	kubeletFlags.AddFlags(pflag.CommandLine)

	// construct KubeletConfiguration object and register command line flags mapping
	kubeletConfig, err := options.NewKubeletConfiguration()
	if err != nil {
		Die(err)
	}
	options.AddKubeletConfigFlags(pflag.CommandLine, kubeletConfig)

	criShimCfg := CriShimConfig{}
	criShimCfg.New()
	pflag.CommandLine.StringVar(&criShimCfg.DevicePath, "cridevices", criShimCfg.DevicePath, "The path where device plugins are located")

	// parse the command line flags into the respective objects
	flag.InitFlags()

	// initialize logging and defer flush
	logs.InitLogs()
	defer logs.FlushLogs()

	// short-circuit on verflag
	verflag.PrintAndExitIfRequested()

	// TODO(mtaufen): won't need this this once dynamic config is GA
	// set feature gates so we can check if dynamic config is enabled
	if err := utilfeature.DefaultMutableFeatureGate.SetFromMap(kubeletConfig.FeatureGates); err != nil {
		Die(err)
	}
	// validate the initial KubeletFlags, to make sure the dynamic-config-related flags aren't used unless the feature gate is on
	if err := options.ValidateKubeletFlags(kubeletFlags); err != nil {
		Die(err)
	}
	if len(kubeletFlags.KubeletConfigFile) > 0 {
		Die(fmt.Errorf("Not supported - configuration file"))
	}

	// construct a KubeletServer from kubeletFlags and kubeletConfig
	kubeletServer := &options.KubeletServer{
		KubeletFlags:         *kubeletFlags,
		KubeletConfiguration: *kubeletConfig,
	}

	// add device plugins and start device manager
	var devicePlugins []string
	devicePluginFiles, err := ioutil.ReadDir(criShimCfg.DevicePath)
	if err != nil {
		glog.Errorf("Unable to list devices, skipping adding of devices - error %v", err)
	}
	for _, f := range devicePluginFiles {
		devicePlugins = append(devicePlugins, path.Join(criShimCfg.DevicePath, f.Name()))
	}
	device.DeviceManager.AddDevicesFromPlugins(devicePlugins)
	device.DeviceManager.Start()

	done := make(chan bool)
	// start the device advertiser
	da, err := kubeadvertise.StartDeviceAdvertiser(kubeletServer, done)
	if err != nil {
		Die(err)
	}
	// run the gpushim
	if err := kubecri.DockerExtInit(kubeletFlags, kubeletConfig, da.KubeClient, da.DevMgr); err != nil {
		Die(err)
	}
	<-done // wait forever
	done <- true
}
