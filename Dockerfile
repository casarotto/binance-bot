# Usar uma imagem base do Go
FROM golang:1.23.5-alpine

# Instalar dependências do sistema
RUN apk add --no-cache gcc musl-dev

# Definir diretório de trabalho
WORKDIR /app

# Copiar os arquivos de dependência primeiro
COPY go.mod go.sum ./

# Baixar dependências
RUN go mod download

# Copiar o código fonte
COPY . .

# Compilar a aplicação
RUN go build -o bot cmd/main.go

# Criar diretório para histórico
RUN mkdir -p /app/history

# Comando para executar a aplicação
ENTRYPOINT ["./bot"] 