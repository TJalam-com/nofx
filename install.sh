#!/bin/bash
# NOFX AI Trading System - One-Click Installation Script
# Automatically generates encryption keys and sets up the environment

set -e

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

print_info() {
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

# Check dependencies
check_dependencies() {
    print_info "Checking dependencies..."
    
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker first."
        print_info "Visit: https://docs.docker.com/get-docker/"
        exit 1
    fi
    
    if ! command -v openssl &> /dev/null; then
        print_error "OpenSSL is not installed. Please install OpenSSL first."
        print_info "macOS: brew install openssl"
        print_info "Ubuntu/Debian: sudo apt-get install openssl"
        exit 1
    fi
    
    print_success "All dependencies are installed"
}

# Generate encryption keys
generate_keys() {
    print_info "Generating encryption keys..."
    
    # Create .env file if it doesn't exist
    if [ ! -f ".env" ]; then
        print_info "Creating .env file..."
        touch .env
        chmod 600 .env
    fi
    
    # Generate DATA_ENCRYPTION_KEY if not exists
    if ! grep -q "^DATA_ENCRYPTION_KEY=" .env 2>/dev/null; then
        DATA_KEY=$(openssl rand -base64 32)
        echo "DATA_ENCRYPTION_KEY=$DATA_KEY" >> .env
        print_success "Generated DATA_ENCRYPTION_KEY"
    else
        print_info "DATA_ENCRYPTION_KEY already exists, skipping..."
    fi
    
    # Generate JWT_SECRET if not exists
    if ! grep -q "^JWT_SECRET=" .env 2>/dev/null; then
        JWT_KEY=$(openssl rand -base64 64)
        echo "JWT_SECRET=\"$JWT_KEY\"" >> .env
        print_success "Generated JWT_SECRET"
    else
        print_info "JWT_SECRET already exists, skipping..."
    fi
    
    # Set default ports if not exists
    if ! grep -q "^NOFX_BACKEND_PORT=" .env 2>/dev/null; then
        echo "NOFX_BACKEND_PORT=8080" >> .env
        print_info "Set default NOFX_BACKEND_PORT=8080"
    fi
    
    if ! grep -q "^NOFX_FRONTEND_PORT=" .env 2>/dev/null; then
        echo "NOFX_FRONTEND_PORT=3000" >> .env
        print_info "Set default NOFX_FRONTEND_PORT=3000"
    fi
    
    if ! grep -q "^NOFX_TIMEZONE=" .env 2>/dev/null; then
        echo "NOFX_TIMEZONE=Asia/Shanghai" >> .env
        print_info "Set default NOFX_TIMEZONE=Asia/Shanghai"
    fi
    
    if ! grep -q "^AI_MAX_TOKENS=" .env 2>/dev/null; then
        echo "AI_MAX_TOKENS=4000" >> .env
        print_info "Set default AI_MAX_TOKENS=4000"
    fi
    
    if ! grep -q "^TRANSPORT_ENCRYPTION=" .env 2>/dev/null; then
        echo "TRANSPORT_ENCRYPTION=false" >> .env
        print_info "Set default TRANSPORT_ENCRYPTION=false (Simple Mode)"
    fi
}

# Setup RSA keys using existing script
setup_rsa_keys() {
    if [ -f "scripts/setup_encryption.sh" ]; then
        print_info "Setting up RSA keys..."
        # Auto-answer prompts: Y for continue, n for regenerate, n for skip
        if [ -f "secrets/rsa_key" ] && [ -f "secrets/rsa_key.pub" ]; then
            print_info "RSA keys already exist, skipping generation..."
        else
            echo -e "Y\nn\nn" | bash scripts/setup_encryption.sh > /dev/null 2>&1 || {
                print_warning "RSA key generation had issues, but continuing..."
            }
            print_success "RSA keys setup completed"
        fi
    else
        print_warning "setup_encryption.sh not found, skipping RSA key generation"
    fi
}

# Create necessary directories
create_directories() {
    print_info "Creating necessary directories..."
    
    mkdir -p data/secrets
    chmod 700 data/secrets
    
    mkdir -p data/decision_logs
    
    mkdir -p prompts
    
    # Create data directory if it doesn't exist
    mkdir -p data
    
    if [ ! -f "data/data.db" ]; then
        install -m 600 /dev/null data/data.db 2>/dev/null || touch data/data.db && chmod 600 data/data.db
    fi
    
    print_success "Directories and files created"
}

# Copy config files if needed
setup_config() {
    if [ ! -f "config.json" ]; then
        if [ -f "config.json.example" ]; then
            print_info "Copying config.json.example to config.json..."
            cp config.json.example config.json
            print_success "Config file created"
        else
            print_warning "config.json.example not found, you may need to create config.json manually"
        fi
    else
        print_info "config.json already exists, skipping..."
    fi
}

# Get server IP for display
get_server_ip() {
    # Try to get public IP first
    local public_ip=$(curl -s --max-time 3 ifconfig.me 2>/dev/null || curl -s --max-time 3 icanhazip.com 2>/dev/null || echo "")
    
    # If no public IP, try local IP
    if [ -z "$public_ip" ]; then
        if command -v ip &> /dev/null; then
            public_ip=$(ip route get 1 2>/dev/null | awk '{print $7}' | head -1)
        elif command -v hostname &> /dev/null; then
            public_ip=$(hostname -I 2>/dev/null | awk '{print $1}')
        fi
    fi
    
    echo "${public_ip:-127.0.0.1}"
}

# Main installation
main() {
    echo ""
    echo "=========================================="
    echo "  NOFX AI Trading System Installation"
    echo "=========================================="
    echo ""
    
    check_dependencies
    create_directories
    generate_keys
    setup_rsa_keys
    setup_config
    
    echo ""
    print_success "Installation completed!"
    echo ""
    
    local SERVER_IP=$(get_server_ip)
    
    print_info "Next steps:"
    echo "  1. Edit config.json if needed (optional)"
    echo "  2. Edit .env if you want to change ports or timezone"
    echo "  3. Run: ./start.sh start --build"
    echo "  4. Access web interface at http://${SERVER_IP}:3000"
    echo ""
    if [ "$SERVER_IP" != "127.0.0.1" ]; then
        echo "  Note: If accessing from local machine, use http://127.0.0.1:3000"
        echo ""
    fi
    print_info "For production deployment, use:"
    echo "  docker compose -f docker-compose.prod.yml up -d --build"
    echo ""
}

main "$@"


