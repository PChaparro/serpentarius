# Build stage 🛠️
FROM docker.io/golang:1.24.3-alpine3.21 AS builder

## Install upx
WORKDIR /source
RUN apk --no-cache add git upx

## Download dependencies
COPY go.mod go.sum ./
RUN go mod download

## Build
COPY . .
RUN go build -o dist/serpentarius.bin ./cmd/http/main.go
RUN upx dist/serpentarius.bin

# Final stage 🚀
FROM docker.io/alpine:3.21.3 AS runner

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

## Run
EXPOSE 3000
ENTRYPOINT ["/opt/serpentarius/serpentarius.bin"]