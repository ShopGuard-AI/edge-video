# Examples

This directory contains example code for using Edge Video.

## Python Examples

### consumer_basic.py
Basic consumer that receives frames from RabbitMQ and fetches them from Redis.

**Usage:**
```bash
cd python
python consumer_basic.py
```

**Requirements:**
```bash
pip install pika redis opencv-python
```

### consumer_status_monitor.py
Advanced consumer that monitors camera status and system events.

**Usage:**
```bash
cd python
python consumer_status_monitor.py
```

## Go Examples

### validate-config
Utility to validate configuration files.

**Usage:**
```bash
cd go/validate-config
go run main.go ../../configs/config.example.toml
```

## Documentation

For more examples and detailed documentation, see:
- [Getting Started Guide](../docs/getting-started/)
- [API Documentation](../docs/api/)
- [Integration Guide](../docs/guides/)
