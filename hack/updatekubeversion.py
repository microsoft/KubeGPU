#!/usr/bin/python

# Updates the vendor directory, assumes the following:
# 1. kubernetes/kubernetes repo is located ../../../../k8s.io/kubernetes
# 2. KubeGPU-scheduler (the code for merging scheduler changes) is located at ../../KubeGPU-scheduler
import argparse
import shutil
import sys
import os
showOnly = False
thisPath = os.path.dirname(os.path.realpath(__file__))
thisBase = os.path.realpath(os.path.join(thisPath, '..'))
k8s = os.path.realpath(os.path.join(thisPath, '..', '..', '..', '..', 'k8s.io', 'kubernetes'))
ksched = os.path.realpath(os.path.join(thisPath, '..', '..', 'KubeGPU-scheduler'))
extScript = os.path.realpath(os.path.join(thisPath, 'extcopy.py'))
dirsK8sToSched = {
    os.path.join(k8s, 'pkg', 'scheduler') : os.path.join(ksched, 'pkg', 'scheduler'),
    os.path.join(k8s, 'cmd', 'kube-scheduler') : os.path.join(ksched, 'cmd', 'kube-scheduler')
}
dirsSchedToMain = {
    os.path.join(ksched, 'pkg', 'scheduler') : os.path.join(thisBase, 'kube-scheduler', 'pkg'),
    os.path.join(ksched, 'cmd', 'kube-scheduler') : os.path.join(thisBase, 'kube-scheduler', 'cmd')
}

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
    if not showOnly:
        os.system(cmd)

# checkout new version of k8s
def checkout(newV):
    try:
        print('In dir {0}'.format(k8s))
        os.chdir(k8s)
        oscmd('git pull')
        oscmd('git checkout {0}'.format(newV))
    except Exception:
        print("Not found dir")
        exit()

def checkoutSched(branch):
    try:
        print('In dir {0}'.format(ksched))
        os.chdir(ksched)
        oscmd('git checkout {0}'.format(branch))
        oscmd('git pull')
    except Exception:
        print("Not found dir")
        exit()

def tryChDir(newDir):
    try:
        os.chdir(newDir)
    except Exception:
        print("Not found dir {0}".format(newDir))
        exit()

def updateVendor(newV):
    checkout(newV)
    # blast the existing vendor tree and copy new stuff
    vendor = os.path.join(thisPath, '..', 'vendor')
    shutil.rmtree(vendor, ignore_errors=True)
    vendork8s = os.path.join(vendor, 'k8s.io', 'kubernetes')
    for extCopy in ['.go', '.h', '.c', '.cgo', '.cpp', '.s']:
        oscmd('python {0} -e "{1}" -s {2} -d {3}'.format(extScript, extCopy, k8s, vendork8s))
    vendork8svendor = os.path.join(vendork8s, 'vendor')
    tryMove(vendork8svendor, vendor) # move vendor
    stagingd = os.path.join(vendork8s, 'staging', 'src', 'k8s.io')
    #print("StagingDir: {0}".format(stagingd))
    tryMove(stagingd, os.path.join(vendor, 'k8s.io')) # move staging

# Workflow:
# 1. run "updateScheduler" to get new code
# 2. fix merge issues
# 3. "git push origin master" in scheduler codebase - same repo as in updateScheduler
# 4. run "copySchedToMain" to copy code to main codebase & perform basic search/replace
# 5. fix issues in build / exe - commit code (manual push)
# 6. run "copyMainToSched" to copy code from main codebase back to scheduler branch master
def updateScheduler(newV):
    checkout(newV)
    checkoutSched("k8s")
    # remove existing code, copy new code
    for d in dirsK8sToSched:
        shutil.rmtree(dirsK8sToSched[d])
        oscmd('python {0} -e "{1}" -s {2} -d {3}'.format(extScript, ".go", d, dirsK8sToSched[d]))
    oscmd('git add . & git commit --all -m "merge version {0} from k8s" & git push'.format(newV))
    # attempt a merge - do manual push after fixing errors
    oscmd('git checkout master')
    oscmd('git pull origin k8s')

def copySchedToMain():
    for d in dirsSchedToMain:
        shutil.rmtree(dirsSchedToMain[d])
        oscmd('python {0} -e "{1}" -s {2} -d {3}'.format(extScript, ".go", d, dirsSchedToMain[d]))
    # search and replace import paths in kube-scheduler directory
    tryChDir(os.path.join(thisBase, 'kube-scheduler'))
    oscmd("find . -name '*.go' -exec sed -i 's?k8s.io/kubernetes/pkg/scheduler?github.com/Microsoft/KubeGPU/kube-scheduler/pkg?g' {} +")
    oscmd("find . -name '*.go' -exec sed -i 's?k8s.io/kubernetes/cmd/kube-scheduler?github.com/Microsoft/KubeGPU/kube-scheduler/cmd?g' {} +")

def copyMainToSched():
    checkoutSched("master")
    for d in dirsSchedToMain:
        shutil.rmtree(d)
        # dst->src switched
        oscmd('python {0} -e "{1}" -s {2} -d {3}'.format(extScript, ".go", dirsSchedToMain[d], d))
    oscmd('git add . & git commit --all -m "merge latest from main codebase" & git push')

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('-v', '--vendor', default=None, help="Update vendor to given version")
    parser.add_argument('-s', '--scheduler', default=None, help="Update scheduler to given version")
    parser.add_argument('-s2m', '--s2m', action='store_true')
    parser.add_argument('-m2s', '--m2s', action='store_true')
    parser.add_argument('--show', action='store_true')
    args = parser.parse_args()
    showOnly = args.show

    if args.vendor is not None:
        updateVendor(args.vendor)

    if args.scheduler is not None:
        updateScheduler(args.scheduler)
    elif args.s2m:
        copySchedToMain()
    elif args.m2s:
        copyMainToSched()

