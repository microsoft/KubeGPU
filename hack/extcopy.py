import argparse
import os
import shutil
from pathlib import Path

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Copy all files with given extension from src to dst")
    parser.add_argument('-e', '--ext', default=None, help="Extension to copy")
    parser.add_argument('-s', '--src', help="Src Directory")
    parser.add_argument('-d', '--dst', help="Dst Directory")
    args = parser.parse_args()
    for (path, dirs, files) in os.walk(args.src):
        pathExt = path[len(args.src)+1:]
        #print("EXT: {0}".format(pathExt))
        for file in files:
            srcF = os.path.join(path, file)
            dstF = os.path.join(args.dst, pathExt, file)
            dstDir = os.path.dirname(dstF)
            #print("{0} {1} {2}".format(srcF, dstF, dstDir))
            if args.ext is None or srcF.endswith(args.ext):
                if not os.path.exists(dstDir):
                    Path(dstDir).mkdir(parents=True, exist_ok=True)
                print("Copy {0} to {1}".format(srcF, dstF))
                shutil.copy(srcF, dstF)