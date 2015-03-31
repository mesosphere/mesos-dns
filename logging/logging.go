package logging

import (
	"github.com/golang/glog"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"sync/atomic"
)

var (
	VerboseFlag     bool
	VeryVerboseFlag bool
	Verbose         *log.Logger
	VeryVerbose     *log.Logger
	Error           *log.Logger
)

type Counter interface {
	Inc()
}

type LogCounter struct {
	value uint64
}

func (lc *LogCounter) Inc() {
	atomic.AddUint64(&lc.value, 1)
}

func (lc *LogCounter) String() string {
	return strconv.FormatUint(atomic.LoadUint64(&lc.value), 10)
}

type LogOut struct {
	MesosRequests    Counter
	MesosSuccess     Counter
	MesosNXDomain    Counter
	MesosFailed      Counter
	NonMesosRequests Counter
	NonMesosSuccess  Counter
	NonMesosNXDomain Counter
	NonMesosFailed   Counter
	NonMesosRecursed Counter
}

var CurLog = LogOut{
	MesosRequests:    &LogCounter{},
	MesosSuccess:     &LogCounter{},
	MesosNXDomain:    &LogCounter{},
	MesosFailed:      &LogCounter{},
	NonMesosRequests: &LogCounter{},
	NonMesosSuccess:  &LogCounter{},
	NonMesosNXDomain: &LogCounter{},
	NonMesosFailed:   &LogCounter{},
	NonMesosRecursed: &LogCounter{},
}

// PrintCurLog prints out the current LogOut and then resets
func PrintCurLog() {
	VeryVerbose.Printf("%+v\n", CurLog)
}

// SetupLogs provides the following logs
// Verbose = optional verbosity
// VeryVerbose = optional verbosity
// Error = stderr
func SetupLogs() {
	// initialize logging flags
	if glog.V(2) {
		VeryVerboseFlag = true
	} else if glog.V(1) {
		VerboseFlag = true
	}

	logopts := log.Ldate | log.Ltime | log.Lshortfile

	if VerboseFlag {
		Verbose = log.New(os.Stdout, "VERBOSE: ", logopts)
		VeryVerbose = log.New(ioutil.Discard, "VERY VERBOSE: ", logopts)
	} else if VeryVerboseFlag {
		Verbose = log.New(os.Stdout, "VERY VERBOSE: ", logopts)
		VeryVerbose = Verbose
	} else {
		Verbose = log.New(ioutil.Discard, "VERBOSE: ", logopts)
		VeryVerbose = log.New(ioutil.Discard, "VERY VERBOSE: ", logopts)
	}

	Error = log.New(os.Stderr, "ERROR: ", logopts)
}
