# Standard File Server, Go Implementation

Golang implementation of the [Standard File](https://standardfile.org/) protocol.

## Running your own server

You can run your own Standard File server, and use it with any SF compatible
client (like Standard Notes). This allows you to have 100% control of your
data. This server implementation is built with Go and can be deployed in
seconds.

_You may require to add `/api` to the url of your server if you plan to use
this server with https://standardnotes.org/_

## Getting started

#### Requirements

- Go 1.12+
- SQLite3 database

#### Instructions

Initialize project:

```
go get github.com/rafaelespinoza/standardfile
go install github.com/rafaelespinoza/standardfile
```

Start the server:

```
standardfile api
```

Stop the server:

```
standardfile api -stop
```

#### Docker instructions

Create a local folder and mount it inside the container:

```
make docker-run
```

This way the data will be keep between container updates. An example docker
compose file is included, run with `make docker-up` it will mount current dir as
data dir.

#### Run server in foreground

Useful when running as systemd service.

```
standardfile api
```

This will not daemonise the service, which might be handy if you want to handle
that on some other level, like with init system, inside docker container, etc.

To stop the service, kill the process or press `ctrl-C` if running in terminal.

#### Run server as background daemon

```
standardfile api -d
```

## Database migrations

To perform migrations run `standardfile db -migrate`

## Deploying to a live server

This should be behind an https-enabled location.

#### nginx sample config

```
server {
    server_name sf.example.com;
    listen 80;
    return 301 https://$server_name$request_uri;
}

server {
    server_name sf.example.com;
    listen 443 ssl http2;

    ssl_certificate /etc/letsencrypt/live/sf.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/sf.example.com/privkey.pem;

    include snippets/ssl-params.conf;

    location / {
      add_header Access-Control-Allow-Origin '*' always;
      add_header Access-Control-Allow-Credentials true always;
      add_header Access-Control-Allow-Headers 'authorization,content-type' always;
      add_header Access-Control-Allow-Methods 'GET, POST, PUT, PATCH, DELETE, OPTIONS' always;
      add_header Access-Control-Expose-Headers 'Access-Token, Client, UID' always;

      if ($request_method = OPTIONS ) {
        return 200;
      }

      proxy_set_header        Host $host;
      proxy_set_header        X-Real-IP $remote_addr;
      proxy_set_header        X-Forwarded-For $proxy_add_x_forwarded_for;
      proxy_set_header        X-Forwarded-Proto $scheme;

      proxy_pass          http://localhost:8888;
      proxy_read_timeout  90;
    }
}
```

## Optional Environment variables

- `SECRET_KEY_BASE="JWT secret key"`

## Contributing

Contributions are encouraged and welcome.

## License

Licensed under MIT
