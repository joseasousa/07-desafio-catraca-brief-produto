FROM golang:1.27-alpine AS source
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM source AS build
RUN CGO_ENABLED=0 go build -trimpath -o /app .

FROM alpine:3.22
WORKDIR /data
COPY --from=build /app /usr/local/bin/catraca
EXPOSE 8080
ENTRYPOINT ["catraca"]
