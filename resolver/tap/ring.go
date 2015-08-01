package tap

type logRing struct {
	input  <-chan *Dnstap
	output chan *Dnstap
}

func newLogRing(in <-chan *Dnstap, out chan *Dnstap) *logRing {
	r := &logRing{
		input:  in,
		output: out,
	}
	return r
}

func (r *logRing) run() {
	defer close(r.output)
	for x := range r.input {
		select {
		case r.output <- x:
		default:
			<-r.output // drop item
			r.output <- x
		}
	}
}
