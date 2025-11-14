# Edge Video for Windows

## Installation

1. Download `EdgeVideoSetup-X.X.X.exe` from releases
2. Run the installer as Administrator
3. Follow the installation wizard
4. Configure cameras in `config\config.toml`

## Service Management

The Edge Video service can be managed using several methods:

### Using Services.msc (Windows Services Manager)
1. Press `Win + R`, type `services.msc`, press Enter
2. Find "Edge Video Camera Capture Service"
3. Right-click → Start/Stop/Restart/Properties

### Using Command Line
```cmd
# Install service
edge-video-service.exe install

# Start service  
edge-video-service.exe start
# OR
net start EdgeVideoService

# Stop service
edge-video-service.exe stop  
# OR
net stop EdgeVideoService

# Uninstall service
edge-video-service.exe uninstall

# Test in console mode (for troubleshooting)
edge-video-service.exe console
```

### Using PowerShell
```powershell
# Start service
Start-Service -Name "EdgeVideoService"

# Stop service
Stop-Service -Name "EdgeVideoService"

# Check service status
Get-Service -Name "EdgeVideoService"

# View service logs (Event Viewer)
Get-WinEvent -LogName Application | Where-Object {$_.ProviderName -eq "EdgeVideoService"}
```

## Configuration

Edit `config\config.toml` to configure:

- **Cameras**: Add RTSP URLs for your cameras
- **RabbitMQ**: Configure AMQP connection settings  
- **Redis**: Configure frame storage (optional)
- **Performance**: Adjust workers, quality, resolution

Example configuration:
```toml
target_fps = 30
protocol = "amqp"

[amqp]
amqp_url = "amqp://user:pass@rabbitmq-server:5672/vhost"
exchange = "cameras"

[redis]
enabled = true
address = "redis-server:6379"
username = ""
password = ""
ttl_seconds = 300
prefix = "frames"

[[cameras]]
id = "camera1"
url = "rtsp://admin:password@192.168.1.100:554/stream"

[[cameras]]  
id = "camera2"
url = "rtsp://admin:password@192.168.1.101:554/stream"
```

**Important**: After changing configuration, restart the service:
```cmd
net stop EdgeVideoService
net start EdgeVideoService
```

## Logging

Service logs are written to Windows Event Log:

1. Open Event Viewer (`eventvwr.msc`)
2. Navigate to Windows Logs → Application
3. Filter by Source: "EdgeVideoService"

Log levels:
- **Info**: Normal operation (service start/stop, camera status)
- **Warning**: Non-critical issues (frame drops, retries)  
- **Error**: Critical errors (connection failures, crashes)

## Troubleshooting

### Service won't start
1. Check Event Viewer for error messages
2. Verify configuration file syntax: `config\config.toml`
3. Test in console mode: `edge-video-service.exe console`
4. Check if required ports are available (RabbitMQ/Redis)

### Camera connection issues
- Verify RTSP URLs are accessible
- Check network connectivity to cameras
- Ensure camera credentials are correct
- Test with VLC or FFmpeg directly

### High CPU/Memory usage
- Reduce `target_fps` in config
- Increase `frame_quality` (lower quality = less CPU)
- Reduce `frame_resolution`
- Check if storage (Redis) is causing bottlenecks

### Registration failures
- Verify `api_url` is accessible from Windows machine
- Check Windows Firewall settings
- Ensure API endpoint accepts POST requests

## Firewall Configuration

Edge Video may need the following ports:
- **554/tcp**: RTSP camera connections
- **5672/tcp**: RabbitMQ (AMQP)
- **6379/tcp**: Redis (if enabled)  
- **Custom**: API registration endpoint

Add Windows Firewall rules as needed:
```cmd
netsh advfirewall firewall add rule name="Edge Video RTSP" dir=out action=allow protocol=TCP remoteport=554
netsh advfirewall firewall add rule name="Edge Video AMQP" dir=out action=allow protocol=TCP remoteport=5672
```

## Uninstallation

1. Open "Add or Remove Programs" 
2. Find "Edge Video" → Uninstall
3. OR run: `uninstall.exe` from installation directory

The uninstaller will:
- Stop and remove the Windows service
- Remove program files
- Ask whether to keep configuration files
- Remove Start Menu shortcuts

## Performance Tuning

For optimal performance on Windows:

### System Configuration
- Ensure Windows is not in "Power Saver" mode
- Disable Windows automatic updates during operation hours
- Consider disabling Windows Defender real-time scanning for Edge Video folder

### Edge Video Configuration  
```toml
[optimization]
max_workers = 20              # Adjust based on CPU cores
buffer_size = 200             # Increase for high-traffic cameras  
frame_quality = 5             # Balance quality vs performance
use_persistent = true         # Better for stable connections
circuit_max_failures = 3     # Faster failure detection
```

### Hardware Recommendations
- **CPU**: Multi-core processor (4+ cores recommended)
- **Memory**: 4GB+ RAM (8GB+ for many cameras)
- **Network**: Gigabit Ethernet for multiple HD cameras
- **Storage**: SSD for better I/O performance

## Support

- GitHub Issues: https://github.com/T3-Labs/edge-video/issues
- Documentation: https://github.com/T3-Labs/edge-video/docs
- Configuration Examples: https://github.com/T3-Labs/edge-video/examples