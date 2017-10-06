package nvidia

import (
	"strconv"
)

type NvidiaPlugin interface {
	GetGPUInfo() ([]byte, error)
	GetGPUCommandLine(deviceIndex []int) ([]byte, error)
}

func deviceIndexToString(deviceIndex []int) string {
	devString := ""
	for i, index := range deviceIndex {
		if i == 0 {
			devString = strconv.Itoa(index)
		} else {
			devString += "+" + strconv.Itoa(index)
		}
	}
	return devString
}
