#!/usr/bin/env python

import argparse
import glob
import os
import string
import subprocess
import sys

# peloton_proto = './vendor/code.uber.internal/infra/peloton/protobuf/'
peloton_proto = './protobuf/'
protoc_cmd = (
    'protoc --proto_path={proto_path} --{generator}_out={mflag}:{gen_dir}'
    ' {file}'
)


def protos():
    f = []
    # Py2 glob has no **
    for g in ['*/*/*.proto', '*/*/*/*.proto', '*/*/*/*/*.proto']:
        f += glob.glob(peloton_proto + g)
    return f


def mflags(files, go_loc):
    pfiles = [string.replace(f, peloton_proto, '') for f in files]
    pfiles.remove('peloton/api/peloton.proto')
    m = string.join(['M' + f + '=' + go_loc +
                     os.path.dirname(f) for f in pfiles], ',')
    m += ',Mpeloton/api/peloton.proto=%speloton/api/peloton' % go_loc
    return m


def generate(generator, f, m, gen_dir):
    print protoc_cmd.format(proto_path=peloton_proto, mflag='${mflag}',
                            gen_dir=gen_dir, file=f, generator=generator)
    cmd = protoc_cmd.format(proto_path=peloton_proto, mflag=m,
                            gen_dir=gen_dir, file=f, generator=generator)
    retval = subprocess.call(cmd, shell=True)

    if retval != 0:
        sys.exit(retval)


def parse_args():
    parser = argparse.ArgumentParser(
        description='Generate types and yarpc stubs from protobuf files')
    parser.add_argument('-l', '--go-loc', help='go location of generated code',
                        default='code.uber.internal/infra/peloton/.gen/')
    parser.add_argument('-o', '--out', help='output dir of generated code',
                        default='.gen')
    args = parser.parse_args()
    return args


def main():
    args = parse_args()
    files = protos()
    m = mflags(files, args.go_loc)

    # For every .proto file in peloton generate us a golang file
    for f in files:
        generate("go", f, m, args.out)

        # Generate yarpc-go files for all files with a service. The yarpc
        # plugin generates bad output for files without any services.
        with open(f) as o:
            lines = o.readlines()

            for l in lines:
                if l.startswith('service '):
                    generate("yarpc-go", f, m, args.out)
                    break


if __name__ == '__main__':
    main()
