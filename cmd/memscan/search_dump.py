#!/usr/bin/env python3
"""Search Xbox RAM dump for UTF-16LE gamertag names."""
import struct, sys

data = open(sys.argv[1], "rb").read()
for name in ["uno", "dos", "tres"]:
    pat = name.encode("utf-16-le")
    off = 0
    while True:
        idx = data.find(pat, off)
        if idx == -1:
            break
        gva = 0x80000000 + idx
        u16s = []
        for i in range(0, 32, 2):
            v = struct.unpack_from("<H", data, idx + i)[0]
            if v == 0:
                break
            u16s.append(v)
        decoded = "".join(chr(c) for c in u16s if 0x20 <= c <= 0x7E)
        print(f"{decoded:>8s}  GVA 0x{gva:08X}")
        off = idx + 2
