# Docker Setup for Agentic Mattermost

This document provides instructions for running the Agentic Mattermost application using Docker.

## Prerequisites

- Docker and Docker Compose installed
- At least 4GB of available RAM
- OpenAI API key (optional, for AI features)

## Quick Start

### Production Environment

1. **Set your OpenAI API key (optional):**
   ```bash
   export OPENAI_API_KEY=your_openai_api_key_here
   ```

2. **Start all services:**
   ```bash
   docker-compose up -d
   ```

3. **Check service status:**
   ```bash
   docker-compose ps
   ```

4. **View logs:**
   ```bash
   # All services
   docker-compose logs -f
   
   # Specific service
   docker-compose logs -f app
   ```

### Development Environment

1. **Start development environment:**
   ```bash
   docker-compose -f docker-compose.dev.yml up -d
   ```

2. **Access development services:**
   - Application: http://localhost:8080
   - Temporal Web UI: http://localhost:8088
   - PostgreSQL: localhost:5432

3. **Debug the application:**
   - Connect your IDE to localhost:5005 for remote debugging
   - The application supports hot reloading with volume mounts

## Services

### Application Stack

- **app**: Spring Boot application (port 8080)
- **postgres**: PostgreSQL database (port 5432)
- **temporal**: Temporal workflow server (port 7233)
- **temporal-web**: Temporal Web UI (port 8088)

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENAI_API_KEY` | OpenAI API key for AI features | `dummy` |
| `SPRING_DATASOURCE_URL` | Database connection URL | `jdbc:postgresql://postgres:5432/postgres` |
| `SPRING_DATASOURCE_USERNAME` | Database username | `postgres` |
| `SPRING_DATASOURCE_PASSWORD` | Database password | `mysecretpassword` |
| `TEMPORAL_SERVICE_ADDRESS` | Temporal server address | `temporal:7233` |

## Building Images

### Production Build

```bash
# Build the application image
docker build -t mattermost-app .

# Build and start all services
docker-compose up --build -d
```

### Development Build

```bash
# Build development image
docker build -f Dockerfile.dev -t mattermost-app-dev .

# Build and start development services
docker-compose -f docker-compose.dev.yml up --build -d
```

## Useful Commands

### Container Management

```bash
# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v

# Restart a specific service
docker-compose restart app

# Execute commands in running container
docker-compose exec app bash
docker-compose exec postgres psql -U postgres -d postgres
```

### Logs and Monitoring

```bash
# View real-time logs
docker-compose logs -f app

# View logs for all services
docker-compose logs -f

# Check container health
docker-compose ps
```

### Database Operations

```bash
# Connect to PostgreSQL
docker-compose exec postgres psql -U postgres -d postgres

# Backup database
docker-compose exec postgres pg_dump -U postgres postgres > backup.sql

# Restore database
docker-compose exec -T postgres psql -U postgres -d postgres < backup.sql
```

### Temporal Operations

```bash
# Access Temporal CLI
docker-compose exec temporal tctl

# List workflows
docker-compose exec temporal tctl workflow list

# View Temporal Web UI
# Open http://localhost:8088 in your browser
```

## Troubleshooting

### Common Issues

1. **Port conflicts:**
   - Ensure ports 8080, 5432, 7233, and 8088 are available
   - Modify ports in docker-compose.yml if needed

2. **Memory issues:**
   - Increase Docker memory allocation
   - Ensure sufficient system resources

3. **Database connection issues:**
   - Wait for PostgreSQL to be healthy before starting the app
   - Check database logs: `docker-compose logs postgres`

4. **Temporal connection issues:**
   - Ensure Temporal server is running: `docker-compose logs temporal`
   - Check Temporal Web UI at http://localhost:8088

### Health Checks

```bash
# Check application health
curl http://localhost:8080/actuator/health

# Check database health
docker-compose exec postgres pg_isready -U postgres

# Check Temporal health
curl http://localhost:7233/health
```

### Cleanup

```bash
# Remove all containers, networks, and volumes
docker-compose down -v

# Remove all images
docker rmi $(docker images -q mattermost-*)

# Clean up Docker system
docker system prune -a
```

## Security Notes

- Change default passwords in production
- Use environment variables for sensitive data
- Consider using Docker secrets for production deployments
- Regularly update base images for security patches

## Performance Tuning

- Adjust JVM heap size in Dockerfile if needed
- Configure database connection pool settings
- Monitor resource usage with `docker stats`
- Consider using Docker volumes for persistent data 

## OpenSearch & Fluent Bit Integration

### Added Services
- **opensearch**: Search and analytics engine (port 9200)
- **opensearch-dashboards**: Web UI for OpenSearch (port 5601)
- **fluent-bit**: Collects all Docker container logs and pushes them to OpenSearch

### How It Works
- All logs from containers (including the app) are tailed by Fluent Bit and sent to OpenSearch.
- You can view/search logs in OpenSearch Dashboards at http://localhost:5601 (default login: no auth, security disabled for dev).

### Usage
1. Start all services:
   ```bash
   docker-compose up -d
   ```
2. Access OpenSearch Dashboards:
   - http://localhost:5601
   - Search index: `docker-logs-*`
3. OpenSearch API:
   - http://localhost:9200

### Fluent Bit Config
- Config file: `fluent-bit.conf` in project root
- Input: Docker container logs
- Output: OpenSearch (index: `docker-logs`)

### Ports
| Service                | Port |
|------------------------|------|
| OpenSearch             | 9200 |
| OpenSearch Dashboards  | 5601 |
| Fluent Bit (internal)  | 24224| 