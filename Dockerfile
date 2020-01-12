# build stage
FROM golang:alpine AS build-env
RUN apk update && apk --no-cache add gcc g++ git
WORKDIR /src

# NOTE: Use the Makefile to place all the relevant files for copying.
COPY go.mod /src
RUN go mod download
RUN go mod verify
COPY . /src
RUN go build -o /src/bin/standardnotes
# build db migration tool
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -o godfish -i -v github.com/rafaelespinoza/godfish/mysql/godfish

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env \
	/src/bin/standardnotes /src/internal/config/standardnotes.json /src/godfish /app/
COPY --from=build-env /src/internal/db/migrations/ /app/
EXPOSE 8888
ENTRYPOINT ["/app/standardnotes", "api"]
