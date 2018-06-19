/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	goflag "flag"
	"io/ioutil"
	"os"
	"path"

	"github.com/golang/glog"

	"github.com/spf13/pflag"

	"github.com/Microsoft/KubeGPU/device-scheduler/device"
	"github.com/Microsoft/KubeGPU/kube-scheduler/cmd/app"
	utilflag "k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	_ "k8s.io/kubernetes/pkg/version/prometheus"        // for version metric registration
)

func main() {
	command := app.NewSchedulerCommand()

	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	// utilflag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	// add the device schedulers
	var deviceSchedulerPlugins []string
	pluginPath := "/schedulerplugins"
	devPlugins, err := ioutil.ReadDir(pluginPath)
	if err != nil {
		glog.Errorf("Cannot read plugins - skipping")
	}
	for _, pluginFile := range devPlugins {
		deviceSchedulerPlugins = append(deviceSchedulerPlugins, path.Join(pluginPath, pluginFile.Name()))
	}
	device.DeviceScheduler.AddDevicesSchedulerFromPlugins(deviceSchedulerPlugins)

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
