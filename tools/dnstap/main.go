package main

import (
	"errors"
	"log"
	"net"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/jdef/framestream"
	"github.com/mesosphere/mesos-dns/resolver/tap"
)

func die(err error) {
	log.Fatalln(err.Error())
}

func main() {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:2000")
	if err != nil {
		die(err)
	}

	tc, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		die(err)
	}

	tc.SetNoDelay(true)
	tc.SetKeepAlive(false)

	log.Println("connected to local dnstap")

	rw := &framestream.RW{
		ReadCloser:  tc,
		WriteCloser: tc,
	}

	reader := framestream.NewReader(rw, nil, 0)
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		reader.Run()
	}()

	in := reader.Input()
	for {
		select {
		case <-ch:
			err := reader.Error()
			if err != nil {
				die(err)
			} else {
				die(errors.New("reader terminated unexpectedly"))
			}
		case d := <-in:
			msg := &tap.Dnstap{}
			err = proto.Unmarshal(d, msg)
			if err != nil {
				die(err)
			}
			if blob, ok := YamlFormat(msg); ok {
				os.Stdout.Write(blob)
			} else {
				log.Printf("## TAP >> %#+v\n", msg.Message)
			}
		}
	}
}
