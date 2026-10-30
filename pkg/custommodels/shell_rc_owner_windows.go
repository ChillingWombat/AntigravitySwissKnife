//go:build windows

package custommodels

import "os"

func preserveShellRCOwner(_ *os.File, _ os.FileInfo) error {
	return nil
}
