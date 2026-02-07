//go:build windows

package main

import "os"

func acquireLock(path string) (func(), error) {
	if path == "" {
		return func() {}, nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	return func() {
		_ = file.Close()
	}, nil
}
