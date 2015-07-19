package main

import (
	"flag"
	"fmt"
	"github.com/IMQS/updater/updater"
	"os"
)

const usageTxt = `commands:
  buildmanifest <dir>  Update manifest in <dir>
  run                  Run in foreground (in console)
  service              Run as a Windows Service
  download             Check for new content, and download
  apply                If an update is ready to be applied, then do so
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

	init := func() {
		if *flagConfig == "" {
			helpDie("No config specified")
		} else if err := upd.Config.LoadFile(*flagConfig); err != nil {
			helpDie(err.Error())
		}

		if err := upd.Initialize(); err != nil {
			helpDie(err.Error())
		}
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
		init()
		upd.Run()
	} else if cmd == "download" {
		init()
		upd.Download()
	} else if cmd == "apply" {
		init()
		upd.Apply()
	} else if cmd == "service" {
		init()
		if !upd.RunAsService() {
			fmt.Printf("Unable to run as service\n")
		}
	} else if cmd == "" {
		helpDie("")
	} else {
		helpDie("Unrecognized command: " + cmd)
	}
}
