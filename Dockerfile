FROM golang:1.26.5-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod verify
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/energy-clicker ./cmd/server

FROM alpine:3.22

RUN adduser -D -u 10001 app
COPY --from=build /out/energy-clicker /usr/local/bin/energy-clicker

USER app
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/energy-clicker"]
