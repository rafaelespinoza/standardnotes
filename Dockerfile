# build stage
FROM golang:alpine AS build-env
RUN apk update && apk --no-cache add gcc g++ git
WORKDIR /src

ARG BUILD_TIME
ARG GIT_BRANCH
ARG SF_VERSION

RUN git clone --branch $GIT_BRANCH \
	--depth 1 \
	https://github.com/rafaelespinoza/standardfile.git \
	/src
RUN go mod download
RUN go mod verify
RUN go build -ldflags="-w -X main.BuildTime=${BUILD_TIME} -X main.Version=${SF_VERSION}" \
	-o /src/bin/sf

# final stage
FROM alpine
RUN mkdir -p /data
WORKDIR /app
COPY --from=build-env /src/bin/sf /src/config/standardfile.json /app/
VOLUME /data
EXPOSE 8888
ENTRYPOINT ["/app/sf", "-config", "/app/standardfile.json", "-db", "/data/sf.db", "api"]
