# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copiar go.mod e go.sum e baixar dependências
COPY go.mod go.sum ./
RUN go mod download

# Copiar o código fonte
COPY . .

# Construir a aplicação
# O build para linux/amd64 é o padrão, mas podemos ser explícitos
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /camera-collector main.go

# Stage 2: Final image
FROM alpine:latest

WORKDIR /app

# Copiar o binário construído
COPY --from=builder /camera-collector .

# Copiar o arquivo de configuração
COPY config.yaml .

# Expor a porta se a aplicação tiver um servidor web (opcional)
# EXPOSE 8080

# Comando para iniciar a aplicação
CMD ["./camera-collector"]
