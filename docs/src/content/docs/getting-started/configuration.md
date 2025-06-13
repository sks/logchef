---
title: Configuration
description: Configure LogChef to match your environment
---

LogChef uses a TOML configuration file to manage its settings. This guide explains all available configuration options.

## Server Settings

Configure the HTTP server and frontend settings:

```toml
[server]
# Port for the HTTP server (default: 8125)
port = 8125

# Host address to bind to (default: "0.0.0.0")
host = "0.0.0.0"

# URL of the frontend application
# Leave empty in production, used only in development
frontend_url = ""

# HTTP server timeout for requests (default: 30s)
http_server_timeout = "30s"
```

## Database Configuration

SQLite database settings for storing metadata:

```toml
[sqlite]
# Path to the SQLite database file
path = "logchef.db"
```

## Authentication

### OpenID Connect (OIDC)

Configure your SSO provider (example using Dex):

```toml
[oidc]
# URL of your OIDC provider
provider_url = "http://dex:5556/dex"

# Authentication endpoint URL (Optional: often discovered via provider_url)
auth_url = "http://dex:5556/dex/auth"

# Token endpoint URL (Optional: often discovered via provider_url)
token_url = "http://dex:5556/dex/token"

# OIDC client credentials
client_id = "logchef"
client_secret = "logchef-secret"

# Callback URL for OIDC authentication
# Must match the URL configured in your OIDC provider
redirect_url = "http://localhost:8125/api/v1/auth/callback"

# Required OIDC scopes
scopes = ["openid", "email", "profile"]
```

### Auth Settings

Configure authentication behavior:

```toml
[auth]
# List of email addresses that have admin privileges
admin_emails = ["admin@corp.internal"]

# Duration of user sessions (e.g., "8h", "24h", "7d")
session_duration = "8h"

# Maximum number of concurrent sessions per user
max_concurrent_sessions = 1
```

## Logging

Configure application logging:

```toml
[logging]
# Log level: "debug", "info", "warn", "error"
level = "info"
```

## AI SQL Generation

Configure settings for AI-powered SQL generation using OpenAI-compatible APIs.

```toml
[ai]
# Enable or disable AI features (default: false)
enabled = true

# --- API Endpoint Configuration ---
# Optional: Base URL for OpenAI-compatible endpoints
# Leave empty for standard OpenAI API
# Examples:
# - OpenRouter: "https://openrouter.ai/api/v1"
# - Azure OpenAI: "https://your-resource.openai.azure.com/"
# - Custom proxy: "https://your-proxy.com/v1"
base_url = ""

# OpenAI API Key (required if AI features are enabled)
# Can also be set via LOGCHEF_AI__API_KEY environment variable
api_key = "sk-your_api_key_here"

# --- Model Parameters ---
# Model to use for SQL generation (default: "gpt-4o")
# Popular options: "gpt-4o", "gpt-4o-mini", "gpt-3.5-turbo"
# For OpenRouter: model names like "openai/gpt-4o", "anthropic/claude-3-sonnet"
model = "gpt-4o"

# Maximum number of tokens to generate (default: 1024)
max_tokens = 1024

# Temperature for generation (0.0-1.0, lower is more deterministic, default: 0.1)
temperature = 0.1
```

### Supported Providers

The AI integration works with any OpenAI-compatible API:

- **OpenAI**: Leave `base_url` empty (default)
- **OpenRouter**: Set `base_url = "https://openrouter.ai/api/v1"`
- **Azure OpenAI**: Configure your Azure endpoint
- **Local Models**: Point to your local OpenAI-compatible server

### API Token Requirements

Your API token needs appropriate permissions for the model you're using. For OpenRouter, make sure your token has access to the specific model.

### Security Considerations

- Store API keys in environment variables for production
- Use the least privileged API tokens possible
- Monitor API usage and costs
- Consider rate limiting for high-traffic deployments

## Environment Variables

All configuration options set in the TOML file can be overridden or supplied via environment variables. This is particularly useful for sensitive information like API keys or for containerized deployments.

Environment variables are prefixed with `LOGCHEF_`. For nested keys in the TOML structure, use a double underscore `__` to represent the nesting.

**Format:** `LOGCHEF_SECTION__KEY=value`

**Examples:**

- Set server port:
  ```bash
  export LOGCHEF_SERVER__PORT=8125
  ```
- Set OIDC provider URL:
  ```bash
  export LOGCHEF_OIDC__PROVIDER_URL="http://dex.example.com/dex"
  ```
- Set admin emails (comma-separated for arrays):
  ```bash
  export LOGCHEF_AUTH__ADMIN_EMAILS="admin@example.com,ops@example.com"
  ```
- Set AI API Key:
  ```bash
  export LOGCHEF_AI__API_KEY="sk-your_actual_api_key_here"
  ```
- Enable AI features and set the model:
  ```bash
  export LOGCHEF_AI__ENABLED=true
  export LOGCHEF_AI__MODEL="gpt-4o"
  ```

Environment variables take precedence over values defined in the TOML configuration file.

## Production Configuration

For production deployments, ensure you:

1. Set appropriate `host` and `port` values
2. Configure a secure `client_secret` for OIDC
3. Set the correct `redirect_url` matching your domain
4. Configure admin emails for initial access
5. Adjust session duration based on your security requirements
6. Set logging level to "info" or "warn"
7. If using AI features, ensure `LOGCHEF_AI__API_KEY` is set securely.

## Example Production Configuration

```toml
[server]
port = 8125
host = "0.0.0.0"
http_server_timeout = "30s"

[sqlite]
path = "/data/logchef.db"

[oidc]
provider_url = "https://dex.example.com"
client_id = "logchef"
client_secret = "your-secure-secret"
redirect_url = "https://logchef.example.com/api/v1/auth/callback"
scopes = ["openid", "email", "profile"]

[auth]
admin_emails = ["admin@example.com"]
session_duration = "8h"
max_concurrent_sessions = 1

[logging]
level = "info"

# AI features configuration (API key should be set via env var)
[ai]
enabled = true
# base_url = ""  # Leave empty for OpenAI, or set for other providers
# api_key = ""   # Use LOGCHEF_AI__API_KEY environment variable
model = "gpt-4o"
max_tokens = 1024
temperature = 0.1
```
