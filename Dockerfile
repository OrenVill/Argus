# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/argus ./cmd/argus

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S argus && adduser -S argus -G argus

COPY --from=builder /out/argus /usr/local/bin/argus
COPY config/argus.container.yaml /etc/argus/argus.yaml

USER argus

EXPOSE 9090

ENTRYPOINT ["/usr/local/bin/argus"]
CMD ["run", "-c", "/etc/argus/argus.yaml"]
