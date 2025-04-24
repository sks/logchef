<p align="center">
  <h1 align="center">Logchef</h1>
</p>

A modern, high-performance log analytics platform that runs as a single binary.

Logchef combines the power of Clickhouse for high-speed log storage with an intuitive frontend, offering powerful querying capabilities and visualization tools for development teams of all sizes.

## Features

- **Universal Clickhouse UI**: Connect to any Clickhouse table, schema-agnostic
- **Dual Query Modes**: Use simple search syntax or full Clickhouse SQL power
- **High Performance**: Leverage Clickhouse for fast queries on large datasets
- **Team-Centric**: Multi-tenant with user roles, teams, and source access control
- **Low Resource Usage**: Minimal footprint on top of Clickhouse
- **Single Binary Deployment**: Easy installation and management

## Installation

### Docker

```shell
# Download the compose file
curl -LO https://github.com/mr-karan/logchef/raw/master/docker-compose.yml

# Run the services
docker compose up -d
```

Visit `http://localhost:8125`

### Binary

- Download the [latest release](https://github.com/mr-karan/logchef/releases) and extract the binary
- Run `./logchef` and visit `http://localhost:8125`

## Screenshots

<!-- Screenshots to be added -->

## Demo

Visit [demo.logchef.app](https://demo.logchef.app) to see Logchef in action.

## Documentation

Comprehensive documentation is available at [logchef.app](https://logchef.app).

## License

Logchef is licensed under the AGPLv3 License.