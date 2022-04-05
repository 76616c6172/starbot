# Stage 1, build the binary in a container
FROM golang:1.17.7-alpine as builder
RUN apk add build-base
WORKDIR /src

## Get dependencies
COPY go.mod go.sum ./
RUN go mod download
COPY . .

## Compile static binary
RUN go build -ldflags '-linkmode external -extldflags "-fno-PIC -static"' .

# Stage 2, build the final container with the minimum required files
FROM alpine as release
WORKDIR /src
COPY --from=builder /src/Starbot /src/run_bot /src/
COPY --from=builder /src/keys/ /src/keys
COPY --from=builder /src/data/ /src/data

ENTRYPOINT ["/src/run_bot"]