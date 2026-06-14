FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/daxi-cloud-api ./cmd/daxi-cloud-api

FROM debian:bookworm-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates sqlite3 \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/daxi-cloud-api /app/daxi-cloud-api
EXPOSE 8088
CMD ["/app/daxi-cloud-api"]
