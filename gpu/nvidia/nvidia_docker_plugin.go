/*
Copyright 2017 The Kubernetes Authors.

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

package nvidia

import (
	"io/ioutil"
	"net/http"
)

func getResponse(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

type NvidiaDockerPlugin struct {
}

func (ndp *NvidiaDockerPlugin) GetGPUInfo() ([]byte, error) {
	return getResponse("http://localhost:3476/v1.0/gpu/info/json")
}

func (ndp *NvidiaDockerPlugin) GetGPUCommandLine(devices []int) ([]byte, error) {
	return getResponse("http://localhost:3476/v1.0/docker/cli?dev=" + deviceIndexToString(devices))
}
