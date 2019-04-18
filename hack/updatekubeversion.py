#!/usr/bin/python

# Updates the vendor directory, assumes the following:
# 1. kubernetes/kubernetes repo is located ../../../../k8s.io/kubernetes
# 2. KubeGPU-scheduler (the code for merging scheduler changes) is located at ../../KubeGPU-scheduler
import shutil
import sys
import os
thisPath = os.path.dirname(os.path.realpath(__file__))
k8s = os.path.join(thisPath, '../../../..', 'k8s.io/kubernetes')
ksched = os.path.join(thisPath, '../..', 'KubeGPU-scheduler')

if __name__ == "__main__":
    newV = sys.argv[1]
    # checkout new version of k8s
    try:
        os.chdir(k8s)
        os.system('git pull')
        os.system('git fetch all')
        os.system('git checkout {0}'.format(newV))
    except Exception:
        print("Not found dir")
        exit()
    # blast the existing vendor tree and copy new stuff
    vendor = os.path.join(thisPath, '..', 'vendor')
    shutil.rmtree(vendor)
    vendork8s = os.path.join(vendor, 'k8s.io/kubernetes')
    os.system('python {0}/extcopy.py -e "*.go" -s {1} -d {2}'.format(thisPath, k8s, vendork8s))
    vendork8svendor = os.path.join(vendork8s, 'vendor')
    for s in os.listdir(vendork8svendor):
        shutil.move(os.path.join(vendork8svendor, s), vendor)
    # move staging stuff
    stagingd = os.path.join(vendork8s, '/staging/src/k8s.io')
    for s in os.listdir(stagingd):
        shutil.move(os.path.join(stagingd, s), os.path.join(vendor, 'k8s.io'))
