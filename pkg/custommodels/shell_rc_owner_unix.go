//go:build unix

package custommodels

import (
	"fmt"
	"os"
	"syscall"
)

func preserveShellRCOwner(file *os.File, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("shell RC ownership is unavailable")
	}
	if err := file.Chown(int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}
	return nil
}
