#!/bin/bash

# Crypto Thing Deployment Script
# This script deploys the crypto tool to /opt/crypto-thing/

set -e

# Configuration
DEPLOY_DIR="/opt/crypto-thing"
CONFIG_DIR="/etc/crypto-thing"
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🚀 Starting Crypto Thing Deployment..."

# Check if we're running as root for system directories
if [[ $EUID -eq 0 ]]; then
   echo "❌ Don't run this script as root. It will use sudo when needed."
   exit 1
fi

# Build the application
echo "📦 Building application..."
cd "$PROJECT_DIR"
go mod tidy
go build -o cryptool .

# Create deployment directories
echo "📁 Creating deployment directories..."
sudo mkdir -p "$DEPLOY_DIR"
sudo mkdir -p "$CONFIG_DIR"

# Copy application files
echo "📋 Copying application files..."
cp cryptool "$DEPLOY_DIR/"
chmod +x "$DEPLOY_DIR/cryptool"

# Copy migrations if they exist
if [ -d "migrations" ]; then
    cp -r migrations "$DEPLOY_DIR/"
fi

# Copy configuration files if they exist
if [ -f "crypto.ini.deploy" ]; then
    echo "⚙️  Copying deployment configuration..."
    sudo cp crypto.ini.deploy "$CONFIG_DIR/crypto.ini"
    sudo chmod 644 "$CONFIG_DIR/crypto.ini"
fi

# Create .env file
echo "🌍 Creating environment configuration..."
cat > "$DEPLOY_DIR/.env" << EOF
# Crypto Tool Configuration
CRYPTO_CONFIG_FILE=$CONFIG_DIR/crypto.ini
EOF

# Set permissions
echo "🔐 Setting permissions..."
sudo chown -R $(whoami):$(whoami) "$DEPLOY_DIR"
sudo chmod 755 "$DEPLOY_DIR/cryptool"
sudo chmod 644 "$DEPLOY_DIR/.env"

# Create systemd service if it doesn't exist
if [ ! -f "/etc/systemd/system/crypto-thing.service" ]; then
    echo "⚙️  Creating systemd service..."
    sudo tee /etc/systemd/system/crypto-thing.service > /dev/null << EOF
[Unit]
Description=Crypto Thing Tool
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=$(whoami)
Group=$(whoami)
WorkingDirectory=$DEPLOY_DIR
EnvironmentFile=$DEPLOY_DIR/.env
ExecStart=$DEPLOY_DIR/cryptool
Restart=always
RestartSec=10

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DEPLOY_DIR $CONFIG_DIR

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    echo "✅ Systemd service created"
else
    echo "ℹ️  Systemd service already exists"
fi

# Verify deployment
echo "✅ Verifying deployment..."
ls -la "$DEPLOY_DIR/"
echo ""
echo "🎉 Deployment completed successfully!"
echo ""
echo "📍 Installation locations:"
echo "   Binary: $DEPLOY_DIR/cryptool"
echo "   Config: $CONFIG_DIR/crypto.ini"
echo "   Environment: $DEPLOY_DIR/.env"
echo ""
echo "🔧 Next steps:"
echo "   1. Edit $CONFIG_DIR/crypto.ini with your actual credentials"
echo "   2. Enable and start the service:"
echo "      sudo systemctl enable crypto-thing"
echo "      sudo systemctl start crypto-thing"
echo ""
echo "📊 Check service status:"
echo "   sudo systemctl status crypto-thing"
echo ""
echo "📋 View logs:"
echo "   sudo journalctl -u crypto-thing -f"
