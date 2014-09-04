// +build !windows

package updater

func RunAsService(handler func()) bool {
	return false
}
