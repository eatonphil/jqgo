#!/usr/bin/env python3

import glob
import json
import os
import shlex
import subprocess
import sys
import tempfile
from datetime import datetime

DEBUG = '-d' in sys.argv or '--debug' in sys.argv
WIN = os.name == 'nt'

def cmd(to_run, bash=False, doNotReplaceWin=False):
    pieces = shlex.split(to_run)
    if WIN and not doNotReplaceWin:
        for i, piece in enumerate(pieces):
            pieces[i] = piece.replace('./dsq', './dsq.exe').replace('/', '\\')
    elif bash or '|' in pieces:
        pieces = ['bash', '-c', to_run]

    return subprocess.run(pieces, cwd=os.getcwd(), capture_output=True, check=True)

tests = 0
failures = 0

def test(name, to_run, want, fail=False, sort=False, winSkip=False, within_seconds=None, want_stderr=None):
    global tests
    global failures
    
    skip = False
    for i, arg in enumerate(sys.argv):
        if arg == '-f' or arg == '--filter':
            if sys.argv[i+1].lower() not in name.lower():
                return
        if arg == '-fo' or arg == '--filter-out':
            if sys.argv[i+1].lower() in name.lower():
                return

    tests += 1
    skipped = True

    t1 = datetime.now()

    print('STARTING: ' + name)
    if DEBUG:
        print(to_run)

    if WIN and winSkip or skip:
      print('  SKIPPED\n')
      print()
      return

    try:
        res = cmd(to_run)
        got = res.stdout.decode()

        got_err = res.stderr.decode()
        if want_stderr and got_err != want_stderr:
            failures += 1
            print(f'  FAILURE: stderr mismatch. Got "{got_err}", wanted "{want_stderr}".')
            print()
            return
    
        if sort:
            got = json.dumps(json.loads(got), sort_keys=True)
            want = json.dumps(json.loads(want), sort_keys=True)
    except json.JSONDecodeError as e:
        failures += 1
        print('  FAILURE: bad JSON: ' + got)
        print()
        return
    except Exception as e:
        if not fail:
            print(f'  FAILURE: unexpected failure: {0} {1}', str(e), e.output.decode())
            failures += 1
            print()
            return
        else:
            got = e.output.decode()
            skipped = False
    if fail and skipped:
        print(f'  FAILURE: unexpected success')
        failures += 1
        print()
        return
    if WIN and '/' in want:
        want = want.replace('/', '\\')
    if want.strip() != got.strip():
        print(f'  FAILURE')
        try:
            with tempfile.NamedTemporaryFile() as want_fp:
                want_fp.write(want.strip().encode())
                want_fp.flush()
                with tempfile.NamedTemporaryFile() as got_fp:
                    got_fp.write(got.strip().encode())
                    got_fp.flush()
                    diff_res = cmd(f'diff {want_fp.name} {got_fp.name} || true', bash=True)
                    print(diff_res.stdout.decode())
        except Exception as e:
            print(e.cmd, e.output.decode())
        failures += 1
        print()
        return

    t2 = datetime.now()
    s = (t2-t1).seconds
    if within_seconds and s > within_seconds:
        print(f'  FAILURE: completed in {s} seconds. Wanted <{within_seconds}s')
        failures += 1
        return

    print(f'  SUCCESS\n')

everything_tests = [
    ["d.b.c", "1"],
    ["a", '"1"'],
    ["x.0", "2"],
    ["x.2.1.aa", "12"],
]
for t in everything_tests:
    to_run = "cat testdata/everything.json | ./jqgo " + t[0]
    test("Basic test of " + t[0], to_run, want=t[1])

# Simple
want = """1
2"""
test("Very simple", "cat testdata/simple.json | ./jqgo 'a'", want=want)
    
print(f"{tests - failures} of {tests} succeeded.")
if failures > 0:
    sys.exit(1)
