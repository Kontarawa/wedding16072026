# build
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod main.go ./
COPY models ./models
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /server .

# run
FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=build /server ./server
COPY index.html ./
COPY static ./static
ENV PORT=8080
EXPOSE 8080
USER 65534:65534
ENTRYPOINT ["./server"]
