# Build stage 🛠️
FROM docker.io/golang:1.27.1-alpine3.24 AS builder

WORKDIR /source

## Download dependencies
COPY go.mod go.sum ./
RUN go mod download

## Build
COPY . .
RUN go build -o dist/serpentarius.bin ./cmd/http/main.go

# Final stage 🚀
FROM docker.io/alpine:3.24.1 AS runner

## Install required system deps
RUN apk --no-cache add \
    chromium \
    fontconfig \
    ttf-liberation \
    tzdata

## Configure Timezone
ENV TZ=America/Bogota

## Update the font cache
RUN fc-cache -f

## Add non-root user
RUN adduser -D -h /opt/serpentarius -s /sbin/nologin serpentarius
WORKDIR /opt/serpentarius
USER serpentarius

## Copy files
COPY --from=builder /source/dist/serpentarius.bin .

## Server
ENV PORT=3001
EXPOSE ${PORT}

## Run
ENTRYPOINT ["/opt/serpentarius/serpentarius.bin"]
