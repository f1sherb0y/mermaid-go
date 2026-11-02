package dagre

// jsSort is V8's Array.prototype.sort (TimSort, third_party/v8/builtins/
// array-sort.tq). With a consistent comparator any stable sort gives the
// same result; dagre's barycenter comparator is inconsistent once a
// barycenter is NaN, and then only the same algorithm gives the same order.
func jsSort[T any](a []T, cmp func(x, y T) int) {
	n := len(a)
	if n < 2 {
		return
	}
	s := &timSort[T]{a: a, cmp: cmp, minGallop: 7}
	minRun := minRunLength(n)
	low, remaining := 0, n
	for remaining != 0 {
		run := s.countAndMakeRun(low, low+remaining)
		if run < minRun {
			forced := min(minRun, remaining)
			s.binaryInsertionSort(low, low+run, low+forced)
			run = forced
		}
		s.runs = append(s.runs, tsRun{low, run})
		s.mergeCollapse()
		low += run
		remaining -= run
	}
	s.mergeForceCollapse()
}

// sortByNumber is lodash's _.sortBy on numbers: NaN goes last (comparing
// greater than anything, itself included, which is inconsistent, hence V8's
// sort), ties keep their order.
func sortByNumber(a []float64) {
	type item struct {
		x float64
		i int
	}
	items := make([]item, len(a))
	for i, x := range a {
		items[i] = item{x, i}
	}
	jsSort(items, func(p, q item) int {
		switch {
		case p.x == q.x:
			return p.i - q.i
		case p.x > q.x || p.x != p.x:
			return 1
		case p.x < q.x || q.x != q.x:
			return -1
		}
		return p.i - q.i
	})
	for i, it := range items {
		a[i] = it.x
	}
}

const minGallopWins = 7

type tsRun struct{ base, length int }

type timSort[T any] struct {
	a         []T
	cmp       func(x, y T) int
	minGallop int
	runs      []tsRun
	tmp       []T
}

func minRunLength(n int) int {
	r := 0
	for n >= 64 {
		r |= n & 1
		n >>= 1
	}
	return n + r
}

// countAndMakeRun returns the length of the run at low, reversing it if it
// is strictly descending.
func (s *timSort[T]) countAndMakeRun(lowArg, high int) int {
	low := lowArg + 1
	if low == high {
		return 1
	}
	run := 2
	prev := s.a[low]
	desc := s.cmp(prev, s.a[low-1]) < 0
	for i := low + 1; i < high; i++ {
		cur := s.a[i]
		order := s.cmp(cur, prev)
		if desc && order >= 0 || !desc && order < 0 {
			break
		}
		prev = cur
		run++
	}
	if desc {
		for i, j := lowArg, lowArg+run-1; i < j; i, j = i+1, j-1 {
			s.a[i], s.a[j] = s.a[j], s.a[i]
		}
	}
	return run
}

func (s *timSort[T]) binaryInsertionSort(low, start, high int) {
	if low == start {
		start++
	}
	for ; start < high; start++ {
		left, right := low, start
		pivot := s.a[start]
		for left < right {
			mid := left + (right-left)>>1
			if s.cmp(pivot, s.a[mid]) < 0 {
				right = mid
			} else {
				left = mid + 1
			}
		}
		copy(s.a[left+1:start+1], s.a[left:start])
		s.a[left] = pivot
	}
}

func (s *timSort[T]) runInvariant(n int) bool {
	if n < 2 {
		return true
	}
	return s.runs[n-2].length > s.runs[n-1].length+s.runs[n].length
}

func (s *timSort[T]) mergeCollapse() {
	for len(s.runs) > 1 {
		n := len(s.runs) - 2
		if !s.runInvariant(n+1) || !s.runInvariant(n) {
			if s.runs[n-1].length < s.runs[n+1].length {
				n--
			}
			s.mergeAt(n)
		} else if s.runs[n].length <= s.runs[n+1].length {
			s.mergeAt(n)
		} else {
			break
		}
	}
}

func (s *timSort[T]) mergeForceCollapse() {
	for len(s.runs) > 1 {
		n := len(s.runs) - 2
		if n > 0 && s.runs[n-1].length < s.runs[n+1].length {
			n--
		}
		s.mergeAt(n)
	}
}

func (s *timSort[T]) mergeAt(i int) {
	baseA, lenA := s.runs[i].base, s.runs[i].length
	baseB, lenB := s.runs[i+1].base, s.runs[i+1].length
	s.runs[i].length = lenA + lenB
	s.runs = append(s.runs[:i+1], s.runs[i+2:]...)

	// Elements of A already in place before B's first can be skipped...
	k := s.gallopRight(s.a, s.a[baseB], baseA, lenA, 0)
	baseA += k
	lenA -= k
	if lenA == 0 {
		return
	}
	// ...as can those of B after A's last.
	lenB = s.gallopLeft(s.a, s.a[baseA+lenA-1], baseB, lenB, lenB-1)
	if lenB == 0 {
		return
	}
	if lenA <= lenB {
		s.mergeLow(baseA, lenA, baseB, lenB)
	} else {
		s.mergeHigh(baseA, lenA, baseB, lenB)
	}
}

// gallopLeft returns where key goes in arr[base:base+length], left of any
// equal elements, searching from hint.
func (s *timSort[T]) gallopLeft(arr []T, key T, base, length, hint int) int {
	lastOfs, offset := 0, 1
	if s.cmp(arr[base+hint], key) < 0 {
		maxOfs := length - hint
		for offset < maxOfs && s.cmp(arr[base+hint+offset], key) < 0 {
			lastOfs = offset
			offset = offset<<1 + 1
		}
		offset = min(offset, maxOfs)
		lastOfs += hint
		offset += hint
	} else {
		maxOfs := hint + 1
		for offset < maxOfs && s.cmp(arr[base+hint-offset], key) >= 0 {
			lastOfs = offset
			offset = offset<<1 + 1
		}
		offset = min(offset, maxOfs)
		lastOfs, offset = hint-offset, hint-lastOfs
	}
	lastOfs++
	for lastOfs < offset {
		m := lastOfs + (offset-lastOfs)>>1
		if s.cmp(arr[base+m], key) < 0 {
			lastOfs = m + 1
		} else {
			offset = m
		}
	}
	return offset
}

// gallopRight is gallopLeft going right of equal elements.
func (s *timSort[T]) gallopRight(arr []T, key T, base, length, hint int) int {
	lastOfs, offset := 0, 1
	if s.cmp(key, arr[base+hint]) < 0 {
		maxOfs := hint + 1
		for offset < maxOfs && s.cmp(key, arr[base+hint-offset]) < 0 {
			lastOfs = offset
			offset = offset<<1 + 1
		}
		offset = min(offset, maxOfs)
		lastOfs, offset = hint-offset, hint-lastOfs
	} else {
		maxOfs := length - hint
		for offset < maxOfs && s.cmp(key, arr[base+hint+offset]) >= 0 {
			lastOfs = offset
			offset = offset<<1 + 1
		}
		offset = min(offset, maxOfs)
		lastOfs += hint
		offset += hint
	}
	lastOfs++
	for lastOfs < offset {
		m := lastOfs + (offset-lastOfs)>>1
		if s.cmp(key, arr[base+m]) < 0 {
			offset = m
		} else {
			lastOfs = m + 1
		}
	}
	return offset
}

func (s *timSort[T]) mergeLow(baseA, lenA, baseB, lenB int) {
	a := s.a
	tmp := append(s.tmp[:0], a[baseA:baseA+lenA]...)
	s.tmp = tmp
	dest, ct, cb := baseA, 0, baseB
	a[dest] = a[cb]
	dest++
	cb++

	finish := func() { // Succeed
		if lenA > 0 {
			copy(a[dest:dest+lenA], tmp[ct:ct+lenA])
		}
	}
	copyB := func() { // the last element of A belongs at the end
		copy(a[dest:dest+lenB], a[cb:cb+lenB])
		a[dest+lenB] = tmp[ct]
	}

	if lenB--; lenB == 0 {
		finish()
		return
	}
	if lenA == 1 {
		copyB()
		return
	}
	minGallop := s.minGallop
	for {
		winsA, winsB := 0, 0
		for {
			if s.cmp(a[cb], tmp[ct]) < 0 {
				a[dest] = a[cb]
				dest++
				cb++
				winsB++
				lenB--
				winsA = 0
				if lenB == 0 {
					finish()
					return
				}
				if winsB >= minGallop {
					break
				}
			} else {
				a[dest] = tmp[ct]
				dest++
				ct++
				winsA++
				lenA--
				winsB = 0
				if lenA == 1 {
					copyB()
					return
				}
				if winsA >= minGallop {
					break
				}
			}
		}
		minGallop++
		for first := true; first || winsA >= minGallopWins || winsB >= minGallopWins; first = false {
			minGallop = max(1, minGallop-1)
			s.minGallop = minGallop
			winsA = s.gallopRight(tmp, a[cb], ct, lenA, 0)
			if winsA > 0 {
				copy(a[dest:dest+winsA], tmp[ct:ct+winsA])
				dest += winsA
				ct += winsA
				lenA -= winsA
				if lenA == 1 {
					copyB()
					return
				}
				if lenA == 0 {
					finish()
					return
				}
			}
			a[dest] = a[cb]
			dest++
			cb++
			if lenB--; lenB == 0 {
				finish()
				return
			}
			winsB = s.gallopLeft(a, tmp[ct], cb, lenB, 0)
			if winsB > 0 {
				copy(a[dest:dest+winsB], a[cb:cb+winsB])
				dest += winsB
				cb += winsB
				lenB -= winsB
				if lenB == 0 {
					finish()
					return
				}
			}
			a[dest] = tmp[ct]
			dest++
			ct++
			if lenA--; lenA == 1 {
				copyB()
				return
			}
		}
		minGallop++
		s.minGallop = minGallop
	}
}

func (s *timSort[T]) mergeHigh(baseA, lenA, baseB, lenB int) {
	a := s.a
	tmp := append(s.tmp[:0], a[baseB:baseB+lenB]...)
	s.tmp = tmp
	dest, ct, ca := baseB+lenB-1, lenB-1, baseA+lenA-1
	a[dest] = a[ca]
	dest--
	ca--

	finish := func() { // Succeed
		if lenB > 0 {
			copy(a[dest-(lenB-1):dest+1], tmp[:lenB])
		}
	}
	copyA := func() { // the first element of B belongs at the front
		dest -= lenA
		ca -= lenA
		copy(a[dest+1:dest+1+lenA], a[ca+1:ca+1+lenA])
		a[dest] = tmp[ct]
	}

	if lenA--; lenA == 0 {
		finish()
		return
	}
	if lenB == 1 {
		copyA()
		return
	}
	minGallop := s.minGallop
	for {
		winsA, winsB := 0, 0
		for {
			if s.cmp(tmp[ct], a[ca]) < 0 {
				a[dest] = a[ca]
				dest--
				ca--
				winsA++
				lenA--
				winsB = 0
				if lenA == 0 {
					finish()
					return
				}
				if winsA >= minGallop {
					break
				}
			} else {
				a[dest] = tmp[ct]
				dest--
				ct--
				winsB++
				lenB--
				winsA = 0
				if lenB == 1 {
					copyA()
					return
				}
				if winsB >= minGallop {
					break
				}
			}
		}
		minGallop++
		for first := true; first || winsA >= minGallopWins || winsB >= minGallopWins; first = false {
			minGallop = max(1, minGallop-1)
			s.minGallop = minGallop
			k := s.gallopRight(a, tmp[ct], baseA, lenA, lenA-1)
			winsA = lenA - k
			if winsA > 0 {
				dest -= winsA
				ca -= winsA
				copy(a[dest+1:dest+1+winsA], a[ca+1:ca+1+winsA])
				lenA -= winsA
				if lenA == 0 {
					finish()
					return
				}
			}
			a[dest] = tmp[ct]
			dest--
			ct--
			if lenB--; lenB == 1 {
				copyA()
				return
			}
			k = s.gallopLeft(tmp, a[ca], 0, lenB, lenB-1)
			winsB = lenB - k
			if winsB > 0 {
				dest -= winsB
				ct -= winsB
				copy(a[dest+1:dest+1+winsB], tmp[ct+1:ct+1+winsB])
				lenB -= winsB
				if lenB == 1 {
					copyA()
					return
				}
				if lenB == 0 {
					finish()
					return
				}
			}
			a[dest] = a[ca]
			dest--
			ca--
			if lenA--; lenA == 0 {
				finish()
				return
			}
		}
		minGallop++
		s.minGallop = minGallop
	}
}
