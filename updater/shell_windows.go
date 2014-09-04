package updater

import (
	"bytes"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// Mirror src directory to dst
func shellMirrorDirectory(src, dst string) (string, error) {
	//fmt.Printf("-- mirror %v to %v --\n", src, dst)
	//return nil
	cmd := exec.Command("robocopy", "/MIR", src, dst)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), interpretRobocopyError(err)
}

func interpretRobocopyError(err error) error {
	const exitMsg = "exit status "
	errString := err.Error()
	if strings.Index(errString, exitMsg) != 0 {
		return err
	}
	flags, errConv := strconv.ParseUint(errString[len(exitMsg):], 10, 64)
	if errConv != nil {
		return err
	}
	if flags >= 8 {
		return errors.New("Robocopy error: " + strconv.FormatUint(flags, 10))
	}
	return nil
}
