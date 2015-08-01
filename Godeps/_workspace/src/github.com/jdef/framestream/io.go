package framestream

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	errExpectedEscapeSequence = errors.New("expected escape sequence")
)

type RW struct {
	io.ReadCloser
	io.WriteCloser
}

func read32(r io.Reader) (tmp uint32, err error) {
	err = binary.Read(r, binary.BigEndian, &tmp)
	return
}

func write32(w io.Writer, v uint32) error {
	return binary.Write(w, binary.BigEndian, v)
}

func (rw *RW) readEscapeSeq() error {
	tmp, err := read32(rw)
	if err != nil {
		return err
	}
	if tmp != 0 {
		return errExpectedEscapeSequence
	}
	return nil
}

func (rw *RW) readControl(wantedType ControlType) (*Control, error) {
	cf, err := rw.readControlFrame(true)
	if err != nil {
		return nil, err
	}
	if wantedType != cf.Type {
		return nil, errors.New(fmt.Sprintf("wanted control frame type %d instead of %d", wantedType, cf.Type))
	}
	return cf, nil
}

func (rw *RW) readControlFrame(withEscape bool) (*Control, error) {
	if withEscape {
		err := rw.readEscapeSeq()
		if err != nil {
			return nil, err
		}
	}

	// control frame len
	len, err := read32(rw)
	if err != nil {
		return nil, err
	}
	if len > ControlFrameMaxLength {
		return nil, errors.New(fmt.Sprintf("control frame length %d exceeds max length %d", len, ControlFrameMaxLength))
	}

	// read control frame
	buf := make([]byte, len, len)
	_, err = io.ReadFull(rw, buf)
	if err != nil {
		return nil, err
	}

	// decode control frame
	cf := &Control{}
	err = cf.Unmarshal(buf, 0)
	if err != nil {
		return nil, err
	}
	return cf, nil
}

func (rw *RW) writeControlFrame(c *Control) error {
	const flags = ControlFlagWithHeader
	buf, err := c.Marshal(flags)
	n, err := rw.Write(buf)
	if err != nil {
		return err
	}
	if n != len(buf) {
		return errors.New(fmt.Sprintf("failed to write full frame"))
	}
	return nil
}

func (rw *RW) writeControl(t ControlType, ct *ContentType) error {
	cc := &Control{
		Type: t,
	}
	if ct != nil {
		cc.ContentTypes = append(cc.ContentTypes, *ct)
	}
	return rw.writeControlFrame(cc)
}
