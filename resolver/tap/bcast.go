package tap

import (
	"errors"
	"math/big"
	"math/rand"
	"sync"
)

type broadcast struct {
	in        <-chan *Dnstap
	out       map[int]Sink
	lock      sync.Mutex
	slots     big.Int
	slotCount int // how many slots are there?
}

type listener struct {
	in      <-chan *Dnstap
	destroy func()
}

func (l *listener) input() <-chan *Dnstap {
	return l.in
}

func newBroadcast(in <-chan *Dnstap, slotCount int) *broadcast {
	if slotCount <= 0 {
		slotCount = 200 // arbitrary
	}
	return &broadcast{
		in:        in,
		out:       map[int]Sink{},
		slotCount: slotCount,
	}
}

func (b *broadcast) run() {
	for x := range b.in {
		var sinks []Sink
		// get a copy of the current sinks
		func() {
			b.lock.Lock()
			defer b.lock.Unlock()
			sinks = make([]Sink, 0, len(b.out))
			for _, out := range b.out {
				sinks = append(sinks, out)
			}
		}()
		// pass the data object to each sink (without blocking ops
		// on the broadcaster)
		for _, out := range sinks {
			out.Write(x)
		}
	}
}

// listen spins up a logRing buffer, forwarding messages from the broadcast to the buffer,
// and returns a listener that read messages from the buffer. callers are expected to call
// destroy when the listener is no longer needed: this removes the listener from its parent
// broadcaster and cleans up the ring buffer associated with the listener.
func (b *broadcast) listen() (*listener, error) {
	in := make(chan *Dnstap, 10)
	out := make(chan *Dnstap, 100)
	sink := newLogRing(in, out)
	go sink.run()

	abort := make(chan struct{})
	var detach func()

	sf := SinkFunc(func(x *Dnstap) {
		select {
		case in <- x:
		case <-abort:
			// allow the logRing to die when the listener is destroyed; this func is called by
			// the broadcaster, so we know we're still registered at this point. close() will
			// shut down the ring buffer and detach() will remove us from the broadcaster.
			detach()
			close(in)
		}
	})
	sid, err := b.register(sf)
	if err != nil {
		close(in) // no one is writing here yet, safe
		return nil, err
	}
	detach = func() {
		b.unregister(sid)
	}
	return &listener{
		in: out,
		destroy: func() {
			// lazy: cause the associated ring to terminate upon next broadcast. should be fine
			// for our needs.
			close(abort)
		},
	}, nil
}

func (b *broadcast) register(s Sink) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	sid, err := b.nextSlot()
	if err != nil {
		return 0, err
	}
	b.out[sid] = s
	return sid, nil
}

func (b *broadcast) unregister(sid int) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.slots.SetBit(&b.slots, sid, 0)
	delete(b.out, sid)
}

var errNoSlotsLeft = errors.New("no slots left in broadcaster")

// nextSlot looks for an open broadcaster slot and returns the slot id if
// found, otherwise returns an error if no slots are open. assumes that caller
// is locking around shared state.
func (b *broadcast) nextSlot() (int, error) {
	// pick a random slot index, check if empty
	// if already taken, scan until found or end of range
	match := -1
	i := int(rand.Int31())
	for j := i; j < b.slotCount && match < 0; j++ {
		x := b.slots.Bit(j)
		if x == 0 {
			match = j
		}
	}
	// if end of range, start from pos 0 and loop around
	for j := 0; j < i && match < 0; j++ {
		x := b.slots.Bit(j)
		if x == 0 {
			match = j
		}
	}
	// nothing left? return error
	if match < 0 {
		return 0, errNoSlotsLeft
	}
	b.slots.SetBit(&b.slots, match, 1)
	return match, nil
}
