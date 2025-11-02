# Stage 1: Build
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache build-base pkgconfig opencv opencv-dev

WORKDIR /app

# Copiar go.mod e go.sum
COPY go.mod go.sum ./

# Copiar o código fonte para resolver dependências locais
COPY . .

# Baixar e sincronizar dependências
RUN go mod download
RUN go mod tidy

# Construir a aplicação com CGO ativado para GoCV/OpenCV
ENV CGO_ENABLED=1
RUN go build -o /camera-collector main.go

# Stage 2: Final image
FROM alpine:latest

# Instala runtime do OpenCV
RUN apk add --no-cache opencv

WORKDIR /app

# Copiar o binário construído
COPY --from=builder /camera-collector .

# Copiar o arquivo de configuração
COPY config.yaml .

# Comando para iniciar a aplicação
CMD ["./camera-collector"]
