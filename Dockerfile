FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /src

COPY backend/go.mod ./go.mod
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/collectarr ./...

FROM alpine:3.20

RUN apk add --no-cache ffmpeg ca-certificates sqlite-libs
RUN adduser -D -H -u 10001 collectarr

WORKDIR /app

COPY --from=builder /out/collectarr /usr/local/bin/collectarr

EXPOSE 8893

ENTRYPOINT ["collectarr"]
