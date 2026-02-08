# NeverIdle — Prestige Waster for Always-Active VPS

**Repository:** https://github.com/merisssas/nv

NeverIdle adalah utilitas ringan untuk menjaga VM/VPS tetap aktif dengan pola beban CPU, memori, disk I/O, dan bandwidth yang bisa diatur. Cocok untuk environment yang berisiko di-reclaim saat idle, atau untuk mempertahankan “activity footprint” yang terlihat natural.

> **Tagline:** _Prestige waster_ — stabil, terukur, dan fleksibel di semua kelas mesin.

---

## Daftar Isi

- [Ringkasan](#ringkasan)
- [Kelebihan Utama](#kelebihan-utama)
- [Fitur Utama](#fitur-utama)
- [Quick Start](#quick-start)
- [Matriks Fitur](#matriks-fitur)
- [Instalasi](#instalasi)
- [Cara Pakai](#cara-pakai)
- [Profil & Preset](#profil--preset)
- [Konfigurasi Lengkap (Flags)](#konfigurasi-lengkap-flags)
- [Tutorial Lengkap](#tutorial-lengkap)
- [Docker (Build, Run, Compose)](#docker-build-run-compose)
- [Praktik Terbaik](#praktik-terbaik)
- [Troubleshooting](#troubleshooting)
- [Keamanan & Catatan Penting](#keamanan--catatan-penting)

---

## Ringkasan

NeverIdle menggabungkan CPU burner, memory allocator, disk writer, dan network activity dalam satu binary lintas platform (Linux/macOS/Windows). Ia bisa berperilaku “realistic” melalui **Baseline Profile** atau **Smart Profile** yang adaptif terhadap kemampuan mesin. Semua worker berjalan aman dengan graceful shutdown, lock file, dan opsi prioritas agar beban tidak mengganggu layanan utama.

---

## Kelebihan Utama

- **All-in-one**: CPU, memori, disk, dan bandwidth dalam satu binary.
- **Adaptif**: auto pause/resume dengan hysteresis untuk CPU/memori.
- **Realistic**: baseline profile dengan pola traffic & memory touch natural.
- **Aman**: graceful shutdown, lock file, dan batas cgroup-aware.
- **Portabel**: bisa dijalankan di Linux/macOS/Windows.
- **Operasional**: cocok untuk VM/VPS kecil sampai besar.

---

## Fitur Utama

- CPU load target + burst/rest pattern
- Memory allocation (GiB) **atau** target persentase
- Disk I/O burst terjadwal
- Speedtest (bandwidth waste) dengan interval
- Baseline web server + TLS ACME
- Maintenance scheduler (random compute/network/memory)
- Smart profile: deteksi spesifikasi + target CPU/mem otomatis
- Priority tuning + lock file

---

## Quick Start

### 1) Mode “Smart” (disarankan)
Target **20–40% CPU** + **~30% memori** dan menampilkan spesifikasi VPS, plus network download ringan setiap 5–10 menit.

```bash
./NeverIdle -smart
```

### 2) Mode Baseline (realistic footprint)

```bash
./NeverIdle -baseline
```

### 3) CPU + Memori + Network minimal

```bash
./NeverIdle -cp 20 -c 1s -m 2 -n 4h
```

---

## Matriks Fitur

| Komponen | Mode | Deskripsi | Default | Catatan |
|---|---|---|---|---|
| CPU | `-cp` + `-c` | Target CPU dengan burst/rest | 35% target (jika hanya `-cp`) | Bisa auto pause/resume dengan `-auto` |
| CPU | `-baseline` | Pola burst 40–70% + cooldown/idle | Aktif di baseline | Realistic traffic footprint |
| CPU | `-smart` | 20–40% CPU dengan auto pause/resume | Aktif di smart | Cocok semua VPS |
| Memori | `-m` | Alokasi GiB tetap | Off | Tidak bisa dengan `-mp` |
| Memori | `-mp` | Target persentase memori | Off | Cgroup-aware |
| Memori | `-smart` | Target ~30% memori | Aktif di smart | Auto adjust |
| Disk | `-d` + `-di` | Disk write burst (MiB/interval) | Off | `-path` untuk folder |
| Network | `-n` | Speedtest periodik | Off | Perlu akses internet |
| Network | `-network-mode` | `speedtest`, `speedtest-random`, `speedtest-worst`, `download` | `speedtest` | `download` untuk fetch file kecil |
| Web | `-baseline` | Dummy web server | Port 8080 | Bisa TLS dengan ACME |
| Scheduler | `-maintenance` | Random compute/network/memory | Off | Pattern natural |

---

## Instalasi

### Opsi A: Download release binary
1. Buka halaman Releases: https://github.com/merisssas/nv/releases
2. Download binary sesuai arsitektur (amd64/arm64).
3. Jadikan executable:

```bash
chmod +x NeverIdle
```

### Opsi B: Build dari source

```bash
git clone https://github.com/merisssas/nv.git
cd nv
go build -o NeverIdle
```

### Opsi C: Docker

```bash
# ARM (default)
sudo docker build -t neveridle:latest .
# AMD64
sudo docker build --build-arg ARCH=amd64 -t neveridle:latest .
```

---

## Cara Pakai

> **Catatan:** nilai persentase menggunakan angka bulat (contoh `20` berarti 20%).

```bash
./NeverIdle -smart
```

Saat program start, setiap worker akan menjalankan satu siklus awal agar kamu bisa langsung melihat efeknya.

---

## Profil & Preset

### 1) Smart Profile (recommended)
- Menampilkan spesifikasi CPU & memori
- Target CPU 20–40% (burst pattern)
- Target memori ~30%
- Auto pause/resume aktif
- **Note (EN):** Smart mode enables CPU + memory + light network download. Disk I/O runs only if you pass `-d` (and optionally `-di`/`-path`).

```bash
./NeverIdle -smart
```

### 2) Baseline Profile (realistic)
- Dummy web server (`/` dan `/healthz`)
- CPU burst 40–70% lalu cooldown/idle
- Memory touch 300–800 MiB setiap 5–15 menit
- Network faker (DNS + HTTP ringan)

```bash
./NeverIdle -baseline
```

### 3) Custom Profile (manual)

```bash
./NeverIdle -cp 25 -c 1s -mp 60 -n 6h -t 4 -d 1024 -di 1h -path /var/tmp
```

---

## Konfigurasi Lengkap (Flags)

### Smart Profile
| Flag | Deskripsi | Default |
|---|---|---|
| `-smart` | Deteksi spesifikasi + target 20–40% CPU dan ~30% memori | Off |

### CPU
| Flag | Deskripsi | Default |
|---|---|---|
| `-cp` | Target CPU percent | 35% (jika `-cp` dipakai) |
| `-c` | Interval kontrol CPU | 1s |
| `-auto` | Auto pause/resume CPU | Off |
| `-auto-interval` | Interval auto check | 2s |
| `-auto-hyst` | Hysteresis percent | 2 |

### Memory
| Flag | Deskripsi | Default |
|---|---|---|
| `-m` | Alokasi memori GiB tetap | Off |
| `-mp` | Target persentase memori | Off |

> `-m` dan `-mp` tidak bisa dipakai bersamaan.

### Network
| Flag | Deskripsi | Default |
|---|---|---|
| `-n` | Interval speedtest (atau interval fixed untuk `download`) | Off |
| `-t` | Concurrent connections | 4 |
| `-network-mode` | Mode network: `speedtest`, `speedtest-random`, `speedtest-worst`, `download` | `speedtest` |
| `-n-min` | Interval minimum untuk `download` | 5m (smart) |
| `-n-max` | Interval maksimum untuk `download` | 10m (smart) |

### Baseline Web Server
| Flag | Deskripsi | Default |
|---|---|---|
| `-baseline` | Aktifkan baseline profile | Off |
| `-web-addr` | Bind address | 0.0.0.0 |
| `-web-port` | Port web server | 8080 |
| `-web-tls` | TLS otomatis ACME | Off |
| `-web-domain` | Domain TLS (wajib jika TLS) | "" |

### Disk
| Flag | Deskripsi | Default |
|---|---|---|
| `-d` | Disk write MiB per interval | Off |
| `-di` | Interval disk write | 30m |
| `-path` | Folder target disk write | current directory |

### Maintenance + Safety
| Flag | Deskripsi | Default |
|---|---|---|
| `-maintenance` | Scheduler randomized | Off |
| `-heartbeat-url` | Heartbeat URL | Off |
| `-lock-file` | Lock file path | /tmp/neveridle.lock |
| `-p` | Priority (0=normal, 19=lowest, 666=background) | 0 |

---

## Tutorial Lengkap

## Tips Tuning

- Kalau CPU usage masih di bawah 20–25%, naikkan **burst percentage** (`-cp`) atau **durasi burst** di konfigurasi CPU (lihat `BurstMin/BurstMax` di `main.go`). 
- Untuk VM besar (contoh 4 OCPU / 24 GB), pastikan memory waste tidak terlalu rendah: target **minimal 5–6 GiB stabil** dengan `-m 6` atau sesuaikan `-mp` agar konsumsi stabil. 
- Untuk network ringan, gunakan `-network-mode download` agar melakukan download file kecil periodic (contoh 5–10 menit). 

### 1) Menjaga VM tetap aktif (minimal)

```bash
./NeverIdle -cp 20 -c 1s -m 2 -n 4h
```

**Output yang diharapkan:** CPU stabil, memori alokasi tetap, dan speedtest berjalan tiap 4 jam.

---

### 2) Smart mode untuk semua jenis VPS

```bash
./NeverIdle -smart
```

**Keunggulan:** deteksi spesifikasi otomatis, beban aman di kisaran 20–40% CPU.

---

### 3) Baseline mode (profil realistis)

```bash
./NeverIdle -baseline
```

Jika ingin TLS:

```bash
./NeverIdle -baseline -web-tls -web-domain example.com
```

---

### 4) Auto pause/resume CPU (adaptive)

```bash
./NeverIdle -cp 25 -c 1s -auto -auto-interval 2s -auto-hyst 3
```

---

### 5) Target persentase memori (auto)

```bash
./NeverIdle -mp 60 -auto-interval 2s -auto-hyst 5
```

---

### 6) Disk write burst

```bash
./NeverIdle -d 1024 -di 30m -path /var/tmp
```

---

### 7) Maintenance scheduler (randomized)

```bash
./NeverIdle -maintenance
```

Optional heartbeat:

```bash
./NeverIdle -maintenance -heartbeat-url "https://example.com/heartbeat"
```

---

### 8) Kombinasi “Prestige Waster” (komplit)

```bash
./NeverIdle \
  -baseline \
  -smart \
  -n 6h -t 4 \
  -d 1024 -di 1h -path /var/tmp \
  -p 666
```

**Efek:** baseline + smart control + disk/network + prioritas rendah.

---

## Docker (Build, Run, Compose)

### Build Image

```bash
git clone https://github.com/merisssas/nv.git
cd nv
docker build -t neveridle:latest .
```

### Run Container

```bash
docker run -d --name neveridle \
  --restart unless-stopped \
  neveridle:latest -smart
```

### Contoh Lengkap (Manual → Docker → Compose)

#### Manual (binary langsung)

```bash
# Smart (CPU + memori)
./NeverIdle -smart

# Baseline (web + cpu + memory touch + network faker)
./NeverIdle -baseline

# Custom manual (CPU + mem + network + disk)
./NeverIdle -cp 25 -c 1s -mp 40 -n 6h -t 4 -d 1024 -di 1h -path /var/tmp
```

#### Docker run (smart + disk I/O)

```bash
docker run -d --name neveridle \
  --restart unless-stopped \
  -v /var/tmp:/var/tmp \
  neveridle:latest -smart -d 1024 -di 1h -path /var/tmp
```

### Compose

```yaml
services:
  neveridle:
    image: ghcr.io/merisssas/nv:latest
    container_name: neveridle
    restart: unless-stopped
    command: ["-smart", "-d", "1024", "-di", "1h", "-path", "/var/tmp"]
    volumes:
      - /var/tmp:/var/tmp
```

---

## Praktik Terbaik

- Gunakan `-smart` untuk semua jenis VPS agar stabil & adaptif.
- Gunakan `-auto` jika VPS menjalankan workload lain.
- Untuk disk write, selalu arahkan ke volume yang aman (`/var/tmp`).
- Jika memakai TLS, pastikan port 80/443 terbuka.
- Jalankan di `tmux`/`screen` agar proses tetap aktif.

---

## Troubleshooting

- **Program keluar segera:** tidak ada worker yang diaktifkan. Gunakan `-smart`, `-baseline`, atau minimal `-cp`/`-m`/`-mp`/`-n`/`-d`.
- **TLS tidak jalan:** domain tidak resolve atau port 80/443 tertutup.
- **Speedtest gagal:** cek akses outbound ke Ookla SpeedTest.
- **RAM terasa penuh:** kurangi `-mp` atau gunakan `-auto`.

---

## Keamanan & Catatan Penting

- Gunakan beban sesuai kapasitas VM agar tidak mengganggu layanan inti.
- Jangan jalankan multiple instance tanpa `-lock-file` yang berbeda.
- Untuk environment sensitif, hindari network speedtest.

---

**NeverIdle = prestige waster terbaik untuk VM/VPS yang selalu terlihat hidup.**
