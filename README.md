# Standard Notes Sync Server

Golang implementation of the [Standard File](https://standardfile.org/) protocol
and backend syncing API for standardnotes.

This project started out as a fork of https://github.com/tectiv3/standardfile,
but has since been heavily rewritten and rearchitected. You can run your own
sync server and use it with a Standard Notes client. This allows you to have
100% control of your data.

## Getting started

#### Requirements

- Go 1.16+
- SQLite3 database

#### Initialize project

```
make deps
make build
```

#### Example CLI usage

```sh
# Start the server in the foreground
./bin/standardnotes api

# Start the server as background daemon
./bin/standardnotes api -d

# Stop the background daemon
./bin/standardnotes api -stop
```

There is some other configuration you can specify either via a flag or a JSON
configuration file. An options set with a CLI flag will override the same option
in the configuration file. Read more about flags, options:

```
./bin/standardnotes -h
./bin/standardnotes api -h
```

## Deployment

#### nginx sample config

This should be behind an https-enabled location.

```
server {
  server_name foo.example.com;
  listen 80;
  return 301 https://$server_name$request_uri;
}

server {
  server_name foo.example.com;
  listen 443 ssl http2;

  # other SSL stuff...

  location / {
    proxy_pass http://localhost:8888;

    add_header Access-Control-Allow-Origin 'https://app.standardnotes.org' always;
    add_header Access-Control-Allow-Headers 'authorization,content-type' always;
    add_header Access-Control-Allow-Methods 'GET, POST, PUT, PATCH, DELETE, OPTIONS' always;
    add_header Access-Control-Expose-Headers 'Access-Token, Client, UID' always;

    if ($request_method = OPTIONS ) {
      return 200;
    }
  }
}
```

## Optional Environment variables

- `SECRET_KEY_BASE="JWT secret key"`

## Contributing

Contributions are encouraged and welcome.

## License

Licensed under MIT
