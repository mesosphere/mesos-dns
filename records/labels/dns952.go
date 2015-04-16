package labels

const DNS952MaxLength int = 24

var dns952table []int32

func init() {
	const tolower = int32('a' - 'A')
	dns952table = make([]int32, 256, 256)
	for i := int32('A'); i < int32('Z'); i++ {
		dns952table[i] = i + tolower
	}
	for i := int32('a'); i < int32('z'); i++ {
		dns952table[i] = i
	}
	for i := int32('0'); i < int32('9'); i++ {
		dns952table[i] = -i
	}
	dns952table[int32('-')] = -int32('-')
	dns952table[int32('.')] = -int32('-')
	dns952table[int32('_')] = -int32('-')
}

// mangle the given name to be compliant as a DNS952 name "component".
// the returned result should match the regexp:
//    ^[a-z]([-a-z0-9]*[a-z0-9])?$
// returns "" if the name cannot be mangled.
func AsDNS952(name string) string {
	if name == "" {
		return ""
	}
	sz := len(name)
	if sz > DNS952MaxLength {
		sz = DNS952MaxLength
	}
	last := sz - 1
	label := make([]byte, sz, sz)
	ll := 0
	la := -1 // index of last alphanumeric
	for _, c := range name {
		b := dns952table[uint8(c)]
		switch {
		case b == -int32('-'):
			if ll == 0 || ll == last {
				continue
			}
			b = -b
		case b < 0:
			if ll == 0 {
				continue
			}
			b = -b
			la = ll
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
