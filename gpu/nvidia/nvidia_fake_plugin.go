package nvidia

import (
	"encoding/json"

	"github.com/KubeGPU/types"
)

type NvidiaFakePlugin struct {
	volumeDriver string
	volume       string
	gInfo        gpusInfo
}

func (np *NvidiaFakePlugin) GetGPUInfo() ([]byte, error) {
	return json.Marshal(&np.gInfo)
}

func (np *NvidiaFakePlugin) GetGPUCommandLine(devices []int) ([]byte, error) {
	cliString := "--volume-driver=" + np.volumeDriver + " --volume=" + np.volume
	cliString += " --device=/dev/nvidiactl --device=/dev/nvidia-uvm --device=/dev/nvidia-uvm-tools"
	for _, deviceIndex := range devices {
		cliString += " --device=" + np.gInfo.Gpus[deviceIndex].Path
	}
	//fmt.Println("CLI String: ", cliString)
	return []byte(cliString), nil
}

func NewFakeNvidiaGPUManager(info *gpusInfo, volume string, volumeDriver string) (types.DeviceManager, error) {
	plugin := &NvidiaFakePlugin{
		gInfo:        *info,
		volume:       volume,
		volumeDriver: volumeDriver,
	}
	return &nvidiaGPUManager{
		gpus: make(map[string]gpuInfo),
		np:   plugin,
	}, nil
}
