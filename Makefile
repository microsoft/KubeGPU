BUILD_DIR ?= _output

.PHONY: all
all: clean kube-scheduler crishim nvidiagpuplugin gpuschedulerplugin

.PHONY: kube-scheduler
kube-scheduler:
	go build -o ${BUILD_DIR}/kube-scheduler ./kube-scheduler/cmd/scheduler.go

.PHONY: crishim
crishim:
	go build -o ${BUILD_DIR}/crishim ./crishim/cmd/crishim.go

.PHONY: nvidiagpuplugin
nvidiagpuplugin:
	go build --buildmode=plugin -o ${BUILD_DIR}/nvidiagpuplugin.so ./nvidiagpuplugin/plugin/nvidiagpu.go

.PHONY: gpuschedulerplugin
gpuschedulerplugin:
	go build --buildmode=plugin -o ${BUILD_DIR}/gpuschedulerplugin.so ./gpuschedulerplugin/plugin/gpuscheduler.go

.PHONY: clean
clean:
	rm -rf ${BUILD_DIR}/*
