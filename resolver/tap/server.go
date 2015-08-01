package tap

import (
	"log"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/jdef/framestream"
)

type Server struct {
	listen    net.Listener
	mainInput chan *Dnstap
	bcast     *broadcast
}

func LocalListenAndServe() (*Server, <-chan error) {
	errCh := make(chan error, 1)
	l, err := net.Listen("tcp", "127.0.0.1:2000")
	if err != nil {
		errCh <- err
		return nil, errCh
	}
	s := NewServer(l)
	go func() {
		errCh <- s.Serve()
	}()
	return s, errCh
}

func NewServer(l net.Listener) *Server {

	//TODO(jdef) tap-sinks --> (in) main (out) --> broadcast --> tcp-clients

	mainOutput := make(chan *Dnstap, 1024) // basis of the ring buffer

	s := &Server{
		listen:    l,
		mainInput: make(chan *Dnstap),
		bcast:     newBroadcast(mainOutput, 0),
	}

	mainRing := newLogRing(s.mainInput, mainOutput)
	go mainRing.run()
	go s.bcast.run()

	return s
}

// NewSink returns a Sink to the caller, allows them to write dnstap messages without
// blocking. If no one is listening for dnstap messages then they're dropped.
func (s *Server) NewSink() Sink {
	sinkIn := make(chan *Dnstap)
	sinkOut := make(chan *Dnstap, 256)
	sinkRing := newLogRing(sinkIn, sinkOut)
	go sinkRing.run()

	// pipe from sink-out to main.in
	go func() {
		for x := range sinkOut {
			s.mainInput <- x
		}
	}()

	// callers write to sink-in
	// TODO(jdef) we expect callers to be daemons, so it's probably not a huge problem that
	// there's nothing to terminate the sink here.
	return SinkFunc(func(x *Dnstap) {
		sinkIn <- x
	})
}

func (s *Server) Serve() error {
	done := make(chan struct{})
	defer close(done)
	for {
		conn, err := s.listen.Accept()
		if err != nil {
			return err
		}
		go s.Handle(conn, done)
	}
}

func (s *Server) Handle(c net.Conn, done <-chan struct{}) {
	log.Println("new tap client:", c.RemoteAddr())
	defer log.Println("tap client finished:", c.RemoteAddr())

	//TODO(jdef) set reasonable r/w timeouts
	// (a) bidi writer, handshake
	// (b) read from main ring, write framed data until stop event
	rw := &framestream.RW{
		ReadCloser:  c,
		WriteCloser: c,
	}

	defer c.Close()
	writer := framestream.NewWriter(rw, nil)

	abort := make(chan struct{})
	defer close(abort)
	go writer.Run(abort)

	writerDone := writer.Done()
	out := writer.Output()

	dataSource, err := s.bcast.listen()
	if err != nil {
		log.Println("ERROR:", err.Error())
		return
	}
	defer dataSource.destroy()
	input := dataSource.input()

writeLoop:
	for {
		select {
		case <-done:
			break writeLoop
		case <-writerDone:
			break writeLoop
		case x := <-input:
			data, err := proto.Marshal(x)
			if err != nil {
				//TODO(jdef) log this somewhere?
				continue
			}
			select {
			case <-done:
				break writeLoop
			case <-writerDone:
				break writeLoop
			case out <- data:
			}
		}
	}
}
