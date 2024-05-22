FROM golang:latest AS builder

WORKDIR /app

ENV PORT=8080

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

ENV GO_ENV=production \
    SUPABASE_URL=xxxxxx \
    SUPABASE_KEY=xxxxxxx \
    DOMAIN=xxxxxx \
    GITHUB_TOKEN=xxxxxxx

RUN CGO_ENABLED=0 GOOS=linux go build -o app .

CMD ["./app"]