FROM golang:1.20 AS builder

WORKDIR /image-board

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o image-board-app .

FROM debian:bullseye-slim

WORKDIR /image-board

COPY --from=builder /image-board/image-board-app .

EXPOSE 3000

CMD ["./image-board-app"]
