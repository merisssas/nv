//go:build linux

package controller

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// SetPriority mengatur nilai "niceness" proses.
// Range: -20 (High Priority) sampai 19 (Low Priority).
func SetPriority(nice int) error {
	pid := os.Getpid()
	
	// Kita gunakan unix.Setpriority yang standar dan stabil.
	// Jika gagal (misal karena permission), kita return error agar bisa dilog.
	err := unix.Setpriority(unix.PRIO_PROCESS, pid, nice)
	if err != nil {
		return fmt.Errorf("failed to set cpu priority: %w", err)
	}
	return nil
}

// SetWorstPriority mengatur prioritas CPU dan I/O ke level terendah
// agar script ini tidak mengganggu kinerja VPS (misal saat SSH atau update).
func SetWorstPriority() {
	// 1. Set CPU Priority ke level terendah (Nice 19)
	if err := SetPriority(19); err != nil {
		fmt.Printf("[Priority] Warning: Failed to set low CPU priority: %v\n", err)
	}

	// 2. Set I/O Priority (Disk Usage)
	// Masalah di kode lama: unix.IoprioSet sering undefined di ARM64.
	// Solusi Modern: Kita panggil syscall langsung. Ini dijamin 100% jalan di Linux manapun.
	
	const (
		ioprioWhoProcess = 1 // Target: Proses
		ioprioClassBE    = 2 // Class: Best Effort (Bukan Idle, agar tidak macet total)
		ioprioData       = 7 // Level: 7 (Paling rendah di Best Effort)
	)

	// Formula IOPRIO Linux: (Class << 13) | Data
	prioVal := (ioprioClassBE << 13) | ioprioData

	// PANGGIL SYSCALL LANGSUNG (Direct Kernel Call)
	// unix.SYS_IOPRIO_SET adalah konstanta nomor syscall yang pasti ada.
	_, _, errno := unix.Syscall(
		unix.SYS_IOPRIO_SET,
		uintptr(ioprioWhoProcess),
		uintptr(os.Getpid()),
		uintptr(prioVal),
	)

	// Syscall mengembalikan errno != 0 jika gagal
	if errno != 0 {
		fmt.Printf("[Priority] Warning: Failed to set I/O priority (errno %d): %v\n", errno, errno)
	} else {
		// Opsional: Uncomment baris di bawah ini untuk debug
		// fmt.Println("[Priority] I/O priority set to lowest (Best Effort 7)")
	}
}
