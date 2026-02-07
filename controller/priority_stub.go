//go:build !linux

package controller

// Solusi Modern:
// Gunakan build tag "!linux".
// Artinya: File ini akan dipakai di Windows, macOS, BSD, dll.
// File 'priority_linux.go' yang akan menangani Linux (Oracle VPS).
// Ini menutup celah build error di macOS/Dev machine.

func SetPriority(int) error {
	// Stable Strategy: "Silent Success"
	// Mengembalikan nil (tidak error) meskipun tidak melakukan apa-apa.
	// Ini mencegah aplikasi crash/panic hanya karena dijalankan di OS 
	// yang tidak mendukung pengaturan prioritas (seperti saat tes lokal).
	return nil
}

func SetWorstPriority() {
	// No-op (No Operation).
	// Biarkan kosong. Tidak perlu melakukan apa-apa di OS non-Linux.
}
