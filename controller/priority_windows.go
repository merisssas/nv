//go:build windows

package controller

import "golang.org/x/sys/windows"

func SetPriority(nice int) error {
	// PENTING:
	// Windows tidak menggunakan konsep angka -20 s/d 19 seperti Linux.
	// Windows menggunakan "Priority Class" (Konstanta Hex).
	// Kita harus memetakan (Mapping) nilai 'nice' Linux ke Class Windows
	// agar tidak terjadi error "Invalid Parameter".

	var priorityClass uint32 = windows.NORMAL_PRIORITY_CLASS

	// Logic Mapping:
	// Semakin tinggi 'nice', semakin rendah prioritasnya.
	switch {
	case nice >= 15:
		// Nice tinggi (sangat santai) -> IDLE
		priorityClass = windows.IDLE_PRIORITY_CLASS
	case nice >= 5:
		// Nice sedang -> BELOW NORMAL
		priorityClass = windows.BELOW_NORMAL_PRIORITY_CLASS
	case nice <= -10:
		// Nice negatif (sangat prioritas) -> HIGH
		priorityClass = windows.HIGH_PRIORITY_CLASS
	case nice < 0:
		// Sedikit prioritas -> ABOVE NORMAL
		priorityClass = windows.ABOVE_NORMAL_PRIORITY_CLASS
	default:
		// 0 atau mendekati 0 -> NORMAL
		priorityClass = windows.NORMAL_PRIORITY_CLASS
	}

	handle := windows.CurrentProcess()
	return windows.SetPriorityClass(handle, priorityClass)
}

func SetWorstPriority() {
	// Langsung paksa ke IDLE Class (Level terendah di Windows)
	// Setara dengan Nice 19 di Linux.
	_ = windows.SetPriorityClass(windows.CurrentProcess(), windows.IDLE_PRIORITY_CLASS)
}
