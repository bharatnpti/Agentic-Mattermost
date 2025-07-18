#!/bin/bash

# Docker run script for Maestro
# Usage: ./docker-run.sh [command] [environment]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
ENVIRONMENT="prod"
COMPOSE_FILE="docker-compose.yml"

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}  Maestro Docker${NC}"
    echo -e "${BLUE}================================${NC}"
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [command] [environment]"
    echo ""
    echo "Commands:"
    echo "  start     - Start the application"
    echo "  stop      - Stop the application"
    echo "  restart   - Restart the application"
    echo "  build     - Build the application"
    echo "  logs      - Show application logs"
    echo "  status    - Show container status"
    echo "  clean     - Clean up containers and volumes"
    echo "  shell     - Open shell in app container"
    echo "  db        - Connect to database"
    echo "  temporal  - Open Temporal Web UI"
    echo "  help      - Show this help message"
    echo ""
    echo "Environments:"
    echo "  prod      - Production environment (default)"
    echo "  dev       - Development environment"
    echo ""
    echo "Examples:"
    echo "  $0 start prod"
    echo "  $0 start dev"
    echo "  $0 logs"
    echo "  $0 clean"
}

# Function to check if Docker is running
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker and try again."
        exit 1
    fi
}

# Function to set environment
set_environment() {
    if [ "$1" = "dev" ]; then
        ENVIRONMENT="dev"
        COMPOSE_FILE="docker-compose.dev.yml"
        print_status "Using development environment"
    else
        ENVIRONMENT="prod"
        COMPOSE_FILE="docker-compose.yml"
        print_status "Using production environment"
    fi
}

###############################################################################
# Wait until every container is healthy / running, or give up after TIMEOUT   #
###############################################################################
wait_for_services_ready() {
    local compose_file="$1"   # first arg  : compose file to inspect
    local timeout="${2:-120}" # second arg : max seconds to wait (default 120)
    local interval="${3:-5}"  # third arg  : polling interval   (default 5)

    print_status "Checking container readiness (timeout ${timeout}s)…"
    local start_ts=$(date +%s)

    while : ; do
        local all_ready=true
        echo -e "${BLUE}-----------------------------------------------${NC}"
        echo -e "${BLUE} Service               Status        Ports${NC}"
        echo -e "${BLUE}-----------------------------------------------${NC}"

        docker-compose -f "$compose_file" ps --services | while read -r svc; do
            local cid=$(docker-compose -f "$compose_file" ps -q "$svc")
            local health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$cid")
            local ports=$(docker inspect -f '{{range $p, $conf := .NetworkSettings.Ports}}{{$p}}→{{(index $conf 0).HostPort}} {{end}}' "$cid")

            # Colour‑code line according to health
            local colour=$GREEN
            if [[ "$health" == "starting" || "$health" == "running" ]]; then
                colour=$YELLOW
                all_ready=false
            elif [[ "$health" != "healthy" ]]; then
                colour=$RED
                all_ready=false
            fi

            printf "%-20s %b%-10s%b  %s\n" "$svc" "$colour" "$health" "$NC" "$ports"
        done
        echo -e "${BLUE}-----------------------------------------------${NC}"

        $all_ready && { print_status "All services are ready!"; return 0; }

        # Abort if timeout reached
        if (( $(date +%s) - start_ts >= timeout )); then
            print_error "Timeout reached – some services never became ready."
            return 1
        fi
        sleep "$interval"
    done
}


# Function to start services
start_services() {
    print_status "Starting services..."
    
    # Check if OpenAI API key is set
    if [ -z "$OPENAI_API_KEY" ]; then
        print_warning "OPENAI_API_KEY not set. AI features will use dummy key."
    fi
    
    docker-compose -f $COMPOSE_FILE up -d

    wait_for_services_ready "$COMPOSE_FILE" 120 5 || exit 1   # <‑‑ NEW CALL

    
    print_status "Services started successfully!"
    print_status "Temporal Web UI: http://localhost:8088"
    print_status "Mattermost Web UI: http://localhost:8065"
    print_status "PostgreSQL: localhost:5432"
    print_status "Infra Services Started, follow below steps to run agent"
    print_status "export MATTERMOST_HOST={Value}"
    print_status "export MATTERMOST_TOKEN={Value}"
    print_status "docker-compose -p maestro -f docker-compose.app.yml up -d"
    
    if [ "$ENVIRONMENT" = "dev" ]; then
        print_status "Debug port: localhost:5005"
    fi
}

# Function to stop services
stop_services() {
    print_status "Stopping services..."
    docker-compose -f $COMPOSE_FILE down
    print_status "Services stopped successfully!"
}

# Function to restart services
restart_services() {
    print_status "Restarting services..."
    docker-compose -f $COMPOSE_FILE restart
    print_status "Services restarted successfully!"
}

# Function to build services
build_services() {
    print_status "Building services..."
    docker-compose -f $COMPOSE_FILE up --build -d
    print_status "Services built and started successfully!"
}

# Function to show logs
show_logs() {
    print_status "Showing logs (Ctrl+C to exit)..."
    docker-compose -f $COMPOSE_FILE logs -f
}

# Function to show status
show_status() {
    print_status "Container status:"
    docker-compose -f $COMPOSE_FILE ps
}

# Function to clean up
clean_up() {
    print_warning "This will remove all containers, networks, and volumes!"
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        print_status "Cleaning up..."
        docker-compose -f $COMPOSE_FILE down -v
        docker system prune -f
        print_status "Cleanup completed!"
    else
        print_status "Cleanup cancelled."
    fi
}

# Function to open shell
open_shell() {
    print_status "Opening shell in app container..."
    docker-compose -f $COMPOSE_FILE exec app bash
}

# Function to connect to database
connect_db() {
    print_status "Connecting to PostgreSQL..."
    docker-compose -f $COMPOSE_FILE exec postgres psql -U postgres -d postgres
}

# Function to open Temporal Web UI
open_temporal() {
    print_status "Opening Temporal Web UI..."
    if command -v open > /dev/null 2>&1; then
        open http://localhost:8088
    elif command -v xdg-open > /dev/null 2>&1; then
        xdg-open http://localhost:8088
    else
        print_status "Please open http://localhost:8088 in your browser"
    fi
}

# Main script logic
main() {
    print_header
    
    # Check if Docker is running
    check_docker
    
    # Parse command line arguments
    COMMAND=${1:-help}
    ENV_ARG=${2:-prod}
    
    # Set environment
    set_environment $ENV_ARG
    export COMPOSE_PROJECT_NAME="maestro"
    
    # Execute command
    case $COMMAND in
        start)
            start_services
            ;;
        stop)
            stop_services
            ;;
        restart)
            restart_services
            ;;
        build)
            build_services
            ;;
        logs)
            show_logs
            ;;
        status)
            show_status
            ;;
        clean)
            clean_up
            ;;
        shell)
            open_shell
            ;;
        db)
            connect_db
            ;;
        temporal)
            open_temporal
            ;;
        help|--help|-h)
            show_usage
            ;;
        *)
            print_error "Unknown command: $COMMAND"
            show_usage
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@" 