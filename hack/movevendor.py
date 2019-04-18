#!/usr/bin/python 
# NOT USED FILE
import os
import argparse
import textwrap
import re
import shutil

#def splitPaths(pathToSplit):
#    return pathToSplit.split('/')

def removeRoot(rootdir, name):
    m = re.match(os.path.normpath(rootdir)+'/(.*)', name)
    return m.group(1)

def copyOrLink(rootdir, src, fd, dst, skippattern, isdir):
    srcName = os.path.join(src, fd)
    #srcPaths = splitPaths(srcName)
    #dstName = os.path.join(dst, *(srcPaths[2:]))
    dstName = os.path.join(dst, removeRoot(rootdir, srcName))
    if re.match(skippattern, srcName) is None:
        if os.path.islink(dstName):
            print "Remove link: {0}".format(dstName)
            os.unlink(dstName)
        if not isdir:
            if os.path.exists(dstName):
                print "Remove existing file: {0}".format(dstName)
                os.remove(dstName)
        parentDir = os.path.dirname(dstName)
        if not os.path.exists(parentDir):
            print "Make parent directory: {0}".format(os.path.dirname(dstName))
            os.mkdir(os.path.dirname(dstName))
        if os.path.islink(srcName):
            dstLinkTo = os.path.normpath(os.path.join(src, os.readlink(srcName)))
            dstLinkConvt = os.path.relpath(dstLinkTo, os.path.dirname(dstName))
            print "S:{0} N:{1} L:{2}".format(src, srcName, os.readlink(srcName))
            print "D:{0} DL:{1} DC:{2}".format(dstName, dstLinkTo, dstLinkConvt)
            print "Symlink Src: {0} Dst: {1}".format(dstLinkConvt, dstName)
            os.symlink(dstLinkConvt, dstName)
        elif isdir:
            if not os.path.exists(dstName):
                print "Create directory: {0}".format(dstName)
                os.mkdir(dstName)
        else:
            print "Copy file Src: {0} Dst: {1}".format(srcName, dstName)
            shutil.copy(srcName, dstName)

def recurse(dir, dst, skippattern):
    for root, folders, files in os.walk(dir, topdown=True):
        for folder in folders:
            copyOrLink(dir, root, folder, dst, skippattern, True)
            #print os.path.join(root,folder)
        for file in files:
            copyOrLink(dir, root, file, dst, skippattern, False)
            #print os.path.join(root,file)

if __name__ == '__main__':
    parser = argparse.ArgumentParser(prog='movevendor.py',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=textwrap.dedent('''\
            Move files from one directory to another, links are preserved even for links pointing outside of directory scope
            '''))
    parser.add_argument("source",
        help = "Source directory")
    parser.add_argument("dest",
        help = "Destination directory")
    args = parser.parse_args()

    recurse(args.source, args.dest, '.*/\_output/.*')

