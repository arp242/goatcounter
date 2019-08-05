#!/usr/bin/env python3
#
# Convert X'..' to regular strings.


import binascii
import re
import sys


print(re.sub(
    r'X\'([0-9a-f]*)\'',
    lambda m: "'" + binascii.unhexlify(m.group(1)).decode() + "'",
    sys.stdin.buffer.read().decode()), end='')
