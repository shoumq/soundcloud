FROM golang:1.22-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/soundcloud-api ./cmd/api

FROM alpine:3.20

WORKDIR /app

COPY --from=build /bin/soundcloud-api /bin/soundcloud-api

EXPOSE 8080

ENTRYPOINT ["/bin/soundcloud-api"]
