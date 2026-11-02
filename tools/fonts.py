"""Makes the font mermaid.live shows: Recursive Sans Linear, from Google Fonts'
Recursive variable font (OFL), pinned to MONO 0, CASL 0, slnt 0, CRSV 0.5; the
wght axis stays variable. Usage: python tools/fonts.py <out dir>."""
import os, sys, urllib.request
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer

out = sys.argv[1]
os.makedirs(out, exist_ok=True)
base = "https://raw.githubusercontent.com/google/fonts/main/ofl/recursive/"
vf = os.path.join(out, "RecursiveVF.ttf")
urllib.request.urlretrieve(base + "Recursive%5BCASL%2CCRSV%2CMONO%2Cslnt%2Cwght%5D.ttf", vf)
urllib.request.urlretrieve(base + "OFL.txt", os.path.join(out, "OFL.txt"))
f = instancer.instantiateVariableFont(TTFont(vf), {"MONO": 0, "CASL": 0, "slnt": 0, "CRSV": 0.5})
f.save(os.path.join(out, "RecursiveSansLinear-VF.ttf"))
