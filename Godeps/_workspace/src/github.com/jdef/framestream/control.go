package framestream

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

const (
	ControlFrameMaxLength            = 512
	ControlFieldContentTypeMaxLength = 256
)

type ControlType uint32

const (
	ControlTypeAccept ControlType = iota + 1
	ControlTypeStart
	ControlTypeStop
	ControlTypeReady
	ControlTypeFinish
)

func (ct *ControlType) Unmarshal(r io.Reader) error {
	ft, err := read32(r)
	if err != nil {
		return err
	}
	switch xct := ControlType(ft); xct {
	case ControlTypeAccept, ControlTypeStart, ControlTypeStop, ControlTypeReady, ControlTypeFinish:
		*ct = xct
	default:
		return errors.New(fmt.Sprintf("unknown control frame type %d", ft))
	}
	return nil
}

type ControlField uint32

const (
	ControlFieldContentType ControlField = iota + 1
)

type ControlFlag int

const (
	ControlFlagWithHeader ControlFlag = 1 << iota
)

//TODO(jdef) why is this a []byte? should it be a string?
type ContentType []byte

type Control struct {
	Type         ControlType
	ContentTypes []ContentType
}

func (c *Control) MatchFieldContentType(match ContentType) bool {
	// STOP and FINISH frames don't have content types
	switch c.Type {
	case ControlTypeStop, ControlTypeFinish:
		return false
	}
	if len(c.ContentTypes) == 0 {
		return true
	}
	// control frame has >= 1 content type which cannot match an unset type
	if match == nil {
		return false
	}
	for _, t := range c.ContentTypes {
		if bytes.Compare(match, t) == 0 {
			return true
		}
	}
	return false
}

// Marshall (fstrm_control_encode)
func (c *Control) Marshal(flags ControlFlag) ([]byte, error) {
	buf := &bytes.Buffer{}
	sz, err := c.calcEncodedSize(flags)
	if err != nil {
		return nil, err
	}
	buf.Grow(int(sz))
	w32 := func(v uint32) {
		if err == nil {
			err = write32(buf, v)
		}
	}

	if (flags & ControlFlagWithHeader) > 0 {
		w32(0)      // escape
		w32(sz - 8) // frame-length: does not include escape field, nor this one
	}

	w32(uint32(c.Type))
	for _, t := range c.ContentTypes {
		if c.Type == ControlTypeStop || c.Type == ControlTypeFinish {
			break // no content type fields for these control frames
		}

		w32(uint32(ControlFieldContentType))
		w32(uint32(len(t)))

		if err == nil {
			_, err = buf.Write(t)
		}
		if c.Type == ControlTypeStart {
			break // only one allowed
		}
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *Control) calcEncodedSize(flags ControlFlag) (uint32, error) {
	result := uint32(0)
	if (flags & ControlFlagWithHeader) > 0 {
		result += 8 // 2x uint32 :: escape, frame-length
	}
	result += 4 // control-type
	for _, t := range c.ContentTypes {
		if c.Type == ControlTypeStop || c.Type == ControlTypeFinish {
			break // no content type fields for these control frames
		}
		result += 8 // 2x uint32 :: control-field-content-type, payload length field
		if len(t) > ControlFieldContentTypeMaxLength {
			return 0, errors.New(fmt.Sprintf("content type field length %d exceeds max allowed %d", len(t), ControlFieldContentTypeMaxLength))
		}
		result += uint32(len(t))

		if c.Type == ControlTypeStart {
			break // only one allowed
		}
	}
	if result > ControlFrameMaxLength {
		return 0, errors.New(fmt.Sprintf("control frame length %d exceeds max allowed %d", result, ControlFrameMaxLength))
	}
	return result, nil
}

// Unmarshal (fstrm_control_decode)
func (c *Control) Unmarshal(buf []byte, flags ControlFlag) error {
	p := bytes.NewBuffer(buf)
	if (flags & ControlFlagWithHeader) > 0 {
		// read outer frame len
		outer, err := read32(p)
		if err != nil {
			return err
		}
		if outer != 0 {
			return errors.New(fmt.Sprintf("expected outer frame length 0 for control frame instead of %d", outer))
		}
		if outer > ControlFrameMaxLength {
			return errors.New(fmt.Sprintf("outer frame length %d exceeds max control frame length %d", outer, ControlFrameMaxLength))
		}
		if outer != uint32(p.Len()) {
			return errors.New(fmt.Sprintf("remaining frame buffer len %d != frame length %d", p.Len(), outer))
		}
	} else {
		if p.Len() > ControlFrameMaxLength {
			return errors.New(fmt.Sprintf("control frame length %d exceeds max length %d", p.Len(), ControlFrameMaxLength))
		}
	}

	// read frame type
	err := (&c.Type).Unmarshal(p)
	if err != nil {
		return err
	}

	for p.Len() > 0 {
		// read control frame field type
		cft, err := read32(p)
		if err != nil {
			return err
		}
		switch cft := ControlField(cft); cft {
		case ControlFieldContentType:
			// payload length
			plen, err := read32(p)
			if err != nil {
				return err
			}
			if plen > uint32(p.Len()) {
				return errors.New(fmt.Sprintf("content type payload len exceeds control frame"))
			}
			if plen > ControlFieldContentTypeMaxLength {
				return errors.New(fmt.Sprintf("content type field len %d exceeds max allowed %d", plen, ControlFieldContentTypeMaxLength))
			}
			buf := p.Next(int(plen))
			if uint32(len(buf)) != plen {
				return errors.New("not enough bytes remain in buffer for content type field payload")
			}
			c.ContentTypes = append(c.ContentTypes, ContentType(buf))
		default:
			return errors.New(fmt.Sprintf("unknown control field frame type: %d", cft))
		}
	}
	// enforce limits of content type fields
	switch tlen := len(c.ContentTypes); c.Type {
	case ControlTypeStart:
		if tlen > 1 {
			return errors.New("too many content type payloads for control-start")
		}
	case ControlTypeStop, ControlTypeFinish:
		if tlen > 0 {
			return errors.New("too many content type payloads for control-start")
		}
	}
	return nil
}
