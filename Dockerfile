# Etapa de construcción (Build)
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Instalar certificados y herramientas base
RUN apk add --no-cache git ca-certificates

# Copiar dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fuente
COPY . .

# Compilar el binario estático
RUN CGO_ENABLED=0 GOOS=linux go build -o micro-test -ldflags="-w -s" main.go

# Etapa final (Runtime)
FROM alpine:latest

WORKDIR /root/

# Copiar certificados desde la etapa de construcción
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/micro-test .
COPY --from=builder /app/docs ./docs

# Exponer puerto
EXPOSE 8089

# Ejecutar aplicación
CMD ["./micro-test"]
