package flags

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	// Run defined if focusing some test, suite or subtest
	Run          *regexp.Regexp
	Verbose      bool
	PanicOnExit0 bool
	TestifyM     string

	Golden bool

	RunString string
)

func init() {
	err := Parse(os.Args[1:])
	if err != nil {
		log.Warn(err)
	}
}

// Parse sets package global variables
// Known issue: If the flags are in the wrong order, the values may be undefined even if specified in the arguments.
// See: https://github.com/golang/go/issues/58839
// TODO: parse arguments without flag pkg
func Parse(args []string) error {
	fSet := flag.NewFlagSet("testing/flags", flag.ContinueOnError)
	fSet.SetOutput(io.Discard)

	fSet.StringVar(&RunString, "test.run", "", "run only tests and examples matching `regexp`")
	fSet.BoolVar(&Verbose, "test.v", false, "verbose: print additional output")
	fSet.BoolVar(&PanicOnExit0, "test.paniconexit0", false, "")
	fSet.StringVar(&TestifyM, "testify.m", "", "")
	fSet.BoolVar(&Golden, "golden", false, "generate golden files")

	err := fSet.Parse(args)
	if err != nil && strings.Contains(err.Error(), "flag provided but not defined") {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if RunString != "" {
		Run, err = regexp.Compile(RunString)
		if err != nil {
			return fmt.Errorf("parse %s regexp: %w", RunString, err)
		}
	}

	return nil
}
