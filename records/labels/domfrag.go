package labels

// mangles the given name in order to produce a valid domain fragment.
// a valid domain fragment will consist of one or more dns952 labels
// concatenated by a '.' char.
func AsDomainFrag(name string) string {
	if name == "" {
		return ""
	}
	sz := len(name)
	frag := make([]byte, sz, sz)
	ll := 0  // overall fragment length so far
	li := -1 // last fragment we found ended here
	for i, c := range name {
		if c == '.' {
			if f := AsDNS952(name[li+1 : i]); f != "" {
				if li > -1 {
					frag[ll] = '.'
					ll++
				}
				// len(f) is <= len(slice-of-name)
				copy(frag[ll:], f)
				ll += len(f)
				li = i
			}
		}
	}
	// final frag
	if f := AsDNS952(name[li+1:]); f != "" {
		if li > -1 {
			frag[ll] = '.'
			ll++
		}
		copy(frag[ll:], f)
		ll += len(f)
	}
	if ll > 0 {
		return string(frag[:ll])
	}
	return ""
}
