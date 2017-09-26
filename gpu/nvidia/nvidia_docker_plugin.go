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
