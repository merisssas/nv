//go:build !linux

package waste

import "os"

func dropFileCache(*os.File) {
}
