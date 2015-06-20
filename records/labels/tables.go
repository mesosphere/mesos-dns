package labels

type dnsCharTable []int32

const replacementRune = -int32('-')

// makeDNS952Table initializes a character lookup table that corresponds to RFC952
// host-label naming conventions. Characters that may not be present as the leading
// character of a label are modeled as non-positive.
func makeDNS952Table() dnsCharTable {
	const tolower = int32('a' - 'A')
	tab := dnsCharTable(make([]int32, 256, 256))
	for i := int32('A'); i <= int32('Z'); i++ {
		tab[i] = i + tolower
	}
	for i := int32('a'); i <= int32('z'); i++ {
		tab[i] = i
	}
	for i := int32('0'); i <= int32('9'); i++ {
		tab[i] = -i
	}
	tab[int32('-')] = replacementRune
	tab[int32('.')] = replacementRune
	tab[int32('_')] = replacementRune
	return tab
}

// makeDNS1123Table initializes a character lookup table that corresponds to RFC1123
// host-label naming conventions; the only difference from dns932table is that digits
// are allowed at the beginning of a label (so they are positive here).
func makeDNS1123Table() dnsCharTable {
	const tolower = int32('a' - 'A')
	tab := dnsCharTable(make([]int32, 256, 256))
	for i := int32('A'); i <= int32('Z'); i++ {
		tab[i] = i + tolower
	}
	for i := int32('a'); i <= int32('z'); i++ {
		tab[i] = i
	}
	for i := int32('0'); i <= int32('9'); i++ {
		tab[i] = i
	}
	tab[int32('-')] = replacementRune
	tab[int32('.')] = replacementRune
	tab[int32('_')] = replacementRune
	return tab
}

// mangle the given name according to this translation table.
// returns "" if the name cannot be mangled.
func (tab dnsCharTable) toLabel(name string, maxlen int) string {
	if name == "" {
		return ""
	}
	sz := len(name)
	if sz > maxlen {
		sz = maxlen
	}
	last := sz - 1
	label := make([]byte, sz, sz)
	ll := 0
	la := -1 // index of last alphanumeric
	for _, c := range name {
		b := tab[uint8(c)]
		switch {
		case b < 0:
			if b == replacementRune {
				if ll == 0 || ll == last {
					continue
				}
				b = -b
				break
			}
			if ll == 0 {
				continue
			}
			b = -b
			fallthrough
		case b > 0:
			la = ll
		default:
			continue
		}
		label[ll] = byte(b)
		ll++
		if ll == sz {
			break
		}
	}
	if ll > 0 && label[ll-1] == '-' {
		ll = la + 1
	}
	if ll > 0 {
		return string(label[:ll])
	}
	return ""
}
