package tap

import (
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type Sink interface {
	Write(*Dnstap)
}

type SinkFunc func(*Dnstap)

func (sf SinkFunc) Write(x *Dnstap) {
	sf(x)
}

// Tap abstracts DNS introspection
type Tap struct {
	sink Sink
}

type TapContext struct {
	tap          *Tap
	msg          *Message
	responseType Message_Type
}

type WrappedWriter struct {
	*TapContext
	dns.ResponseWriter
}

func (w *WrappedWriter) Write(buf []byte) (int, error) {
	w.TapContext.WriteResponse(buf)
	return w.ResponseWriter.Write(buf)
}

func (w *WrappedWriter) WriteMsg(m *dns.Msg) error {
	//TODO(jdef) terrible for performance! would be nice to have better access to this raw buffer from inside ResponseWriter
	data, err := m.Pack()
	if err != nil {
		log.Println("ERROR:", err.Error())
	} else {
		w.TapContext.WriteResponse(data)
	}
	return w.ResponseWriter.WriteMsg(m)
}

func NewTap(sink Sink) *Tap {
	t := &Tap{
		sink: sink,
	}
	return t
}

func (t *Tap) ClientSpike(s *dns.Server, h dns.Handler) dns.Handler {
	var protocol *SocketProtocol
	switch s.Net {
	case "tcp", "tcp4", "tcp6":
		protocol = SocketProtocol_TCP.Enum()
	case "udp", "udp4", "udp6":
		protocol = SocketProtocol_UDP.Enum()
	}

	var parseResponseAddrOnce sync.Once
	return dns.HandlerFunc(func(w dns.ResponseWriter, m *dns.Msg) {
		var responseAddr []byte
		var responsePort uint32
		parseResponseAddrOnce.Do(func() {
			host, port, err := net.SplitHostPort(w.LocalAddr().String())
			if err != nil {
				return
			}
			if iport, err := strconv.Atoi(port); err != nil {
				responsePort = uint32(iport)
			}
			if ip := net.ParseIP(host); ip != nil {
				responseAddr = ([]byte)(ip)
			}
		})
		ctx := t.Begin(w, m, Message_CLIENT_QUERY, protocol, responseAddr, responsePort)
		wrapped := &WrappedWriter{
			ResponseWriter: w,
			TapContext:     ctx,
		}
		h.ServeDNS(wrapped, m)
	})
}

func (t *Tap) Begin(w dns.ResponseWriter, m *dns.Msg, qtype Message_Type, proto *SocketProtocol, resAddr []byte, resPort uint32) *TapContext {
	now := time.Now()
	clientAddr := w.RemoteAddr().String()
	_, fam := parseAddr(clientAddr)
	packed, _ := m.Pack() // TODO(jdef) this is terrible, we need to intercept this some other way?!
	qsec := uint64(now.Unix())
	qnsec := uint32(now.Nanosecond())
	tapmsg := &Dnstap{
		Type: Dnstap_MESSAGE.Enum(),
		Message: &Message{
			Type:           &qtype,
			SocketFamily:   fam,
			SocketProtocol: proto,
			QueryMessage:   packed,
			QueryTimeSec:   &qsec,
			QueryTimeNsec:  &qnsec,
		},
	}
	if resPort != 0 {
		tapmsg.Message.ResponsePort = &resPort
	}
	if resAddr != nil {
		tapmsg.Message.ResponseAddress = resAddr
	}
	ctx := &TapContext{
		tap:          t,
		msg:          tapmsg.Message,
		responseType: Message_CLIENT_RESPONSE,
	}
	t.log(tapmsg)
	return ctx
}

func (t *TapContext) WriteResponse(packed []byte) {
	now := time.Now()
	qsec := uint64(now.Unix())
	qnsec := uint32(now.Nanosecond())
	t.tap.log(&Dnstap{
		Type: Dnstap_MESSAGE.Enum(),
		Message: &Message{
			Type:             &t.responseType,
			SocketFamily:     t.msg.SocketFamily,
			SocketProtocol:   t.msg.SocketProtocol,
			QueryTimeSec:     t.msg.QueryTimeSec,
			QueryTimeNsec:    t.msg.QueryTimeNsec,
			ResponseAddress:  t.msg.ResponseAddress,
			ResponsePort:     t.msg.ResponsePort,
			ResponseMessage:  packed,
			ResponseTimeSec:  &qsec,
			ResponseTimeNsec: &qnsec,
		},
	})
}

func parseAddr(addr string) (ip net.IP, family *SocketFamily) {
	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		ip = net.ParseIP(host)
		if ip != nil {
			if ipv4 := ip.To4(); ipv4 != nil {
				family = SocketFamily_INET.Enum()
			} else if ipv6 := ip.To16(); ipv6 != nil {
				family = SocketFamily_INET6.Enum()
			}
		}
	}
	return
}

func (t *Tap) log(m *Dnstap) {
	t.sink.Write(m)
}
