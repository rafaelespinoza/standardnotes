#
# build stage
#
FROM golang:alpine AS build-env
RUN apk update && apk --no-cache add curl gcc g++ git
WORKDIR /src

# Use the Makefile to place all the relevant files for copying.
COPY go.mod /src
RUN go mod download && go mod verify
COPY . /src
RUN go build -o /src/standardnotes
# build db migration tool
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -o /src/godfish -i -v github.com/rafaelespinoza/godfish/mysql/godfish

#
# final stage
#
FROM alpine
WORKDIR /app
RUN mkdir -p /app/db/migrations /app/config
COPY --from=build-env /src/standardnotes /src/godfish /app/bin/
COPY --from=build-env /src/internal/db/migrations /app/db/migrations/
COPY --from=build-env /src/internal/config/standardnotes.json /app/config/
EXPOSE 8888
ENTRYPOINT ["/app/bin/standardnotes"]
