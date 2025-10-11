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
                        scp ${BINARY_NAME} ${DEPLOY_USER}@${DEPLOY_HOST}:${DEPLOY_DIR}/
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "chmod +x ${DEPLOY_DIR}/${BINARY_NAME}"

                        # Copy migrations if they exist
                        if [ -d "${WORKSPACE}/migrations" ]; then
                            scp -r migrations ${DEPLOY_USER}@${DEPLOY_HOST}:${DEPLOY_DIR}/
                        fi

                        # Copy configuration files if they exist
                        if [ -f "${WORKSPACE}/crypto.ini.deploy" ]; then
                            # Check if crypto.ini already exists on target host
                            if ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "test -f ${CONFIG_DIR}/crypto.ini"; then
                                echo "Configuration file ${CONFIG_DIR}/crypto.ini already exists on target host. Skipping deployment configuration."
                            else
                                echo "Deploying configuration file to ${CONFIG_DIR}/crypto.ini"
                                scp crypto.ini.deploy ${DEPLOY_USER}@${DEPLOY_HOST}:/tmp/crypto.ini
                                ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo mv /tmp/crypto.ini ${CONFIG_DIR}/crypto.ini"
                                ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo chmod 644 ${CONFIG_DIR}/crypto.ini"
                            fi
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
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "cat > ${DEPLOY_DIR}/.env << 'EOF'
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
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo tee /etc/systemd/system/crypto-thing.service > /dev/null" << 'EOF'
[Unit]
Description=Crypto Thing Tool
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=${DEPLOY_USER}
Group=${DEPLOY_USER}
WorkingDirectory=${DEPLOY_DIR}
EnvironmentFile=${DEPLOY_DIR}/.env
ExecStart=/opt/crypto-thing/cryptool daemon --port 40000
Restart=always
RestartSec=10

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DEPLOY_DIR} ${CONFIG_DIR}

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
                """
            }
        }

        failure {
            echo 'Deployment to Pinky failed!'
            sshagent(credentials: ["${SSH_KEY_ID}"]) {
                sh """
                    echo "Cleaning up failed deployment on ${DEPLOY_HOST}..."
                    ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo rm -rf ${DEPLOY_DIR}"
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
