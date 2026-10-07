# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /out/coterie ./apps/server

FROM alpine:3.20
RUN adduser -D -u 10001 coterie
USER coterie
COPY --from=build /out/coterie /usr/local/bin/coterie
EXPOSE 8080
ENTRYPOINT ["coterie"]
