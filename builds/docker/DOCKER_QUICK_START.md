# Quick Start: Docker Testing on ARM Mac

## 🚀 One-Command Testing

```bash
# Make script executable (first time only)
chmod +x test-docker.sh

# Start everything and run tests
./test-docker.sh start

# Stop and cleanup
./test-docker.sh stop
```

## 📋 Manual Steps (Alternative)

### 1. Build and Start
```bash
# Build image
docker build -t proxyrouter:latest .

# Start with docker-compose
docker-compose up -d
```

### 2. Test Services
```bash
# Test API
curl http://localhost:8081/healthz

# Test HTTP proxy
curl -x http://localhost:8080 http://httpbin.org/ip

# Test SOCKS5 proxy
curl --socks5 localhost:1080 http://httpbin.org/ip

# Open Admin UI
open http://localhost:6000/admin
```

## 🔧 Useful Commands

```bash
# View logs
docker-compose logs -f

# Access container shell
docker-compose exec proxyrouter /bin/sh

# Check container status
docker-compose ps

# Restart services
docker-compose restart

# Stop everything
docker-compose down
```

## 🌐 Service URLs

| Service | URL | Description |
|---------|-----|-------------|
| HTTP Proxy | `http://localhost:8080` | HTTP/HTTPS proxy |
| SOCKS5 Proxy | `socks5://localhost:1080` | SOCKS5 proxy |
| API | `http://localhost:8081` | REST API |
| Admin UI | `http://localhost:6000/admin` | Web interface |
| Tor | `socks5://localhost:9050` | Tor network |

## 🔑 Default Credentials

- **Username**: `admin`
- **Password**: `admin`
- **Note**: You'll be forced to change password on first login

## 🐛 Troubleshooting

### Port Already in Use
```bash
# Check what's using the ports
lsof -i :8080 -i :1080 -i :8081 -i :6000 -i :9050

# Kill conflicting processes
sudo lsof -ti:8080 | xargs kill -9
```

### Permission Issues
```bash
# Fix data directory permissions
sudo chown -R $(whoami):$(id -gn) data/
```

### Docker Issues
```bash
# Restart Docker Desktop
# Check Docker Desktop settings → Resources
# Ensure sufficient CPU/Memory allocation
```

## 📊 Testing Checklist

- [ ] API endpoints respond
- [ ] HTTP proxy works
- [ ] SOCKS5 proxy works
- [ ] Admin UI accessible
- [ ] Tor connectivity (may take time to start)
- [ ] Database migrations completed
- [ ] All services started successfully

## 🧹 Cleanup

```bash
# Stop and remove containers
docker-compose down

# Remove images
docker rmi proxyrouter:latest

# Clean data
rm -rf data/
```
