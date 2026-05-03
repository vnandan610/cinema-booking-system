FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cinema-api ./cmd

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app
WORKDIR /app

COPY --from=build /out/cinema-api /app/cinema-api
COPY static /app/static

ENV PORT=8080
ENV STATIC_DIR=/app/static

USER app
EXPOSE 8080

ENTRYPOINT ["/app/cinema-api"]
