# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/wedding-invitation .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/wedding-invitation /app/wedding-invitation
COPY static ./static
ENV LISTEN_ADDR=:8080
ENV DATA_DIR=/app/data
ENV GUEST_DB=/app/data/guests.json
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/app/wedding-invitation"]
