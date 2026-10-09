# syntax=docker/dockerfile:1

# The client is built first so the Go stage can embed the output.
FROM node:22-alpine AS web
WORKDIR /web
COPY apps/web/package*.json ./
RUN npm ci
COPY apps/web/ ./
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
# The Vite output lands over the committed placeholder, so the go:embed
# directive always has real files to pack.
COPY --from=web /web/dist/ ./apps/server/web/
RUN CGO_ENABLED=0 go build -o /out/coterie ./apps/server

FROM alpine:3.20
RUN adduser -D -u 10001 coterie
USER coterie
COPY --from=build /out/coterie /usr/local/bin/coterie
# Schema source of truth ships with the image (MIGRATE_ON_START default true).
COPY migrations /migrations
ENV TZ=UTC \
    MIGRATIONS_DIR=/migrations
EXPOSE 8080
ENTRYPOINT ["coterie"]
