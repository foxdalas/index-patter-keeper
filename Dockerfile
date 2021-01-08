FROM golang:alpine as build

RUN apk add git

WORKDIR /app
COPY go.mod go.sum /app/
RUN go mod download
COPY . .
RUN apk add alpine-sdk
RUN go build -o index-pattern-keeper .

FROM alpine:3.12
RUN apk --no-cache add ca-certificates
COPY --from=build /app/index-pattern-keeper /bin/
ENTRYPOINT ["/bin/index-pattern-keeper"]
