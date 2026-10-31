FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/service .

FROM alpine:3.22
RUN adduser -D -u 1001 app
COPY --from=build /out/service /usr/local/bin/service
USER app
EXPOSE 8081
ENTRYPOINT ["service"]
