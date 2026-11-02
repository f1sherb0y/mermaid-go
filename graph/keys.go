package graph

import (
	"cmp"
	"slices"
	"unicode/utf8"
)

// graphlib keeps nodes, edges, parents and children in plain JS objects, so
// every listing it returns follows JS property order: keys that are array
// indices ("0", "17", never "017" or "-1") first, ascending by value, then all
// other keys in insertion order. Deleting a key and adding it again moves a
// non-index key to the end. dagre's results depend on this order, so the
// containers below reproduce it.

// arrayIndex reports whether s is a canonical JS array index (0 to 2^32-2)
// and returns its value; -1 otherwise.
func arrayIndex(s string) int64 {
	if len(s) == 0 || len(s) > 10 || (s[0] == '0' && len(s) > 1) {
		return -1
	}
	var n int64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int64(c-'0')
	}
	if n > 1<<32-2 {
		return -1
	}
	return n
}

// Less compares strings the way JS's < does: by UTF-16 code units. It differs
// from Go's < only between a supplementary character and one in U+E000-U+FFFF.
func Less(a, b string) bool {
	for i := 0; i < len(a) && i < len(b); {
		if a[i] == b[i] {
			i++
			continue
		}
		if a[i] < utf8.RuneSelf || b[i] < utf8.RuneSelf {
			return a[i] < b[i]
		}
		// Back up to the start of the differing runes.
		j := i
		for j > 0 && !utf8.RuneStart(a[j]) {
			j--
		}
		ra, _ := utf8.DecodeRuneInString(a[j:])
		rb, _ := utf8.DecodeRuneInString(b[j:])
		return utf16First(ra) < utf16First(rb) ||
			utf16First(ra) == utf16First(rb) && ra < rb
	}
	return len(a) < len(b)
}

// utf16First is the first UTF-16 code unit of r.
func utf16First(r rune) rune {
	if r >= 0x10000 {
		return 0xD800 + (r-0x10000)>>10
	}
	return r
}

// keySet is an ordered set of nodes in JS property order, with a count per
// node (graphlib counts parallel edges in its predecessor and successor maps).
// The first nIdx entries are the array-index keys, sorted.
type keySet[N, E any] struct {
	items []keyCount[N, E]
	nIdx  int
}

type keyCount[N, E any] struct {
	n *node[N, E]
	c int32
}

func (s *keySet[N, E]) find(n *node[N, E]) int {
	// Scan from the end: the key looked for is most often the one just added.
	for i := len(s.items) - 1; i >= 0; i-- {
		if s.items[i].n == n {
			return i
		}
	}
	return -1
}

// add inserts n, which must be absent, with count 1.
func (s *keySet[N, E]) add(n *node[N, E]) {
	if n.idx < 0 {
		s.items = append(s.items, keyCount[N, E]{n, 1})
		return
	}
	i := 0
	for i < s.nIdx && s.items[i].n.idx < n.idx {
		i++
	}
	s.items = append(s.items, keyCount[N, E]{})
	copy(s.items[i+1:], s.items[i:])
	s.items[i] = keyCount[N, E]{n, 1}
	s.nIdx++
}

func (s *keySet[N, E]) removeAt(i int) {
	if i < s.nIdx {
		s.nIdx--
	}
	s.items = append(s.items[:i], s.items[i+1:]...)
}

func (s *keySet[N, E]) remove(n *node[N, E]) {
	if i := s.find(n); i >= 0 {
		s.removeAt(i)
	}
}

func (s *keySet[N, E]) inc(n *node[N, E]) {
	if i := s.find(n); i >= 0 {
		s.items[i].c++
	} else {
		s.add(n)
	}
}

func (s *keySet[N, E]) dec(n *node[N, E]) {
	if i := s.find(n); i >= 0 {
		if s.items[i].c--; s.items[i].c == 0 {
			s.removeAt(i)
		}
	}
}

func (s *keySet[N, E]) ids() []string {
	out := make([]string, len(s.items))
	for i, it := range s.items {
		out[i] = it.n.id
	}
	return out
}

// nodeOrder is the graph's node list in JS property order: index keys sorted
// in idx, other keys in insertion order in strs (removed ones left as nil and
// compacted now and then).
type nodeOrder[N, E any] struct {
	idx  []*node[N, E]
	strs []*node[N, E]
	dead int
}

func (o *nodeOrder[N, E]) add(n *node[N, E]) {
	if n.idx < 0 {
		n.seq = len(o.strs)
		o.strs = append(o.strs, n)
		return
	}
	i := len(o.idx)
	for i > 0 && o.idx[i-1].idx > n.idx {
		i--
	}
	o.idx = append(o.idx, nil)
	copy(o.idx[i+1:], o.idx[i:])
	o.idx[i] = n
}

func (o *nodeOrder[N, E]) remove(n *node[N, E]) {
	if n.idx >= 0 {
		for i, m := range o.idx {
			if m == n {
				o.idx = append(o.idx[:i], o.idx[i+1:]...)
				return
			}
		}
		return
	}
	o.strs[n.seq] = nil
	o.dead++
	if o.dead > 32 && o.dead > len(o.strs)/2 {
		k := 0
		for _, m := range o.strs {
			if m != nil {
				m.seq = k
				o.strs[k] = m
				k++
			}
		}
		clear(o.strs[k:])
		o.strs = o.strs[:k]
		o.dead = 0
	}
}

func (o *nodeOrder[N, E]) each(fn func(n *node[N, E])) {
	for _, n := range o.idx {
		fn(n)
	}
	for _, n := range o.strs {
		if n != nil {
			fn(n)
		}
	}
}

// KeyOrder returns keys, given in insertion order and without duplicates,
// in the order a JS object built from them lists them.
func KeyOrder(keys []string) []string {
	type ik struct {
		i int64
		k string
	}
	var idx []ik
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if i := arrayIndex(k); i >= 0 {
			idx = append(idx, ik{i, k})
		}
	}
	if len(idx) == 0 {
		return append(out, keys...)
	}
	slices.SortFunc(idx, func(a, b ik) int { return cmp.Compare(a.i, b.i) })
	for _, x := range idx {
		out = append(out, x.k)
	}
	for _, k := range keys {
		if arrayIndex(k) < 0 {
			out = append(out, k)
		}
	}
	return out
}
