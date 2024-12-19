FROM golang:1.23.4-alpine3.21

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o ./discord_bot

CMD ["./discord_bot"]