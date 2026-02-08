# Tahap 1: Builder
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY . .

RUN rm -f go.sum && go mod tidy

# ✅ Multi-arch support (Buildx injects TARGETOS/TARGETARCH)
ARG TARGETOS=linux
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o neveridle .

# Tahap 2: Runner
FROM alpine:latest

WORKDIR /root/
RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/neveridle .

ENV TZ=Europe/Berlin

ENTRYPOINT ["./neveridle"]
