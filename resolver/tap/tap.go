package tap

import (
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

type clientIO struct {
	tap           *Tap
	protocol      *SocketProtocol
	responseAddr  []byte
	responsePort  uint32
	parseAddrOnce sync.Once
}

type clientReader struct {
	clientIO
	in dns.Reader
}

type clientWriter struct {
	clientIO
	out dns.Writer
}

func (c *clientReader) ReadTCP(conn *net.TCPConn, timeout time.Duration) (m []byte, err error) {
	if m, err = c.in.ReadTCP(conn, timeout); err != nil {
		return
	}
	c.parseLocalAddr(conn.LocalAddr)
	c.logQuery(m, conn.RemoteAddr)
	return
}

func (c *clientReader) ReadUDP(conn *net.UDPConn, timeout time.Duration) (m []byte, s *dns.SessionUDP, err error) {
	if m, s, err = c.in.ReadUDP(conn, timeout); err != nil {
		return
	}
	c.parseLocalAddr(conn.LocalAddr)
	c.logQuery(m, s.RemoteAddr)
	return
}

func (c *clientWriter) Write(m []byte) (int, error) {
	//TODO(jdef) need to get this somehow from the writer delegate?
	//c.parseLocalAddr(conn.LocalAddr)
	c.logResponse(m)
	return c.out.Write(m)
}

func (c *clientIO) parseLocalAddr(a func() net.Addr) {
	c.parseAddrOnce.Do(func() {
		host, port, err := net.SplitHostPort(a().String())
		if err != nil {
			return
		}
		if iport, err := strconv.Atoi(port); err != nil {
			c.responsePort = uint32(iport)
		}
		if ip := net.ParseIP(host); ip != nil {
			c.responseAddr = ([]byte)(ip)
		}
	})
}

func (c *clientReader) logQuery(m []byte, remoteAddr func() net.Addr) {
	now := time.Now()

	var fam *SocketFamily
	if remoteAddr != nil {
		//TODO(jdef) remote address is nil, we're no longer getting it
		//from dns.ResponseWriter and conn.RemoteAddr() is useless for
		//UDP sessions.
		if ra := remoteAddr(); ra != nil {
			clientAddr := ra.String()
			_, fam = parseAddr(clientAddr)
		}
	}

	qsec := uint64(now.Unix())
	qnsec := uint32(now.Nanosecond())
	tapmsg := &Dnstap{
		Type: Dnstap_MESSAGE.Enum(),
		Message: &Message{
			Type:           Message_CLIENT_QUERY.Enum(),
			SocketFamily:   fam,
			SocketProtocol: c.protocol,
			QueryMessage:   m[:],
			QueryTimeSec:   &qsec,
			QueryTimeNsec:  &qnsec,
		},
	}
	if c.responsePort != 0 {
		tapmsg.Message.ResponsePort = &c.responsePort
	}
	if c.responseAddr != nil {
		tapmsg.Message.ResponseAddress = c.responseAddr
	}
	c.tap.log(tapmsg)
}

func (c *clientWriter) logResponse(m []byte) {
	now := time.Now()

	/*TODO(jdef) need to get this from dns.Writer somehow
	clientAddr := remoteAddr().String()
	_, fam := parseAddr(clientAddr)
	*/

	qsec := uint64(now.Unix())
	qnsec := uint32(now.Nanosecond())
	tapmsg := &Dnstap{
		Type: Dnstap_MESSAGE.Enum(),
		Message: &Message{
			Type:           Message_CLIENT_RESPONSE.Enum(),
			SocketProtocol: c.protocol,
			/*
				SocketFamily:   fam,
			*/
			ResponseMessage:  m[:],
			ResponseTimeSec:  &qsec,
			ResponseTimeNsec: &qnsec,
		},
	}
	if c.responsePort != 0 {
		tapmsg.Message.ResponsePort = &c.responsePort
	}
	if c.responseAddr != nil {
		tapmsg.Message.ResponseAddress = c.responseAddr
	}
	c.tap.log(tapmsg)
}

func NewTap(sink Sink) *Tap {
	t := &Tap{
		sink: sink,
	}
	return t
}

func (t *Tap) ClientDecorators(s *dns.Server) (dns.DecorateReader, dns.DecorateWriter) {
	var protocol *SocketProtocol
	switch s.Net {
	case "tcp", "tcp4", "tcp6":
		protocol = SocketProtocol_TCP.Enum()
	case "udp", "udp4", "udp6":
		protocol = SocketProtocol_UDP.Enum()
	}

	reader := dns.DecorateReader(func(r dns.Reader) dns.Reader {
		return &clientReader{
			clientIO: clientIO{
				tap:      t,
				protocol: protocol,
			},
			in: r,
		}
	})
	writer := dns.DecorateWriter(func(w dns.Writer) dns.Writer {
		return &clientWriter{
			clientIO: clientIO{
				tap:      t,
				protocol: protocol,
			},
			out: w,
		}
	})
	return reader, writer
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
