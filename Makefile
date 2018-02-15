BUILD_DIR ?= _output

.PHONY: all
all: clean kube-scheduler crishim

.PHONY: kube-scheduler
kube-scheduler:
	go build -o ${BUILD_DIR}/kube-scheduler ./kube-scheduler/cmd/scheduler.go

.PHONY: crishim
crishim:
	go build -o ${BUILD_DIR}/crishim ./crishim/cmd/crishim.go


.PHONY: clean
clean:
	rm -rf ${BUILD_DIR}/*