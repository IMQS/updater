package updater

import (
	"testing"
)

func TestCygwin(t *testing.T) {
	u := NewUpdater()
	pairs := []string{
		"c:\\", "/cygdrive/c/",
		"C:\\", "/cygdrive/c/",
		"c:\\bin", "/cygdrive/c/bin",
		"c:\\bin/", "/cygdrive/c/bin/",
		"c:/bin\\", "/cygdrive/c/bin/",
		"e:\\bin", "/cygdrive/e/bin",
	}
	for i := 0; i < len(pairs); i += 2 {
		cyg := u.pathToCygwin(pairs[i])
		if cyg != pairs[i+1] {
			t.Errorf("cygwin path conversion of %v. Expected %v, but instead %v", pairs[i], pairs[i+1], cyg)
		}
	}
}
