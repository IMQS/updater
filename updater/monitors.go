package updater

// This is the place to put functions that run before and after synchronizing directories

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os/exec"
	"path"
	"strings"
	"time"
)

var ErrServiceNotStopping = errors.New("Service not stopping")

func readLines(filename string) ([]string, error) {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return []string{}, err
	}
	lines := strings.Split(string(raw), "\n")
	for i := range lines {
		lines[i] = strings.Trim(lines[i], "\r\n \t")
	}
	return lines, nil
}

/*
Do the conservative thing here, and return all services, from current and next.
We should really only need the service names from current. However, imagine the
case where a developer adds a new service to imqsbin, and somehow forgets to
have it added to servicenames. Once that service is running, the machine would
be unable to perform any updates, because the final mirror sync would fail due
to that rogue service that is never stopped.
By using the 'next' service names also, we allow an update to be pushed out
which would provide the new service name, thereby unbricking the server.
*/
func imqsServiceNames(upd *Updater) []string {
	oldNames, _ := readLines(path.Join(upd.Config.BinDir.LocalPath, "servicenames"))
	newNames, _ := readLines(path.Join(upd.Config.BinDir.LocalPathNext, "servicenames"))
	for _, nNew := range newNames {
		exists := false
		for _, nOld := range oldNames {
			if nOld == nNew {
				exists = true
				break
			}
		}
		if !exists {
			oldNames = append(oldNames, nNew)
		}
	}
	return oldNames
}

func stopService(name string) {
	exec.Command("sc", "stop", name).Run()
}

func startService(name string) {
	exec.Command("sc", "start", name).Run()
}

func isServiceRunning(name string) bool {
	var stdout bytes.Buffer
	cmd := exec.Command("sc", "query", name)
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		if strings.Index(stdout.String(), "service does not exist") != -1 {
			return false
		}
		// Assume the service is running, because that is the conservative thing here
		return true
	}
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		if strings.Index(line, "service does not exist") != -1 {
			return false
		}
		if strings.Index(line, "STATE") != -1 && strings.Index(line, "STOPPED") != -1 {
			return false
		}
	}
	return true
}

func beforeSyncImqs(upd *Updater, updatedDirs []*SyncDir) error {
	services := imqsServiceNames(upd)
	upd.log.Infof("Stopping services (%v)", strings.Join(services, ", "))
	for _, s := range services {
		stopService(s)
	}
	start := time.Now()
	for {
		running := []string{}
		for _, s := range services {
			if isServiceRunning(s) {
				running = append(running, s)
			}
		}
		if len(running) == 0 {
			upd.log.Infof("All services stopped")
			break
		}
		if time.Now().Sub(start) > time.Second*time.Duration(upd.Config.ServiceStopWaitSeconds) {
			upd.log.Errorf("Abandoning update, because services (%v) are not stopping (timeout %vs)", strings.Join(running, ", "), upd.Config.ServiceStopWaitSeconds)
			for _, s := range services {
				startService(s)
			}
			return ErrServiceNotStopping
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

func afterSyncImqs(upd *Updater, updatedDirs []*SyncDir) {

	// TODO: run install.rb

	services := imqsServiceNames(upd)
	upd.log.Infof("Starting services (%v)", strings.Join(services, ", "))
	for _, s := range services {
		startService(s)
	}

	// TODO: if imqsbin/bin/imqsupdater.exe is different to imqsvar/bin/imqsupdater.exe, then update ourselves,
	// perhaps by copying the new imqsupdater.exe to c:\imqsvar\imqsupdater-temp.exe, and then launching that
	// as "c:\imqsvar\imqsupdater-temp update-self"
}
