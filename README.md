<p align="center">
  <h1 align="center">🔍 Logchef</h1>
  <p align="center">A modern, high-performance log analytics platform</p>
</p>

<p align="center">
  <a href="https://demo.logchef.app"><strong>Try Demo</strong></a> · 
  <a href="https://logchef.app"><strong>Read Docs</strong></a>
</p>

---

Logchef is a lightweight, powerful log analytics platform that runs as a single binary. It leverages Clickhouse for blazing-fast log storage and provides an intuitive interface for querying and visualizing logs, making it perfect for development teams of all sizes.

## ✨ Features

- **Schema-Agnostic Log Exploration**: Query any Clickhouse table regardless of schema
- **Flexible Query Options**: Simple search syntax or full Clickhouse SQL
- **High Performance**: Fast queries on large datasets with minimal resource usage
- **Team-Based Access Control**: Secure multi-tenant log access with fine-grained permissions
- **Single Binary Deployment**: Easy installation and management

## 🚀 Quick Start

### Docker

```shell
# Download the compose file
curl -LO https://github.com/mr-karan/logchef/raw/master/deployment/docker/docker-compose.yml

# Run the services
docker compose up -d
```

Visit `http://localhost:8125` to start exploring your logs.

## 📚 Documentation

For comprehensive documentation, visit [logchef.app](https://logchef.app).

## 📊 Screenshots

<!-- Screenshots to be added -->

## 📜 License

Logchef is licensed under the AGPLv3 License.