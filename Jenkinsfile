pipeline {
    agent any

    environment {
        DEPLOY_HOST = 'pinky'
        DEPLOY_USER = 'grimlock'  // or your deployment user
        DEPLOY_DIR = '/opt/crypto-thing'
        CONFIG_DIR = '/etc/crypto-thing'
        BINARY_NAME = 'cryptool'
        GO_VERSION = '1.22'
        SSH_KEY_ID = 'Jenkins-private-key'  // Jenkins credential ID for SSH key
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Setup Go Environment') {
            steps {
                sh """
                    # Install Go if not present
                    if ! command -v go &> /dev/null; then
                        echo "Installing Go ${GO_VERSION}..."
                        wget -q https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz
                        sudo rm -rf /usr/local/go
                        sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
                        echo 'export PATH=\$PATH:/usr/local/go/bin' >> ~/.bashrc
                        export PATH=\$PATH:/usr/local/go/bin
                    fi

                    # Verify Go installation
                    go version
                """
            }
        }

        stage('Build Application') {
            steps {
                sh """
                    echo "Building crypto tool..."
                    cd ${WORKSPACE}
                    go mod tidy
                    go build -o ${BINARY_NAME} .
                """
            }
        }

        stage('Deploy to Pinky') {
            steps {
                sshagent(credentials: ["${SSH_KEY_ID}"]) {
                    sh """
                        echo "Deploying to ${DEPLOY_HOST}..."

                        # Test SSH connection
                        ssh -o StrictHostKeyChecking=no -l ${DEPLOY_USER} ${DEPLOY_HOST} 'echo "SSH connection successful"'

                        # Create deployment directories on remote host
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo mkdir -p ${DEPLOY_DIR} ${CONFIG_DIR}"

                        # Set ownership for deployment directory (allow jenkins user to write)
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo chown -R ${DEPLOY_USER}:${DEPLOY_USER} ${DEPLOY_DIR}"

                        # Copy binary to remote host
                        BINARY_DEST="${DEPLOY_USER}@${DEPLOY_HOST}:${DEPLOY_DIR}/"
                        scp ${BINARY_NAME} "${BINARY_DEST}"
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "chmod +x ${DEPLOY_DIR}/${BINARY_NAME}"

                        # Copy migrations if they exist
                        if [ -d "${WORKSPACE}/migrations" ]; then
                            echo "Copying database migrations..."
                            MIGRATION_DEST="${DEPLOY_USER}@${DEPLOY_HOST}:${DEPLOY_DIR}/"
                            scp -r migrations "${MIGRATION_DEST}"
                        else
                            echo "Warning: Migrations directory not found in workspace"
                        fi

                        # Copy service management files if they exist
                        if [ -d "${WORKSPACE}/devops/systemd" ]; then
                            echo "Copying service management files..."
                            # Construct the remote path and use it directly
                            REMOTE_DEST="${DEPLOY_USER}@${DEPLOY_HOST}:${DEPLOY_DIR}/"
                            scp -r devops/systemd "${REMOTE_DEST}"
                            if [ $? -eq 0 ]; then
                                echo "Setting executable permissions on daemon manager..."
                                ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "chmod +x ${DEPLOY_DIR}/devops/systemd/daemon-manager.sh"
                                ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "chmod 644 ${DEPLOY_DIR}/devops/systemd/crypto-thing.service"
                                echo "Service files deployed successfully"
                            else
                                echo "Warning: Failed to copy service management files"
                            fi
                        else
                            echo "Warning: Service management directory not found in workspace"
                        fi
                    """
                }
            }
        }

        stage('Configure Remote Environment') {
            steps {
                sshagent(credentials: ["${SSH_KEY_ID}"]) {
                    sh """
                        # Create .env file on remote host
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "cat > ${DEPLOY_DIR}/.env << EOF"
# Crypto Tool Configuration
CRYPTO_CONFIG_FILE=${CONFIG_DIR}/crypto.ini
DAEMON_PORT=40000
EOF"

                        # Set proper permissions for .env file
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "chmod 644 ${DEPLOY_DIR}/.env"
                    """
                }
            }
        }

        stage('Setup Remote Service') {
            steps {
                sshagent(credentials: ["${SSH_KEY_ID}"]) {
                    sh """
                        # Create systemd service file on remote host
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo tee /etc/systemd/system/crypto-thing.service > /dev/null" << EOF
[Unit]
Description=Crypto Thing Tool
After=network.target
Wants=network.target

[Service]
Type=simple
User=${DEPLOY_USER}
Group=${DEPLOY_USER}
WorkingDirectory=${DEPLOY_DIR}
EnvironmentFile=${DEPLOY_DIR}/.env

# Start the daemon with websocket server
ExecStart=${DEPLOY_DIR}/${BINARY_NAME} daemon --port 40000

# Restart policy
Restart=always
RestartSec=10

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DEPLOY_DIR} ${CONFIG_DIR}

# Resource limits
LimitNOFILE=65536
MemoryLimit=512M

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=crypto-thing

[Install]
WantedBy=multi-user.target
EOF

                        # Reload systemd and set permissions
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo systemctl daemon-reload"
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo chown -R ${DEPLOY_USER}:${DEPLOY_USER} ${DEPLOY_DIR}"
                    """
                }
            }
        }

        stage('Verify Remote Deployment') {
            steps {
                sshagent(credentials: ["${SSH_KEY_ID}"]) {
                    sh """
                        echo "Verifying deployment on ${DEPLOY_HOST}..."

                        # Check files exist and have correct permissions
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "ls -la ${DEPLOY_DIR}/"

                        # Verify service files were copied
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "test -f ${DEPLOY_DIR}/devops/systemd/daemon-manager.sh && echo 'Service manager found' || echo 'Warning: Service manager not found'"
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "test -f /etc/systemd/system/crypto-thing.service && echo 'Systemd service found' || echo 'Warning: Systemd service not found'"

                        # Test binary runs (basic smoke test)
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "${DEPLOY_DIR}/${BINARY_NAME} --help"
                    """
                }
            }
        }
    }

    post {
        success {
            echo 'Deployment to Pinky completed successfully!'
            sshagent(credentials: ["${SSH_KEY_ID}"]) {
                sh """
                    echo "Crypto tool deployed to: ${DEPLOY_HOST}:${DEPLOY_DIR}"
                    echo "Configuration file: ${CONFIG_DIR}/crypto.ini"
                    echo "Environment file: ${DEPLOY_DIR}/.env"
                    echo ""
                    echo "Remote commands to manage the service:"
                    echo "ssh ${DEPLOY_USER}@${DEPLOY_HOST} 'sudo systemctl enable crypto-thing'"
                    echo "ssh ${DEPLOY_USER}@${DEPLOY_HOST} 'sudo systemctl start crypto-thing'"
                    echo "ssh ${DEPLOY_USER}@${DEPLOY_HOST} 'sudo systemctl status crypto-thing'"
                    echo "ssh ${DEPLOY_USER}@${DEPLOY_HOST} 'sudo journalctl -u crypto-thing -f'"
                    echo "ssh ${DEPLOY_USER}@${DEPLOY_HOST} '${DEPLOY_DIR}/devops/systemd/daemon-manager.sh status'"
                """
            }
        }

        failure {
            echo 'Deployment to Pinky failed!'
            sshagent(credentials: ["${SSH_KEY_ID}"]) {
                sh """
                    echo "Cleaning up failed deployment on ${DEPLOY_HOST}..."
                    ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo rm -rf ${DEPLOY_DIR}"
                    ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo rm -f /etc/systemd/system/crypto-thing.service"
                    ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo systemctl daemon-reload"
                """
            }
        }

        cleanup {
            sh """
                echo "Cleaning up local build artifacts..."
                rm -f ${BINARY_NAME}
            """
        }
    }
}
