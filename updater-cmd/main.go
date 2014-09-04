package main

import (
	"flag"
	"fmt"
	"github.com/IMQS/updater"
	"os"
)

const usageTxt = `commands:
  buildmanifest <dir>  Update <dir>/hash
  updatehash <dir>     Build manifest in <dir>
  run                  Run in foreground
  service              Run as a Windows Service
`

func main() {

	flagConfig := flag.String("config", "", "JSON config file (must be specified)")

	flag.Usage = func() {
		os.Stderr.WriteString(usageTxt)
		fmt.Fprintf(os.Stderr, "options:\n")
		flag.PrintDefaults()
	}

	helpDie := func(msg string) {
		if msg != "" {
			fmt.Fprintf(os.Stderr, "%v\n", msg)
		} else {
			flag.Usage()
		}
		os.Exit(1)
	}

	errDie := func(err error) {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	flag.CommandLine.Parse(os.Args[1:])

	cmd := flag.Arg(0)

	upd := updater.NewUpdater()

	initSvc := func() {
		if *flagConfig == "" {
			helpDie("No config specified")
		} else if err := upd.Config.LoadFile(*flagConfig); err != nil {
			helpDie(err.Error())
		}

		if err := upd.Initialize(); err != nil {
			helpDie(err.Error())
		}
	}

	run := func() {
		upd.Run()
	}

	if cmd == "buildmanifest" {
		if len(flag.Args()) != 2 {
			helpDie("no directory specified")
		}
		root := flag.Arg(1)
		if manifest, err := updater.BuildManifest(root); err != nil {
			errDie(err)
		} else {
			if err := manifest.Write(root); err != nil {
				errDie(err)
			}
		}
	} else if cmd == "run" {
		initSvc()
		fmt.Printf("Starting\n")
		run()
	} else if cmd == "service" {
		initSvc()
		fmt.Printf("Starting as service\n")
		if !updater.RunAsService(run) {
			fmt.Printf("Unable to run as service\n")
		}
	} else if cmd == "" {
		helpDie("")
	} else {
		helpDie("Unrecognized command: " + cmd)
	}
}
