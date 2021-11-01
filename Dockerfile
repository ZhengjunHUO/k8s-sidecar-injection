FROM golang:1.17-alpine AS build
WORKDIR /workspace
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY pkg/ ./pkg/
COPY cmd/main.go .
RUN CGO_ENABLED=0 go build -o sidecar-injection .

FROM alpine:3.14
LABEL maintainer="Zhengjun HUO"
COPY --from=build /workspace/sidecar-injection /usr/local/bin/sidecar-injection
ENTRYPOINT ["/usr/local/bin/sidecar-injection"]
