#!/usr/bin/python 
import os
import argparse
import textwrap
import re

def copyOrLink(src, fd, dst, skippattern, isdir):
    srcName = os.path.join(src, fd)
    dstName = os.path.join(dst, fd)
    if re.match(skippattern, srcName) is None:
        if os.path.islink(srcName):
            dstLinkTo = os.path.normpath(os.path.join(src, os.readlink(srcName)))
            print "S: {0} N:{1} L:{2} DN:{3} D:{4}".format(src, srcName, os.readlink(srcName), dstName, dstLinkTo)

def recurse(dir, dst, skippattern):
    for root, folders, files in os.walk(dir, topdown=True):
        for folder in folders:
            copyOrLink(root, folder, dst, skippattern, True)
            #print os.path.join(root,folder)
        for file in files:
            copyOrLink(root, file, dst, skippattern, False)
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

