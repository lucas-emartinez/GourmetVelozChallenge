# Etapa de construcción
FROM golang:1.24 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Asegurar que el binario sea estático
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app ./cmd/main.go

# Etapa final con Alpine
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/app /app/app

# Exponer el puerto
EXPOSE 8080

# Ejecutar la aplicación
ENTRYPOINT ["/app/app"]
