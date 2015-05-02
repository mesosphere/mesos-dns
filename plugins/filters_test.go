package plugins

import (
	"testing"

	"github.com/miekg/dns"
)

func TestFilters_Empty(t *testing.T) {
	invoked := false
	h := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		invoked = true
	})
	filtered := FilterSet(nil).Handler(h)
	filtered.ServeDNS(nil, nil)
	if !invoked {
		t.Fatalf("end of filter chain not invoked")
	}
}

func TestFilters_Single(t *testing.T) {
	invoked := false
	h := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		invoked = true
	})

	var filters FilterSet
	filtered := false
	filters = append(filters, FilterFunc(func(w dns.ResponseWriter, r *dns.Msg, chain dns.Handler) {
		filtered = true
		if invoked {
			t.Fatalf("end of chain already invoked")
		}
		// continue processing filters
		chain.ServeDNS(w, r)
	}))

	filteredHandler := filters.Handler(h)
	filteredHandler.ServeDNS(nil, nil)

	if !filtered {
		t.Fatalf("filter not invoked")
	}
	if !invoked {
		t.Fatalf("end of filter chain not invoked")
	}
}

func TestFilters_SingleAbortive(t *testing.T) {
	invoked := false
	h := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		invoked = true
	})

	var filters FilterSet
	filtered := false
	filters = append(filters, FilterFunc(func(w dns.ResponseWriter, r *dns.Msg, chain dns.Handler) {
		filtered = true
		if invoked {
			t.Fatalf("end of chain already invoked")
		}
		// don't invoke chain, abort processing
	}))

	filteredHandler := filters.Handler(h)
	filteredHandler.ServeDNS(nil, nil)

	if !filtered {
		t.Fatalf("filter not invoked")
	}
	if invoked {
		t.Fatalf("end of filter chain invoked")
	}
}

func TestFilters_Multi(t *testing.T) {
	invoked := false
	h := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		invoked = true
	})

	var filters FilterSet
	filtered := []bool{false, false, false}
	for k, _ := range filtered {
		i := k // don't use a loop var in a callback func
		filters = append(filters, FilterFunc(func(w dns.ResponseWriter, r *dns.Msg, chain dns.Handler) {
			filtered[i] = true
			if invoked {
				t.Fatalf("end of chain already invoked")
			}

			// test LIFO algorithm: "lower" filters should not have been invoked
			for j := 0; j < i; j++ {
				if filtered[j] {
					t.Fatalf("filter invoked out of order")
				}
			}

			// continue processing filters
			chain.ServeDNS(w, r)
		}))
	}

	filteredHandler := filters.Handler(h)
	filteredHandler.ServeDNS(nil, nil)

	for i, f := range filtered {
		if !f {
			t.Fatalf("filter %d not invoked", i)
		}
	}
	if !invoked {
		t.Fatalf("end of filter chain not invoked")
	}
}

func TestFilters_MultiAbortive(t *testing.T) {
	invoked := false
	h := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		invoked = true
	})

	var filters FilterSet
	filtered := []bool{false, false, false}
	for k, _ := range filtered {
		i := k // don't use a loop var in a callback func
		filters = append(filters, FilterFunc(func(w dns.ResponseWriter, r *dns.Msg, chain dns.Handler) {
			filtered[i] = true
			if invoked {
				t.Fatalf("end of chain already invoked")
			}

			// test LIFO algorithm: "lower" filters should not have been invoked
			for j := 0; j < i; j++ {
				if filtered[j] {
					t.Fatalf("filter invoked out of order")
				}
			}

			// abort when i == 1
			if i != 1 {
				chain.ServeDNS(w, r)
			}
		}))
	}

	filteredHandler := filters.Handler(h)
	filteredHandler.ServeDNS(nil, nil)

	if !filtered[2] {
		t.Fatalf("filter 2 not invoked")
	}
	if !filtered[1] {
		t.Fatalf("filter 1 not invoked")
	}
	if filtered[0] {
		t.Fatalf("filter 0 invoked")
	}
	if invoked {
		t.Fatalf("end of filter chain invoked")
	}
}
