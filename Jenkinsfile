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
                    cd ${env.WORKSPACE}
                    go mod tidy
                    go build -o ${env.BINARY_NAME} .
                """
            }
        }

        stage('Deploy to Pinky') {
            steps {
                sshagent(credentials: ["${SSH_KEY_ID}"]) {
                    sh """
                        echo "Deploying to ${env.DEPLOY_HOST}..."

                        # Test SSH connection
                        ssh -o StrictHostKeyChecking=no -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} 'echo "SSH connection successful"'

                        # Create deployment directories on remote host
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo mkdir -p ${env.DEPLOY_DIR} ${env.CONFIG_DIR}"

                        # Set ownership for deployment directory (allow jenkins user to write)
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo chown -R ${env.DEPLOY_USER}:${env.DEPLOY_USER} ${env.DEPLOY_DIR}"

                        # Copy binary to remote host via /tmp, then move with sudo to destination (more robust perms)
                        echo "Deploying binary to: ${env.DEPLOY_USER}@${env.DEPLOY_HOST}:${env.DEPLOY_DIR}/"
                        scp ${env.BINARY_NAME} ${env.DEPLOY_USER}@${env.DEPLOY_HOST}:/tmp/${env.BINARY_NAME}
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo mv /tmp/${env.BINARY_NAME} ${env.DEPLOY_DIR}/${env.BINARY_NAME} && sudo chown ${env.DEPLOY_USER}:${env.DEPLOY_USER} ${env.DEPLOY_DIR}/${env.BINARY_NAME} && sudo chmod 0755 ${env.DEPLOY_DIR}/${env.BINARY_NAME}"

                        # Copy migrations if they exist
                        if [ -d "${env.WORKSPACE}/migrations" ]; then
                            echo "Copying database migrations..."
                            scp -r migrations ${env.DEPLOY_USER}@${env.DEPLOY_HOST}:${env.DEPLOY_DIR}/
                        else
                            echo "Warning: Migrations directory not found in workspace"
                        fi

                        # Copy service management files if they exist
                        if [ -d "${env.WORKSPACE}/devops/systemd" ]; then
                            echo "Copying service management files..."
                            scp -r devops ${env.DEPLOY_USER}@${env.DEPLOY_HOST}:${env.DEPLOY_DIR}/
                            if [ \$? -eq 0 ]; then
                                echo "Setting executable permissions on daemon manager..."
                                ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "chmod +x ${env.DEPLOY_DIR}/devops/systemd/daemon-manager.sh"
                                ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "chmod 644 ${env.DEPLOY_DIR}/devops/systemd/crypto-thing.service"
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
                        # Copy .env from devops to remote and install
                        echo "Installing environment file..."
                        scp devops/.env.deploy ${env.DEPLOY_USER}@${env.DEPLOY_HOST}:/tmp/.env
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo mv /tmp/.env ${env.DEPLOY_DIR}/.env && sudo chown ${env.DEPLOY_USER}:${env.DEPLOY_USER} ${env.DEPLOY_DIR}/.env && sudo chmod 0644 ${env.DEPLOY_DIR}/.env"

                        # Copy crypto.ini from devops to remote config directory
                        echo "Installing configuration file..."
                        scp devops/crypto.ini.deploy ${env.DEPLOY_USER}@${env.DEPLOY_HOST}:/tmp/crypto.ini
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo mv /tmp/crypto.ini ${env.CONFIG_DIR}/crypto.ini && sudo chown root:root ${env.CONFIG_DIR}/crypto.ini && sudo chmod 0644 ${env.CONFIG_DIR}/crypto.ini"
                    """
                }
            }
        }

        stage('Setup Remote Service') {
            steps {
                sshagent(credentials: ["${SSH_KEY_ID}"]) {
                    sh """
                        # Create systemd service file on remote host
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo tee /etc/systemd/system/crypto-thing.service > /dev/null" << EOF
[Unit]
Description=Crypto Thing Tool
After=network.target
Wants=network.target

[Service]
Type=simple
User=${env.DEPLOY_USER}
Group=${env.DEPLOY_USER}
WorkingDirectory=${env.DEPLOY_DIR}
EnvironmentFile=${env.DEPLOY_DIR}/.env

# Start the daemon with websocket server
ExecStart=${env.DEPLOY_DIR}/${env.BINARY_NAME} daemon 40000

# Restart policy
Restart=always
RestartSec=10

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${env.DEPLOY_DIR} ${env.CONFIG_DIR}

# Resource limits
LimitNOFILE=65536
MemoryMax=512M

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=crypto-thing

[Install]
WantedBy=multi-user.target
EOF

                        # Reload systemd, set permissions, enable and restart service
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo systemctl daemon-reload"
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo chown -R ${env.DEPLOY_USER}:${env.DEPLOY_USER} ${env.DEPLOY_DIR}"
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo systemctl enable crypto-thing"
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo systemctl restart crypto-thing"
                    """
                }
            }
        }

        stage('Verify Remote Deployment') {
            steps {
                sshagent(credentials: ["${SSH_KEY_ID}"]) {
                    sh """
                        echo "Verifying deployment on ${env.DEPLOY_HOST}..."

                        # Check files exist and have correct permissions
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "ls -la ${env.DEPLOY_DIR}/"

                        # Verify service files were copied
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "test -f ${env.DEPLOY_DIR}/devops/systemd/daemon-manager.sh && echo 'Service manager found' || echo 'Warning: Service manager not found'"
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "test -f /etc/systemd/system/crypto-thing.service && echo 'Systemd service found' || echo 'Warning: Systemd service not found'"

                        # Test binary runs (basic smoke test)
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "${env.DEPLOY_DIR}/${env.BINARY_NAME} --help"

                        # Verify service is enabled
                        echo "Checking service enablement..."
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo systemctl is-enabled crypto-thing"

                        # Verify service is active; if not, show status and logs then fail
                        echo "Checking service active state..."
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo systemctl is-active --quiet crypto-thing || { sudo systemctl status --no-pager crypto-thing; sudo journalctl -u crypto-thing -n 200 --no-pager; exit 1; }"

                        # Wait for health endpoint with retries
                        echo "Waiting for health endpoint..."
                        ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} 'ok=0; for i in \$(seq 1 15); do if curl -fsS http://localhost:40000/health >/dev/null; then ok=1; echo "Health endpoint OK"; break; else echo "Health not ready yet (\$i/15)"; sleep 2; fi; done; if [ "\$ok" -ne 1 ]; then echo "Health check failed"; sudo systemctl status --no-pager crypto-thing; sudo journalctl -u crypto-thing -n 200 --no-pager; exit 1; fi'
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
                    echo "Crypto tool deployed to: ${env.DEPLOY_HOST}:${env.DEPLOY_DIR}"
                    echo "Configuration file: ${env.CONFIG_DIR}/crypto.ini"
                    echo "Environment file: ${env.DEPLOY_DIR}/.env"
                    echo ""
                    echo "Remote commands to manage the service:"
                    echo "ssh ${env.DEPLOY_USER}@${env.DEPLOY_HOST} 'sudo systemctl enable crypto-thing'"
                    echo "ssh ${env.DEPLOY_USER}@${env.DEPLOY_HOST} 'sudo systemctl start crypto-thing'"
                    echo "ssh ${env.DEPLOY_USER}@${env.DEPLOY_HOST} 'sudo systemctl status crypto-thing'"
                    echo "ssh ${env.DEPLOY_USER}@${env.DEPLOY_HOST} 'sudo journalctl -u crypto-thing -f'"
                    echo "ssh ${env.DEPLOY_USER}@${env.DEPLOY_HOST} '${env.DEPLOY_DIR}/devops/systemd/daemon-manager.sh status'"
                """
            }
        }

        failure {
            echo 'Deployment to Pinky failed!'
            sshagent(credentials: ["${SSH_KEY_ID}"]) {
                sh """
                    echo "Cleaning up failed deployment on ${DEPLOY_HOST}..."
                    ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo rm -rf ${env.DEPLOY_DIR}"
                    ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo rm -f /etc/systemd/system/crypto-thing.service"
                    ssh -l ${env.DEPLOY_USER} ${env.DEPLOY_HOST} "sudo systemctl daemon-reload"
                """
            }
        }

        cleanup {
            sh """
                echo "Cleaning up local build artifacts..."
                rm -f ${env.BINARY_NAME}
            """
        }
    }
}
