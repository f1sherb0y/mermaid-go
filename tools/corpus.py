"""Copies every ```mermaid-example block of mermaid's syntax docs into
testdata/corpus/<page>/<nn>.mmd. Usage: python tools/corpus.py <mermaid repo>."""
import os, re, sys, shutil

repo = sys.argv[1]
docs = os.path.join(repo, "docs", "syntax")
out = os.path.join(os.path.dirname(__file__), "..", "testdata", "corpus")
shutil.rmtree(out, ignore_errors=True)
total = 0
for name in sorted(os.listdir(docs)):
    if not name.endswith(".md"):
        continue
    text = open(os.path.join(docs, name), encoding="utf-8").read().replace("\r\n", "\n")
    blocks = re.findall(r"^```mermaid-example\n(.*?)^```", text, re.S | re.M)
    seen = set()
    page = name[:-3]
    for b in blocks:
        if b in seen:
            continue
        seen.add(b)
        d = os.path.join(out, page)
        os.makedirs(d, exist_ok=True)
        open(os.path.join(d, "%02d.mmd" % len(seen)), "w", encoding="utf-8", newline="\n").write(b)
    total += len(seen)
    print(page, len(seen))
print("total", total)
