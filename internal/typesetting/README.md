# go-text/typesetting, patched

A copy of [github.com/go-text/typesetting](https://github.com/go-text/typesetting)
v0.3.5 (BSD-3-Clause / Unlicense, see LICENSE): its Go sources only, without
tests, test data and `testutils`.

One change, in `harfbuzz/set_digest.go`: `setDigest.addRange` returned early
as soon as any of its three filters was full, so the other two missed the
rest of the coverage. A lookup whose coverage fills one filter (Recursive's
kerning, for one) was then skipped for glyphs it covers: "Wo" got no kerning.
HarfBuzz fills each filter on its own. The early return is removed (adding to
a full filter changes nothing).

To be offered upstream; drop this copy once a release has the fix.
