# Development Environment Guide

This guide explains how to set up and run the local development environment.

## Prerequisites

-   [Docker](https://docs.docker.com/get-docker/)
-   [`just`](https://github.com/casey/just) (a command runner)
-   [`vector`](https://vector.dev/docs/setup/installation/) (for log ingestion)

## Setup and Running

1.  **Start Services:**
    Spin up the necessary background services (local OIDC server - Dex, ClickHouse server) using Docker Compose via `just`:
    ```bash
    just dev-docker
    ```

2.  **Run Frontend:**
    In a separate terminal tab, start the frontend development server:
    ```bash
    just run-frontend
    ```

3.  **Run Backend:**
    In another separate terminal tab, start the backend development server:
    ```bash
    just run-backend
    ```

## Ingesting Dummy Logs

To send sample logs to the running system, you can use `vector`. Open a new terminal tab and run one of the following commands:

-   **HTTP Logs:**
    ```bash
    vector -c http.toml
    ```

-   **Syslog Logs:**
    ```bash
    vector -c syslog.toml
    ```

This will start ingesting the respective dummy log data into ClickHouse via the configured Vector pipelines. 