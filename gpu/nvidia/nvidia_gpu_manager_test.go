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
	"encoding/json"
	"testing"

	"strconv"

	v1 "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubelet/gpu"
)

const (
	jsonString = `{"Version":{"Driver":"375.20","CUDA":"8.0"},"Devices":[{"UUID":"GPU00","Path":"/dev/nvidia0","Model":"GeForce GTX TITAN X","Power":250,"CPUAffinity":0,"PCI":{"BusID":"0000:04:00.0","BAR1":256,"Bandwidth":15760},"Clocks":{"Cores":1392,"Memory":3505},"Topology":[{"BusID":"0000:05:00.0","Link":5},{"BusID":"0000:08:00.0","Link":3},{"BusID":"0000:09:00.0","Link":3}],"Family":"Maxwell","Arch":"5.2","Cores":3072,"Memory":{"ECC":false,"Global":12238,"Shared":96,"Constant":64,"L2Cache":3072,"Bandwidth":336480}},{"UUID":"GPU01","Path":"/dev/nvidia1","Model":"GeForce GTX TITAN X","Power":250,"CPUAffinity":0,"PCI":{"BusID":"0000:05:00.0","BAR1":256,"Bandwidth":15760},"Clocks":{"Cores":1392,"Memory":3505},"Topology":[{"BusID":"0000:04:00.0","Link":5},{"BusID":"0000:08:00.0","Link":3},{"BusID":"0000:09:00.0","Link":3}],"Family":"Maxwell","Arch":"5.2","Cores":3072,"Memory":{"ECC":false,"Global":12238,"Shared":96,"Constant":64,"L2Cache":3072,"Bandwidth":336480}},{"UUID":"GPU02","Path":"/dev/nvidia2","Model":"GeForce GTX TITAN X","Power":250,"CPUAffinity":0,"PCI":{"BusID":"0000:08:00.0","BAR1":256,"Bandwidth":15760},"Clocks":{"Cores":1392,"Memory":3505},"Topology":[{"BusID":"0000:04:00.0","Link":3},{"BusID":"0000:05:00.0","Link":3},{"BusID":"0000:09:00.0","Link":5}],"Family":"Maxwell","Arch":"5.2","Cores":3072,"Memory":{"ECC":false,"Global":12238,"Shared":96,"Constant":64,"L2Cache":3072,"Bandwidth":336480}},{"UUID":"GPU03","Path":"/dev/nvidia3","Model":"GeForce GTX TITAN X","Power":250,"CPUAffinity":0,"PCI":{"BusID":"0000:09:00.0","BAR1":256,"Bandwidth":15760},"Clocks":{"Cores":1392,"Memory":3505},"Topology":[{"BusID":"0000:04:00.0","Link":3},{"BusID":"0000:05:00.0","Link":3},{"BusID":"0000:08:00.0","Link":5}],"Family":"Maxwell","Arch":"5.2","Cores":3072,"Memory":{"ECC":false,"Global":12238,"Shared":96,"Constant":64,"L2Cache":3072,"Bandwidth":336480}},{"UUID":"GPU04","Path":"/dev/nvidia4","Model":"GeForce GTX TITAN X","Power":250,"CPUAffinity":1,"PCI":{"BusID":"0000:85:00.0","BAR1":256,"Bandwidth":15760},"Clocks":{"Cores":1392,"Memory":3505},"Topology":[{"BusID":"0000:86:00.0","Link":5},{"BusID":"0000:89:00.0","Link":3},{"BusID":"0000:8A:00.0","Link":3}],"Family":"Maxwell","Arch":"5.2","Cores":3072,"Memory":{"ECC":false,"Global":12238,"Shared":96,"Constant":64,"L2Cache":3072,"Bandwidth":336480}},{"UUID":"GPU05","Path":"/dev/nvidia5","Model":"GeForce GTX TITAN X","Power":250,"CPUAffinity":1,"PCI":{"BusID":"0000:86:00.0","BAR1":256,"Bandwidth":15760},"Clocks":{"Cores":1392,"Memory":3505},"Topology":[{"BusID":"0000:85:00.0","Link":5},{"BusID":"0000:89:00.0","Link":3},{"BusID":"0000:8A:00.0","Link":3}],"Family":"Maxwell","Arch":"5.2","Cores":3072,"Memory":{"ECC":false,"Global":12238,"Shared":96,"Constant":64,"L2Cache":3072,"Bandwidth":336480}},{"UUID":"GPU06","Path":"/dev/nvidia6","Model":"GeForce GTX TITAN X","Power":250,"CPUAffinity":1,"PCI":{"BusID":"0000:89:00.0","BAR1":256,"Bandwidth":15760},"Clocks":{"Cores":1392,"Memory":3505},"Topology":[{"BusID":"0000:85:00.0","Link":3},{"BusID":"0000:86:00.0","Link":3},{"BusID":"0000:8A:00.0","Link":5}],"Family":"Maxwell","Arch":"5.2","Cores":3072,"Memory":{"ECC":false,"Global":12238,"Shared":96,"Constant":64,"L2Cache":3072,"Bandwidth":336480}},{"UUID":"GPU07","Path":"/dev/nvidia7","Model":"GeForce GTX TITAN X","Power":250,"CPUAffinity":1,"PCI":{"BusID":"0000:8A:00.0","BAR1":256,"Bandwidth":15760},"Clocks":{"Cores":1392,"Memory":3505},"Topology":[{"BusID":"0000:85:00.0","Link":3},{"BusID":"0000:86:00.0","Link":3},{"BusID":"0000:89:00.0","Link":5}],"Family":"Maxwell","Arch":"5.2","Cores":3072,"Memory":{"ECC":false,"Global":12238,"Shared":96,"Constant":64,"L2Cache":3072,"Bandwidth":336480}}]}`

	volumeDriver = "nvidia-docker"
	volumeName   = "nvidia_driver_375.20:/usr/local/nvidia:ro"
)

func assertMapEqual(t *testing.T, cap v1.ResourceList, capExpected map[string]int64) {
	if len(cap) != len(capExpected) {
		t.Errorf("Length not same - expected %v - have %v", len(capExpected), len(cap))
	}
	for key, val := range capExpected {
		capV, available := cap[v1.ResourceName(key)]
		if !available {
			t.Errorf("Expected resource %v not available", key)
		}
		if capV.Value() != val {
			t.Errorf("Expected resource %v - expected %v - have %v", key, val, capV.Value())
		}
	}
}

func setAllocFrom(info *gpusInfo, cont *v1.Container, from int, to int) {
	fromS := strconv.Itoa(from)
	toS := info.Gpus[to].ID
	fromLoc := v1.ResourceName(string(v1.ResourceGroupPrefix) + "/gpu/" + fromS + "/cards")
	grp1 := to / 4
	grp0 := to / 2
	prefix := "/gpugrp1/" + strconv.Itoa(grp1) + "/gpugrp0/" + strconv.Itoa(grp0)
	toLoc := v1.ResourceName(string(v1.ResourceGroupPrefix) + prefix + "/gpu/" + toS + "/cards")
	cont.Resources.AllocateFrom[fromLoc] = toLoc
}

func checkElemEqual(t *testing.T, a1 []string, a2 []string) {
	if len(a1) != len(a2) {
		t.Errorf("Lengths don't match %v vs %v", len(a1), len(a2))
	}
	a1Map := make(map[string]int)
	a2Map := make(map[string]int)
	for _, val := range a1 {
		a1Map[val] = a1Map[val] + 1
	}
	for _, val := range a2 {
		a2Map[val] = a2Map[val] + 1
	}
	if len(a1Map) != len(a2Map) {
		t.Errorf("Not same number of unique elements %v vs %v", len(a1Map), len(a2Map))
	}
	for key, val1 := range a1Map {
		val2, available := a2Map[key]
		if !available {
			t.Errorf("Key %v does not exist in 2", key)
		}
		if val1 != val2 {
			t.Errorf("Counts don't match for key %v, cnt1 %v, cnt2 %v", key, val1, val2)
		}
	}
}

func testAlloc(t *testing.T, ngm gpu.GPUManager, info *gpusInfo, alloc map[int]int) {
	container := v1.Container{}
	container.Resources.AllocateFrom = make(v1.ResourceLocation)
	for from, to := range alloc {
		setAllocFrom(info, &container, from, to)
	}
	pod := v1.Pod{}
	pod.Name = "TestPod"
	volumeNameGet, volumeDriverGet, devicesGet, err := ngm.AllocateGPU(&pod, &container)
	if err != nil {
		t.Errorf("Got error %v", err)
	}
	if volumeNameGet != volumeName {
		t.Errorf("Volume name incorrect - expected %v - got %v", volumeName, volumeNameGet)
	}
	if volumeDriverGet != volumeDriver {
		t.Errorf("Volume driver incorrect - expected %v - got %v", volumeDriver, volumeDriverGet)
	}
	devices := []string{"/dev/nvidiactl", "/dev/nvidia-uvm", "/dev/nvidia-uvm-tools"}
	for _, to := range alloc {
		devices = append(devices, info.Gpus[to].Path)
	}
	checkElemEqual(t, devices, devicesGet)
}

func TestAlloc(t *testing.T) {
	var info gpusInfo
	err := json.Unmarshal([]byte(jsonString), &info)
	if err != nil {
		t.Errorf("Got error %v", err)
	}
	//fmt.Println("gpusInfo", info)
	ngm, err := NewFakeNvidiaGPUManager(&info, volumeName, volumeDriver)
	if err != nil {
		t.Errorf("Got error %v", err)
	}
	cap := ngm.Capacity()
	//fmt.Println("Capacity")
	//fmt.Println(ngm.Capacity())

	// test capacity returned
	capExpected := make(map[string]int64)
	capExpected[string(v1.ResourceNvidiaGPU)] = int64(len(info.Gpus))
	for i := 0; i < len(info.Gpus); i++ {
		grp1 := i / 4
		//grp0 := (i / 2) % 2
		grp0 := i / 2
		prefix := "/gpugrp1/" + strconv.Itoa(grp1) + "/gpugrp0/" + strconv.Itoa(grp0)
		capExpected[string(v1.ResourceGroupPrefix)+prefix+"/gpu/"+info.Gpus[i].ID+"/cards"] = 1
		capExpected[string(v1.ResourceGroupPrefix)+prefix+"/gpu/"+info.Gpus[i].ID+"/memory"] = info.Gpus[i].Memory.Global * int64(1024) * int64(1024)
	}
	assertMapEqual(t, cap, capExpected)

	// test alloc GPU00
	alloc := map[int]int{4: 2, 3: 0, 5: 1}
	testAlloc(t, ngm, &info, alloc)
}
