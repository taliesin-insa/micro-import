# Dockerfile References: https://docs.docker.com/engine/reference/builder/

# Start from the latest golang base image
# Bad practice but anyway
FROM golang:latest AS builder

# Dépendances nécessaires pour compiler le fichier protocole
RUN apt-get update

# Define directory
ADD src /src
WORKDIR /src/micro-import

# Download dependencies (if you try to build your image without following lines you will see missing packages)
RUN go get -u github.com/gorilla/mux
RUN go get -u github.com/stretchr/testify/assert
RUN go get -u github.com/taliesin-insa/lib-auth

RUN go get -u github.com/prometheus/client_golang/prometheus
RUN go get -u github.com/prometheus/client_golang/prometheus/promauto
RUN go get -u github.com/prometheus/client_golang/prometheus/promhttp

# Build all project statically (prevent some exec user process caused "no such file or directory" error)
ENV CGO_ENABLED=0
RUN go build -o main .

# Build the docker image from a lightest one (otherwise it weights more than 1Go)
FROM alpine:latest

# Expose port 12345 to the outside world
EXPOSE 8080

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy on the executive env
 COPY --from=builder /src/micro-import/ .

# Command to run the executable
CMD ["./main"]

