#!/bin/bash

# Crypto Thing Daemon Service Management Script
# Provides easy commands for managing the crypto daemon service

SERVICE_NAME="crypto-thing"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Install service
install_service() {
    check_root

    print_status "Installing crypto daemon service..."

    # Copy service file
    if [[ -f "${SERVICE_NAME}.service" ]]; then
        cp "${SERVICE_NAME}.service" "$SERVICE_FILE"
        print_success "Service file installed to $SERVICE_FILE"
    else
        print_error "Service file ${SERVICE_NAME}.service not found in current directory"
        exit 1
    fi

    # Reload systemd
    systemctl daemon-reload
    print_success "Systemd daemon reloaded"

    # Enable service
    systemctl enable "$SERVICE_NAME"
    print_success "Service enabled to start on boot"
}

# Start service
start_service() {
    check_root

    print_status "Starting crypto daemon service..."
    systemctl start "$SERVICE_NAME"

    if [[ $? -eq 0 ]]; then
        print_success "Service started successfully"

        # Wait a moment and check status
        sleep 2
        systemctl status "$SERVICE_NAME" --no-pager -l
    else
        print_error "Failed to start service"
        exit 1
    fi
}

# Stop service
stop_service() {
    check_root

    print_status "Stopping crypto daemon service..."
    systemctl stop "$SERVICE_NAME"

    if [[ $? -eq 0 ]]; then
        print_success "Service stopped successfully"
    else
        print_error "Failed to stop service"
        exit 1
    fi
}

# Restart service
restart_service() {
    check_root

    print_status "Restarting crypto daemon service..."
    systemctl restart "$SERVICE_NAME"

    if [[ $? -eq 0 ]]; then
        print_success "Service restarted successfully"
    else
        print_error "Failed to restart service"
        exit 1
    fi
}

# Check service status
status_service() {
    print_status "Checking crypto daemon service status..."
    systemctl status "$SERVICE_NAME" --no-pager -l
}

# View service logs
view_logs() {
    print_status "Viewing crypto daemon service logs..."
    journalctl -u "$SERVICE_NAME" -f
}

# Check service health
check_health() {
    print_status "Checking daemon health endpoint..."

    # Try to connect to health endpoint
    if curl -s http://localhost:40000/health > /dev/null 2>&1; then
        print_success "Daemon is healthy and responding"

        # Get full health status
        echo
        curl -s http://localhost:40000/health | jq . 2>/dev/null || curl -s http://localhost:40000/health
    else
        print_error "Daemon is not responding on port 40000"
        print_status "Make sure the service is running: sudo systemctl start $SERVICE_NAME"
    fi
}

# Show usage
show_usage() {
    echo "Crypto Thing Daemon Service Manager"
    echo
    echo "Usage: $0 [command]"
    echo
    echo "Commands:"
    echo "  install     Install and enable the systemd service"
    echo "  start       Start the daemon service"
    echo "  stop        Stop the daemon service"
    echo "  restart     Restart the daemon service"
    echo "  status      Show service status"
    echo "  logs        View service logs (follow mode)"
    echo "  health      Check daemon health endpoint"
    echo "  help        Show this help message"
    echo
    echo "Examples:"
    echo "  sudo $0 install     # Install service"
    echo "  sudo $0 start       # Start daemon"
    echo "  $0 health           # Check if daemon is responding"
    echo "  $0 logs             # View logs"
}

# Main logic
case "${1:-help}" in
    install)
        install_service
        ;;
    start)
        start_service
        ;;
    stop)
        stop_service
        ;;
    restart)
        restart_service
        ;;
    status)
        status_service
        ;;
    logs)
        view_logs
        ;;
    health)
        check_health
        ;;
    help|--help|-h)
        show_usage
        ;;
    *)
        print_error "Unknown command: $1"
        echo
        show_usage
        exit 1
        ;;
esac
