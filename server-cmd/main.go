package main

/*
The main purpose of this program is to take the set of builds uploaded via rsync,
and get them ready for deployment via HTTP.

Example of "incoming" directory, uploaded via rsync:

/incoming
	/channels
		/rc
			35		(an empty file named "35")
		/stable
			10		(an empty file named "10")
		/team-mm
			37		(an empty file named "37")
	/versions
		/10
			...all the files of version 10
		/35
			...all the files of version 35
		/37
			...all the files of version 37

Example of "files" directory, hosted by nginx:

/files
	/channels
		/rc --symlink--> /files/versions/35
		/stable --symlink--> /files/versions/10
		/team-mm --symlink--> /files/versions/37
	/versions
		/10
			...all the files of version 10
		/35
			...all the files of version 35
		/37
			...all the files of version 37

What makes all of this pretty simple is the fact that a "version" is immutable.
The job that we need to do here can be summarized as:
1. Copy new versions from "incoming" into "files".
2. Update all of the symlinks in "files/channels" so that they point to the correct version.

How do we achieve (2) atomically?
We first create a new symlink, and then we use "mv -T" to rename the new symlink to
the old symlink.
For example, if /files/channels/rc links to /files/versions/20, and we want to change
it to link to /21, then what we do is:
ln -s /files/versions/21 /files/channels/rc.new
mv -T /files/channels/rc.new /files/channels/rc
The "mv" step is atomic.

*/

import (
	"bytes"
	"github.com/IMQS/cli"
	"github.com/IMQS/log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var logger *log.Logger

func run(name string, args []string, options cli.OptionSet) {
	var err error
	switch name {
	case "serve":
		root := args[0]
		http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir(root))))
		err = http.ListenAndServe(":8080", nil)
	case "process-incoming":
		err = processIncoming(args[0], args[1])
	}
	if err != nil {
		logger.Errorf("%v", err)
	}
}

func main() {
	logger = log.New(log.Stdout)
	app := cli.App{}
	app.Description = "update-server [options] command"
	app.DefaultExec = run
	app.AddCommand("serve", "Run an HTTP server on 8080, with the URL /files/* serving up content from root-dir/*", "root-dir")
	app.AddCommand("process-incoming", "Prepare and move rsync-uploaded build files into the HTTP-hosted live area", "incoming-dir", "hosted-dir")
	app.Run()
}

func processIncoming(incomingSrc, hostedDst string) error {
	// rsync -r /cygdrive/c/dev/head/updater/examples/server-case1/incoming/versions/ /cygdrive/c/dev/head/updater/examples/server-case1/try-files/versions
	// go run src\github.com\IMQS\updater\server-cmd\main.go process-incoming /cygdrive/c/dev/head/updater/examples/server-case1/incoming /cygdrive/c/dev/head/updater/examples/server-case1/try-files

	if err := shellExec("rsync", "-r", incomingSrc+"/versions/", hostedDst+"/versions"); err != nil {
		return err
	}

	filepath.Walk(incomingSrc+"/channels", func(path string, info os.FileInfo, err error) error {
		if info.
	})

	return nil
}

func shellExec(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	if err != nil {
		logger.Errorf("Error executing \"%v %v\":\nstdout: %v\nstderr: %v", cmd, strings.Join(args, " "), stdout.String(), stderr.String())
	}
	return err
}
