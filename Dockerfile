FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy -v

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -o pansou

FROM alpine:3.19
RUN mkdir -p /app/cache
COPY --from=builder /app/pansou .
CMD ["/app/pansou"]
