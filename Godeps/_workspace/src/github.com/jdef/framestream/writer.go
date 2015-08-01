package framestream

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

type Writer struct {
	*RW
	contentTypes []ContentType // read-only data
	data         chan []byte
	stop         chan struct{} // signal chan, closes when writer-owner says data writes should stop
	err          error
	errLock      sync.RWMutex
	finish       chan struct{} // used to signal close of bidi network connections
	done         chan struct{} // closes when either .stop or .finish closes
}

func (r *Writer) Error() error {
	r.errLock.RLock()
	defer r.errLock.RUnlock()
	return r.err
}

func (r *Writer) setError(err error) {
	if err != nil {
		r.errLock.Lock()
		defer r.errLock.Unlock()
		if r.err == nil {
			r.err = err
		}
	}
}

func (w *Writer) Output() chan<- []byte {
	return w.data
}

func (w *Writer) Done() <-chan struct{} {
	return w.done
}

type writerStateFn func(*Writer) writerStateFn

func NewWriter(rw *RW, types []ContentType) *Writer {
	writer := &Writer{
		RW:     rw,
		data:   make(chan []byte, 2),
		stop:   make(chan struct{}),
		finish: make(chan struct{}),
		done:   make(chan struct{}),
	}
	for _, t := range types {
		switch l := len(t); {
		case l == 0:
			continue
		case l <= ControlFieldContentTypeMaxLength:
			writer.contentTypes = append(writer.contentTypes, t)
		default:
			// TODO(jdef) original implementation generates an error, we'll just ignore for now
		}
	}
	go func() {
		defer close(writer.done)
		select {
		case <-writer.stop:
		case <-writer.finish:
		}
	}()
	return writer
}

func (w *Writer) Run(shouldStop <-chan struct{}) {
	go func() {
		// TODO(jdef) probably want something better here later
		defer close(w.stop)
		<-shouldStop
	}()

	st := writerStateOpening
	for {
		next := st(w)
		if next == nil {
			break
		}
		st = next
	}
}

func writerStateFailed(w *Writer) writerStateFn {
	//TODO(jdef) implement some cleanup here?
	return nil
}

func writerStateClosed(w *Writer) writerStateFn {
	//TODO(jdef) implement some cleanup here
	return nil
}

func writerStateClosing(w *Writer) writerStateFn {
	err := w.writeControl(ControlTypeStop, nil)
	w.setError(err)

	if w.ReadCloser != nil {
		// wait for finish before terminating the connection
		t := time.NewTimer(15 * time.Second)
		select {
		case <-t.C:
			// timed out waiting got FINISH, proceed to close the connection
		case <-w.finish:
			// FINISH control frame received
			t.Stop()
		}
	}

	err = w.WriteCloser.Close()
	w.setError(err)
	return writerStateClosed
}

// monitorConnection is called for bidi connections and watches for the client
// to disappear. this func blocks until the connection closes or generates a
// a non-timeout error (possibly protocol related).
func (w *Writer) monitorConnection(conn net.Conn) {
	defer close(w.finish)
	var err error
	for {
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		// test the connection by attempting to read a FINISH control frame.
		// we should only see this if we've initiated a close by sending a
		// STOP, but it doesn't hurt to start watching for this before then
		// because we're really not expecting the client to send anything
		// else at this point. we also don't have to write anything special
		// back to the client if we receive it because the client closes their
		// end of the connection upon sending.
		_, err = w.readControl(ControlTypeFinish)
		if err == io.EOF {
			// connection closed
		} else if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
			// timeout
			time.Sleep(5 * time.Second) // TODO(jdef) extract constant
			continue
		}
		break
	}
	w.setError(err)
}

func writerStateOpen(w *Writer) writerStateFn {
	if w.ReadCloser != nil {
		if conn, ok := w.ReadCloser.(net.Conn); ok {
			go w.monitorConnection(conn)
		}
	}
writeLoop:
	for {
		select {
		case <-w.finish:
			return writerStateClosing
		case <-w.stop:
			return writerStateClosing
		case buf := <-w.data:
			// we possibly won a tie, make sure that we're not stopped yet
			select {
			case <-w.finish:
				return writerStateClosing
			case <-w.stop:
				return writerStateClosing
			default:
				x := len(buf)
				err := write32(w, uint32(x))
				if err == nil {
					n, err := w.Write(buf)
					if err == nil && n == len(buf) {
						continue writeLoop
					}
				}
				w.setError(err)
				return writerStateFailed
			}
		}
	}
}

func writerStateOpening(w *Writer) writerStateFn {
	var err error
	if w.ReadCloser != nil {
		err = w.openBidi()
	} else {
		err = w.openUni()
	}
	w.setError(err)
	if err != nil {
		return writerStateFailed
	}
	return writerStateOpen
}

func (w *Writer) openBidi() error {
	// write a READY frame
	ready := &Control{
		Type:         ControlTypeReady,
		ContentTypes: w.contentTypes,
	}
	err := w.writeControlFrame(ready)
	if err != nil {
		return err
	}

	// read ACCEPT frame and find matching content type
	af, err := w.readControl(ControlTypeAccept)
	if err != nil {
		return err
	}

	match := true
	var matchType ContentType

	for _, t := range w.contentTypes {
		if af.MatchFieldContentType(t) {
			matchType = t
			break
		}
		match = false
		continue
	}
	if !match {
		return errors.New("failed to find matching content-type in ACCEPT frame")
	}

	// send a START frame, indicate a matching control type (if any)
	start := &Control{
		Type: ControlTypeStart,
	}
	if matchType != nil {
		start.ContentTypes = append(start.ContentTypes, matchType)
	}
	return w.writeControlFrame(start)
}

func (w *Writer) openUni() error {
	// send a START frame, indicate a matching control type (if any)
	start := &Control{
		Type: ControlTypeStart,
	}
	if len(w.contentTypes) > 0 {
		start.ContentTypes = append(start.ContentTypes, w.contentTypes[0])
	}
	return w.writeControlFrame(start)
}
