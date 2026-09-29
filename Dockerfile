# Estágio de build
FROM golang:alpine AS builder

WORKDIR /app

# Traz os arquivos de dependência primeiro para aproveitar o cache do Docker
COPY go.mod go.sum ./
RUN go mod download

# Copia o resto do projeto
COPY . .

# Compila o binário de forma estática, apontando para o arquivo principal (cmd/api)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /api ./cmd/api

# Estágio final (imagem mais leve possível)
FROM alpine:latest

# Ca-certificates é fundamental para o Go fazer requisições HTTPS externas (ex: API do ChatGPT)
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /

# Trazemos apenas o binário compilado para a imagem final
COPY --from=builder /api /api

# A porta padrão onde nossa aplicação sobe (segundo o README)
EXPOSE 8080

# Roda o servidor
ENTRYPOINT ["/api"]
