// +build !windows

package updater

import (
	"github.com/IMQS/log"
)

func runService(log *log.Logger, handler func()) bool {
	return false
}
