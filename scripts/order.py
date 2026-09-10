#!/usr/bin/env python3
"""Order top-level declarations in Go files.

    scripts/order.py $(find internal cmd -name '*.go')     # rewrite
    scripts/order.py --check $(find internal cmd -name '*.go')

Order: constants, variables, exported types, unexported types, exported
functions, exported methods, unexported methods, unexported functions. Ties
keep source order, so related declarations stay adjacent.

Purely textual: a declaration block is its doc comment plus everything through
the closing brace at column 0, so comments and directives travel with what they
document.
"""
import re, sys, pathlib

def split(src):
    lines = src.split("\n")
    # header: everything up to and including the import block (or package line)
    i = 0
    while i < len(lines) and not lines[i].startswith("package "):
        i += 1
    i += 1
    end = i
    j = i
    while j < len(lines):
        if lines[j].startswith("import ("):
            while not lines[j].startswith(")"):
                j += 1
            end = j + 1
            break
        if lines[j].startswith("import "):
            end = j + 1
            break
        if re.match(r'^(func|type|var|const|//go:)', lines[j]):
            break
        j += 1
    header = "\n".join(lines[:end])

    blocks, cur = [], []
    k = end
    while k < len(lines):
        ln = lines[k]
        if re.match(r'^(func|type|var|const)\b', ln):
            body = [ln]
            if ln.rstrip().endswith("{") or ln.rstrip().endswith("("):
                closer = ")" if ln.rstrip().endswith("(") and not ln.startswith("func") else "}"
                while k + 1 < len(lines) and lines[k] != closer:
                    k += 1
                    body.append(lines[k])
            blocks.append(("\n".join(cur + body)).strip("\n"))
            cur = []
        elif ln.strip() == "":
            if cur and not "".join(cur).strip():
                cur = []
            elif cur:
                cur.append(ln)
        else:
            cur.append(ln)
        k += 1
    trailing = "\n".join(cur).strip("\n")
    return header, blocks, trailing

def rank(b):
    """consts, vars, exported types, unexported types, exported funcs,
    exported methods, unexported methods, unexported funcs."""
    decl = next(l for l in b.split("\n") if re.match(r'^(func|type|var|const)\b', l))
    m = re.match(r'^(func|type|var|const)\s+(\([^)]*\)\s*)?([A-Za-z_]\w*)?', decl)
    kind, recv, name = m.group(1), m.group(2), m.group(3) or ""
    if kind == "const": return 0
    if kind == "var":   return 1
    exported = name[:1].isupper()
    if kind == "type":  return 2 if exported else 3
    if recv:            return 5 if exported else 6   # methods
    return 4 if exported else 7                        # plain functions

def main(path):
    p = pathlib.Path(path)
    src = p.read_text()
    header, blocks, trailing = split(src)
    if not blocks:
        return False
    ordered = sorted(range(len(blocks)), key=lambda i: (rank(blocks[i]), i))
    if ordered == list(range(len(blocks))):
        return False
    out = header + "\n\n" + "\n\n".join(blocks[i] for i in ordered)
    if trailing:
        out += "\n\n" + trailing
    p.write_text(out.rstrip("\n") + "\n")
    return True

args = sys.argv[1:]
check = "--check" in args
files = [a for a in args if a != "--check"]

if check:
    bad = []
    for f in files:
        src = pathlib.Path(f).read_text()
        header, blocks, trailing = split(src)
        if blocks and sorted(range(len(blocks)), key=lambda i: (rank(blocks[i]), i)) != list(range(len(blocks))):
            bad.append(f)
    for f in bad:
        print(f"{f}: declarations out of order", file=sys.stderr)
    sys.exit(1 if bad else 0)

changed = [f for f in files if main(f)]
print(f"reordered {len(changed)} of {len(files)} files")
