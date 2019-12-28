# build stage
FROM golang:alpine AS build-env
RUN apk update && apk --no-cache add gcc g++ git
WORKDIR /src

# NOTE: Use the Makefile to place all the relevant files for copying.
COPY go.mod /src
RUN go mod download
RUN go mod verify
COPY . /src
RUN go build -o /src/bin/sf

# final stage
FROM alpine
RUN mkdir -p /data
WORKDIR /app
COPY --from=build-env /src/bin/sf /src/config/standardfile.json /app/
VOLUME /data
EXPOSE 8888
ENTRYPOINT ["/app/sf", "-config", "/app/standardfile.json", "-cors", "-db", "/data/sf.db", "api"]
