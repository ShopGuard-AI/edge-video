# Configuration Files

This directory contains all configuration files for Edge Video.

## Configuration Files

### config.example.toml
Basic configuration template. Copy this file to `config.toml` in the root directory and customize it.

```bash
cp config.example.toml ../config.toml
```

### config.memory-control.toml
Configuration optimized for memory-constrained environments (e.g., Windows with limited RAM).

Includes:
- Memory controller settings
- Optimized buffer sizes
- Throttling configuration

### config.test.toml
Configuration for running tests.

## Docker Compose

### docker-compose/docker-compose.yml
Production Docker Compose setup with RabbitMQ, Redis, and Edge Video.

**Usage:**
```bash
cd docker-compose
docker-compose up -d
```

### docker-compose/docker-compose.test.yml
Testing environment with additional debugging tools.

## Quick Start

1. Copy example configuration:
   ```bash
   cp configs/config.example.toml config.toml
   ```

2. Edit camera URLs and credentials in `config.toml`

3. Run:
   ```bash
   ./edge-video --config config.toml
   ```

## Documentation

For detailed configuration options, see:
- [Configuration Guide](../docs/getting-started/configuration.md)
- [Memory Control](../docs/MEMORY-CONTROL.md)
- [Multi-tenancy Setup](../docs/guides/vhost-implementation.md)
