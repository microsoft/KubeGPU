BUILD_DIR ?= _output

.PHONY: all
all: clean nvidiagpuplugin gpuschedulerplugin

.PHONY: nvidiagpuplugin
nvidiagpuplugin:
	go build --buildmode=plugin -o ${BUILD_DIR}/nvidiagpuplugin.so ./nvidiagpuplugin/plugin/nvidiagpu.go

.PHONY: gpuschedulerplugin
gpuschedulerplugin:
	go build --buildmode=plugin -o ${BUILD_DIR}/gpuschedulerplugin.so ./gpuschedulerplugin/plugin/gpuscheduler.go

.PHONY: clean
clean:
	rm -rf ${BUILD_DIR}/*

.PHONY: test
test:
	cd ./gpuplugintypes; go test; cd ../gpuschedulerplugin; go test; cd ../nvidiagpuplugin/gpu/nvidia; go test

