package exchanger

import (
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	. "github.com/mesosphere/mesos-dns/dnstest"
	"github.com/miekg/dns"
)

func TestWhile(t *testing.T) {
	for i, tt := range []struct {
		pred Pred
		exs  []Exchanger
		want exchanged
	}{
		{ // error
			nil,
			stubs(exchanged{nil, 0, errors.New("foo")}),
			exchanged{nil, 0, errors.New("foo")},
		},
		{ // always true predicate
			func(*dns.Msg) bool { return true },
			stubs(exchanged{nil, 0, nil}, exchanged{nil, 1, nil}),
			exchanged{nil, 1, nil},
		},
		{ // nil exchangers
			nil,
			nil,
			exchanged{nil, 0, nil},
		},
		{ // empty exchangers
			nil,
			stubs(),
			exchanged{nil, 0, nil},
		},
		{ // false predicate
			func(calls int) Pred {
				return func(*dns.Msg) bool {
					calls++
					return calls != 2
				}
			}(0),
			stubs(exchanged{nil, 0, nil}, exchanged{nil, 1, nil}, exchanged{nil, 2, nil}),
			exchanged{nil, 1, nil},
		},
	} {
		var got exchanged
		got.m, got.rtt, got.err = While(tt.pred, tt.exs...).Exchange(nil, "")
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test #%d: got: %v, want: %v", i, got, tt.want)
		}
	}
}

func TestRecurse(t *testing.T) {
	for i, tt := range []struct {
		*dns.Msg
		want string
	}{
		{ // Authoritative with answers
			Message(
				Header(true, 0),
				Answers(
					A(RRHeader("localhost", dns.TypeA, 0), net.IPv6loopback.To4()),
				),
				NSs(
					SOA(RRHeader("", dns.TypeSOA, 0), "next", "", 0),
				),
			),
			"",
		},
		{ // Authoritative, empty answers, no SOA records
			Message(
				Header(true, 0),
				NSs(
					NS(RRHeader("", dns.TypeNS, 0), "next"),
				),
			),
			"",
		},
		{ // Not authoritative, no SOA record
			Message(Header(false, 0)),
			"",
		},
		{ // Not authoritative, one SOA record
			Message(
				Header(false, 0),
				NSs(SOA(RRHeader("", dns.TypeSOA, 0), "next", "", 0)),
			),
			"next:53",
		},
		{ // Authoritative, empty answers, one SOA record
			Message(
				Header(true, 0),
				NSs(
					NS(RRHeader("", dns.TypeNS, 0), "foo"),
					SOA(RRHeader("", dns.TypeSOA, 0), "next", "", 0),
				),
			),
			"next:53",
		},
	} {
		if got := Recurse(tt.Msg); got != tt.want {
			t.Errorf("test #%d: got: %v, want: %v", i, got, tt.want)
		}
	}
}

func TestRecursion(t *testing.T) {
	for i, tt := range []struct {
		max  int
		rec  Recurser
		ex   Exchanger
		want exchanged
	}{
		{
			0,
			func(*dns.Msg) string { return "next" },
			seq(stubs(exchanged{rtt: 1})...),
			exchanged{rtt: 1},
		},
		{
			1,
			func(*dns.Msg) string { return "next" },
			seq(stubs(exchanged{rtt: 0}, exchanged{rtt: 1}, exchanged{rtt: 2})...),
			exchanged{rtt: 1},
		},
		{
			0,
			nil,
			seq(stubs(exchanged{err: errors.New("foo")})...),
			exchanged{err: errors.New("foo")},
		},
		{
			2,
			func(calls int) Recurser {
				return func(*dns.Msg) string {
					if calls++; calls <= 1 {
						return "next"
					}
					return ""
				}
			}(0),
			seq(stubs(exchanged{rtt: 0}, exchanged{rtt: 1}, exchanged{rtt: 2})...),
			exchanged{rtt: 1},
		},
	} {
		var got exchanged
		got.m, got.rtt, got.err = Recursion(tt.max, tt.rec)(tt.ex).Exchange(nil, "")
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test #%d: got: %v, want: %v", i, got, tt.want)
		}
	}
}

func seq(exs ...Exchanger) Exchanger {
	var i int
	return Func(func(m *dns.Msg, a string) (*dns.Msg, time.Duration, error) {
		ex := exs[i]
		i++
		return ex.Exchange(m, a)
	})
}

func stubs(ed ...exchanged) []Exchanger {
	exs := make([]Exchanger, len(ed))
	for i := range ed {
		exs[i] = stub(ed[i])
	}
	return exs
}

func stub(e exchanged) Exchanger {
	return Func(func(*dns.Msg, string) (*dns.Msg, time.Duration, error) {
		return e.m, e.rtt, e.err
	})
}

type exchanged struct {
	m   *dns.Msg
	rtt time.Duration
	err error
}
