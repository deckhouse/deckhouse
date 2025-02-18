# Upmeter

## What is it?

Upmeter is a Deckhouse status dashboard and status page.

This project is the backend. The resulting docker image is used as the server and agents. Agents probe kubernetes and custom
features within the cluster. The server collects and serves probe statuses to 'webui' and 'status' web apps.

## Package name

The package is named `d8.io/upmeter` to distinguish its name from Go standard library packages.

## Development

Use makefile to install dependencies, run tests, and build the project locally.

Local run can be achieved with emulator mode. Emulator mode is a mode when the agent runs in a loop and generates random statuses.

Install the migrator

```sh
CGO_ENABLED=1 go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.15
```

Prepare the database for agent and run the agent in emulator mode

```sh
migrate -verbose -path ./pkg/db/migrations/agent/ -database 'sqlite3://./db-emu-agent.sqlite?x-no-tx-wrap=true' up

go run ./cmd/upmeter/ agent --emulation --db-path=./db-emu-agent.sqlite
```

Prepare the database for server and run the server

```sh
migrate -verbose -path ./pkg/db/migrations/server/ -database 'sqlite3://./db-emu-server.sqlite?x-no-tx-wrap=true' up

go run ./cmd/upmeter/ start --origins=1 --db-path=./db-emu-server.sqlite
```

Open the test page http://localhost:8091/, API is available at http://localhost:8091/api/status/range for inspection, e.g. http://localhost:8091/api/status/range?from=1722278000&to=1723278000&step=300&group=control-plane&probe=__total__
