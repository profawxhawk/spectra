FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /spectra ./cmd/spectra

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /spectra /usr/local/bin/spectra

EXPOSE 6666 8080

ENTRYPOINT ["spectra"]
CMD ["serve"]
