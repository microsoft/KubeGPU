package nvidia

import (
	"encoding/json"

	devtypes "github.com/Microsoft/KubeDevice-API/pkg/device"
)

type NvidiaFakePlugin struct {
	volumeDriver string
	volume       string
	gInfo        GpusInfo
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

func NewFakeNvidiaGPUManager(info *GpusInfo, volume string, volumeDriver string) (devtypes.Device, error) {
	plugin := &NvidiaFakePlugin{
		gInfo:        *info,
		volume:       volume,
		volumeDriver: volumeDriver,
	}
	return &NvidiaGPUManager{
		gpus:    make(map[string]GpuInfo),
		np:      plugin,
		useNVML: false,
	}, nil
}
