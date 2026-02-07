# Tahap 1: Builder
# KEMBALI KE GO 1.24 (Wajib karena gopsutil butuh versi ini)
FROM golang:1.24-alpine AS builder

# Install git (Wajib untuk fetch dependency)
RUN apk add --no-cache git

WORKDIR /app

# 1. Copy SEMUA source code dari repo
COPY . .

# 2. AUTO-FIX MAGIC COMMAND:
# - rm -f go.sum  : Hapus checksum lama yang bikin error
# - go mod tidy   : Download dependency pakai Go 1.24 (sesuai permintaan library)
RUN rm -f go.sum && go mod tidy

# 3. Build Binary untuk Oracle ARM (Ampere)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o neveridle .

# Tahap 2: Runner
FROM alpine:latest

WORKDIR /root/

# Install CA Certs & Tzdata
RUN apk --no-cache add ca-certificates tzdata

# Copy binary dari tahap builder
COPY --from=builder /app/neveridle .

# Set Timezone Frankfurt
ENV TZ=Europe/Berlin

ENTRYPOINT ["./neveridle"]
CMD ["-cp", "15", "-m", "1", "-n", "1h"]
