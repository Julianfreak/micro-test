
# syntax=docker/dockerfile:1.7

FROM golang:1.23-alpine AS build

WORKDIR /src
RUN mkdir -p /out && cp /etc/ssl/certs/ca-certificates.crt /out/runtime-ca-certificates.crt
COPY go.mod go.sum ./
RUN --mount=type=secret,id=go-ca,target=/tmp/host-ca-bundle.crt,required=false \
    if [ -s /tmp/host-ca-bundle.crt ]; then \
      cat /tmp/host-ca-bundle.crt >> /etc/ssl/certs/ca-certificates.crt; \
    fi && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/micro-test .

FROM scratch
COPY --from=build /out/micro-test /micro-test
COPY --from=build /out/runtime-ca-certificates.crt /etc/ssl/cert.pem
USER 65532:65532

EXPOSE 8089
ENTRYPOINT ["/micro-test"]
