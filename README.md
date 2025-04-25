<p align="center">
  <h1 align="center">Logchef</h1>
</p>

A modern, high-performance log analytics platform that runs as a single binary.

Logchef combines the power of Clickhouse for high-speed log storage with an intuitive frontend, offering powerful querying capabilities and visualization tools for development teams of all sizes.

## Features

- **Schema-Agnostic Log Viewer**: Connect to and view logs from any Clickhouse table, regardless of schema.
- **Dual Query Modes**: Use simple search syntax or full Clickhouse SQL power
- **High Performance & Low Footprint**: Acts as an efficient view layer on top of Clickhouse, enabling fast queries on large datasets with minimal resource usage.
- **Robust RBAC**: Implement fine-grained access control. Assign users to teams and connect data sources to teams, ensuring secure multi-tenant log access.
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

## Screenshots

<!-- Screenshots to be added -->

## Demo

Visit [demo.logchef.app](https://demo.logchef.app) to see Logchef in action.

## Documentation

Comprehensive documentation is available at [logchef.app](https://logchef.app).

## License

Logchef is licensed under the AGPLv3 License.