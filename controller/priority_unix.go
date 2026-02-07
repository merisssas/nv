//go:build unix && !linux

package controller

import (
	"golang.org/x/sys/unix"
)

// SetPriority untuk OS Unix non-Linux (macOS, BSD, dll).
// Menggunakan library modern 'golang.org/x/sys/unix'.
func SetPriority(nice int) error {
	// PRIO_PROCESS: Mengatur prioritas berdasarkan Process ID.
	// 0: Mengacu pada PID proses saat ini (Current Process).
	return unix.Setpriority(unix.PRIO_PROCESS, 0, nice)
}

func SetWorstPriority() {
	// Set CPU 'niceness' ke level paling rendah (19).
	// Artinya proses ini akan sangat mengalah pada aplikasi lain di sistem.
	// Kita tidak mengatur I/O Priority di sini karena API-nya tidak standar
	// di semua varian Unix (beda dengan Linux).
	SetPriority(19)
}
