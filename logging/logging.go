package logging

import (
	"io/ioutil"
	"log"
	"os"
)

var (
	VerboseFlag     bool
	VeryVerboseFlag bool
	Verbose         *log.Logger
	VeryVerbose     *log.Logger
	Error           *log.Logger
)

type LogOut struct {
	MesosRequests    int
	MesosSuccess     int
	MesosNXDomain    int
	MesosFailed      int
	NonMesosRequests int
	NonMesosSuccess  int
	NonMesosNXDomain int
	NonMesosFailed   int
	NonMesosRecursed int
}

var CurLog LogOut

// PrintCurLog prints out the current LogOut and then resets
func PrintCurLog() {
	Verbose.Printf("%+v\n", CurLog)
}

// SetupLogs provides the following logs
// Verbose = optional verbosity
// VeryVerbose = optional verbosity
// Error = stderr
func SetupLogs() {
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
