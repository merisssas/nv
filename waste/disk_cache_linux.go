//go:build linux

package waste

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func dropFileCache(f *os.File) {
	if f == nil {
		return
	}
	if err := unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED); err != nil {
		fmt.Printf("[DISK] Warning: Failed to drop page cache: %v\n", err)
	}
}
