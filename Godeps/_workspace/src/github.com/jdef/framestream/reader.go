package framestream

import (
	"errors"
	"io"
	"log"
	"sync"
)

const (
	DefaultReaderMaxFrameSize = 1048576
)

type Reader struct {
	*RW
	contentTypes []ContentType
	maxFrameSize uint32
	err          error
	errLock      sync.RWMutex
	data         chan []byte
}

func NewReader(rw *RW, types []ContentType, maxFrameSize uint32) *Reader {
	if maxFrameSize < DefaultReaderMaxFrameSize {
		maxFrameSize = DefaultReaderMaxFrameSize
	}
	reader := &Reader{
		RW:           rw,
		maxFrameSize: maxFrameSize,
		data:         make(chan []byte, 2), // TODO(jdef) should queue depth be configurable?
	}
	for _, t := range types {
		switch l := len(t); {
		case l == 0:
			continue
		case l <= ControlFieldContentTypeMaxLength:
			reader.contentTypes = append(reader.contentTypes, t)
		default:
			// TODO(jdef) original implementation generates an error, we'll just ignore for now
		}
	}
	return reader
}

type readerStateFn func(*Reader) readerStateFn

func (r *Reader) Run() {
	defer close(r.data) // all writes to this chan happen in the same goroutine as this func
	st := readerStateOpening
	for {
		next := st(r)
		if next == nil {
			break
		}
		st = next
	}
}

func (r *Reader) Input() <-chan []byte {
	return r.data
}

func (r *Reader) Error() error {
	r.errLock.RLock()
	defer r.errLock.RUnlock()
	return r.err
}

func (r *Reader) setError(err error) {
	if err != nil {
		r.errLock.Lock()
		defer r.errLock.Unlock()
		if r.err == nil {
			r.err = err
		}
	}
}

func readerStateOpening(r *Reader) readerStateFn {
	var err error
	if r.WriteCloser == nil {
		err = r.openUni()
	} else {
		err = r.openBidi()
	}
	r.setError(err)
	if err != nil {
		return nil
	}
	log.Println("tap:reader opened")
	return readerStateOpen
}

func readerStateFailed(r *Reader) readerStateFn {
	//TODO(jdef) to any required cleanup work here
	return nil
}

func readerStateClosing(r *Reader) readerStateFn {
	var err error
	if r.WriteCloser != nil {
		err = r.writeControl(ControlTypeFinish, nil)
		r.setError(err)
		err = r.WriteCloser.Close()
		r.setError(err)
	}

	err = r.ReadCloser.Close()
	r.setError(err)
	return readerStateClosed
}

func readerStateClosed(r *Reader) readerStateFn {
	//TODO(jdef) to any required cleanup work here
	return nil
}

func readerStateOpen(r *Reader) readerStateFn {
	// read data from the stream and write it to a data chan
	var buf []byte
	framelen, err := read32(r)
	if err != nil {
		goto failed
	}
	if framelen == 0 {
		// control frame
		cf, err := r.readControlFrame(false)
		if err != nil {
			goto failed
		}
		if cf.Type == ControlTypeStop {
			// end of stream
			return readerStateClosing
		}
		// ignore all other control frames, try to read data
		return readerStateOpen
	}

	// data frame
	if framelen > r.maxFrameSize {
		goto failed
	}
	buf = make([]byte, int(framelen), int(framelen))
	_, err = io.ReadFull(r, buf)
	if err != nil {
		goto failed
	}
	r.data <- buf
	return readerStateOpen
failed:
	r.setError(err)
	return readerStateFailed
}

// openUni (fstrm__reader_open_unidirectional)
func (r *Reader) openUni() error {
	// read START frame & match content type
	start, err := r.readControl(ControlTypeStart)
	if err != nil {
		return err
	}
	match := true
	for _, t := range r.contentTypes {
		match = start.MatchFieldContentType(t)
		if match {
			break
		}
	}
	if !match {
		return errors.New("failed to match content-type for START control-frame")
	}
	return nil
}

// openBidi opens a bidirectional stream with proper handshaking semantics
func (r *Reader) openBidi() error {
	// read READY frame
	ready, err := r.readControl(ControlTypeReady)
	if err != nil {
		return err
	}

	// write ACCEPT frame w/ matching content types from READY frame
	af := &Control{
		Type: ControlTypeAccept,
	}
	for _, t := range r.contentTypes {
		if ready.MatchFieldContentType(t) {
			af.ContentTypes = append(af.ContentTypes, t)
		}
	}
	err = r.writeControlFrame(af)
	if err != nil {
		return err
	}

	return r.openUni()
}
