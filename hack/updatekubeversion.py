#!/usr/bin/python

# Updates the vendor directory, assumes the following:
# 1. kubernetes/kubernetes repo is located ../../../../k8s.io/kubernetes
# 2. KubeGPU-scheduler (the code for merging scheduler changes) is located at ../../KubeGPU-scheduler
import shutil
import sys
import os
thisPath = os.path.dirname(os.path.realpath(__file__))
k8s = os.path.join(thisPath, '..', '..', '..', '..', 'k8s.io', 'kubernetes')
ksched = os.path.join(thisPath, '..', '..', 'KubeGPU-scheduler')
extScript = os.path.join(thisPath, 'extcopy.py')

def tryMove(src, dst):
    done = False
    try:
        if not os.path.exists(dst):
            shutil.move(src, dst)
            done = True
    except Exception:
        pass

    if not done and os.path.isdir(src):
        for (path, dirs, files) in os.walk(src):
            pathExt = path[len(src)+1:]
            for p in dirs+files:
                tryMove(os.path.join(path, p), os.path.join(dst, pathExt, p))

def oscmd(cmd):
    print(cmd)
    os.system(cmd)

if __name__ == "__main__":
    newV = sys.argv[1]
    # checkout new version of k8s
    try:
        os.chdir(k8s)
        os.system('git pull')
        os.system('git checkout {0}'.format(newV))
    except Exception:
        print("Not found dir")
        exit()
    # blast the existing vendor tree and copy new stuff
    vendor = os.path.join(thisPath, '..', 'vendor')
    shutil.rmtree(vendor, ignore_errors=True)
    vendork8s = os.path.join(vendor, 'k8s.io', 'kubernetes')
    for extCopy in ['.go', '.h', '.c', '.cgo', '.cpp', '.s']:
        oscmd('python {0} -e "{1}" -s {2} -d {3}'.format(extScript, extCopy, k8s, vendork8s))
    vendork8svendor = os.path.join(vendork8s, 'vendor')
    tryMove(vendork8svendor, vendor) # move vendor
    stagingd = os.path.join(vendork8s, 'staging', 'src', 'k8s.io')
    print("StagingDir: {0}".format(stagingd))
    tryMove(stagingd, os.path.join(vendor, 'k8s.io')) # move staging
