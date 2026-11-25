
FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/micro-test .

FROM scratch
COPY --from=build /out/micro-test /micro-test
COPY --from=build /etc/ssl/cert.pem /etc/ssl/cert.pem
USER 65532:65532

EXPOSE 8089
ENTRYPOINT ["/micro-test"]
