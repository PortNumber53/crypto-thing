# 🚀 GitHub + Jenkins CI/CD Setup Guide

This guide will help you set up automated deployment of your crypto tool whenever code is pushed or merged to your GitHub repository.

## 📋 Prerequisites

- GitHub repository (create one if you don't have it yet)
- Jenkins server with SSH access to your deployment target ('pinky')
- SSH key pair for Jenkins authentication
- Webhook secret for secure communication

## 1. 🔐 GitHub Repository Setup

### Create/Update Repository
1. **Create repository** (if needed):
   - Go to [GitHub.com](https://github.com) → New Repository
   - Name: `crypto-thing` (or your preferred name)
   - Set to **Private** for security
   - **Don't initialize** with README (you already have code)

2. **Push your code**:
```bash
# If you haven't pushed yet
git remote add origin https://github.com/YOUR_USERNAME/crypto-thing.git
git branch -M main
git push -u origin main

# If updating existing repo
git add .
git commit -m "feat: add Jenkins deployment pipeline"
git push
```

### Generate Webhook Secret
1. **Go to repository Settings** → Webhooks
2. **Click "Add webhook"** (we'll configure it later)
3. **Generate a secret**:
   ```bash
   # Generate a secure random string (at least 32 characters)
   openssl rand -hex 32
   # Copy this value - you'll need it for both GitHub and Jenkins
   ```

## 2. 🔧 Jenkins Job Configuration

### Create Jenkins Pipeline Job
1. **Open Jenkins** → **New Item**
2. **Item name**: `crypto-thing-deploy`
3. **Type**: **Pipeline**
4. **OK**

### Configure Pipeline
```groovy
pipeline {
    agent any

    environment {
        DEPLOY_HOST = 'pinky'
        DEPLOY_USER = 'YOUR_DEPLOY_USER'  // e.g., 'jenkins'
        DEPLOY_DIR = '/opt/crypto-thing'
        CONFIG_DIR = '/etc/crypto-thing'
        BINARY_NAME = 'cryptool'
        GO_VERSION = '1.22'
        SSH_KEY_ID = 'pinky-deploy-key'
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
                    if ! command -v go &> /dev/null; then
                        echo "Installing Go ${GO_VERSION}..."
                        wget -q https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz
                        sudo rm -rf /usr/local/go
                        sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
                        echo 'export PATH=\$PATH:/usr/local/go/bin' >> ~/.bashrc
                        export PATH=\$PATH:/usr/local/go/bin
                    fi
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

        stage('Deploy to Production') {
            steps {
                sshagent(credentials: ["${SSH_KEY_ID}"]) {
                    sh """
                        echo "Deploying to ${DEPLOY_HOST}..."

                        # Test SSH connection
                        ssh -o StrictHostKeyChecking=no -l ${DEPLOY_USER} ${DEPLOY_HOST} 'echo "SSH connection successful"'

                        # Create deployment directories
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo mkdir -p ${DEPLOY_DIR} ${CONFIG_DIR}"
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "sudo chown -R ${DEPLOY_USER}:${DEPLOY_USER} ${DEPLOY_DIR}"

                        # Copy binary
                        scp ${BINARY_NAME} ${DEPLOY_USER}@${DEPLOY_HOST}:${DEPLOY_DIR}/
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "chmod +x ${DEPLOY_DIR}/${BINARY_NAME}"

                        # Copy migrations if they exist
                        if [ -d "${WORKSPACE}/migrations" ]; then
                            scp -r migrations ${DEPLOY_USER}@${DEPLOY_HOST}:${DEPLOY_DIR}/
                        fi

                        # Copy configuration files if they exist (preserve existing)
                        if [ -f "${WORKSPACE}/crypto.ini.deploy" ]; then
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
CRYPTO_CONFIG_FILE=${CONFIG_DIR}/crypto.ini
EOF"
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
ExecStart=${DEPLOY_DIR}/${BINARY_NAME}
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

        stage('Verify Deployment') {
            steps {
                sshagent(credentials: ["${SSH_KEY_ID}"]) {
                    sh """
                        echo "Verifying deployment on ${DEPLOY_HOST}..."
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "ls -la ${DEPLOY_DIR}/"
                        ssh -l ${DEPLOY_USER} ${DEPLOY_HOST} "${DEPLOY_DIR}/${BINARY_NAME} --help"
                    """
                }
            }
        }
    }

    post {
        success {
            echo 'Deployment completed successfully!'
            sshagent(credentials: ["${SSH_KEY_ID}"]) {
                sh """
                    echo "To manage the service on ${DEPLOY_HOST}:"
                    echo "ssh ${DEPLOY_USER}@${DEPLOY_HOST} 'sudo systemctl enable crypto-thing'"
                    echo "ssh ${DEPLOY_USER}@${DEPLOY_HOST} 'sudo systemctl start crypto-thing'"
                    echo "ssh ${DEPLOY_USER}@${DEPLOY_HOST} 'sudo systemctl status crypto-thing'"
                """
            }
        }

        failure {
            echo 'Deployment failed!'
        }

        cleanup {
            sh "rm -f ${BINARY_NAME}"
        }
    }
}
```

### Configure Build Triggers
1. **Go to job Configuration** → **Build Triggers**
2. **Check**: "GitHub hook trigger for GITScm polling"
3. **Check**: "Poll SCM" (optional, as backup)

**Note**: The `githubPush()` option is not supported in this Jenkins version. Webhook triggers are configured in the job settings above instead of in the Jenkinsfile.

## 3. 🔑 Jenkins Credentials Setup

### SSH Key for Deployment
1. **Jenkins Dashboard** → **Manage Credentials**
2. **Click "Add Credentials"**
3. **Kind**: SSH Username with private key
4. **ID**: `pinky-deploy-key` (matches Jenkinsfile)
5. **Username**: Your deployment user (e.g., `jenkins`)
6. **Private Key**: Paste your private SSH key content
7. **Save**

### GitHub Access Token (Optional)
For accessing private repos or triggering builds:
1. **GitHub Settings** → **Developer settings** → **Personal access tokens**
2. **Generate new token** with `repo` scope
3. **Add to Jenkins** as "Secret text" credential

## 4. 🌐 Webhook Configuration

### GitHub Webhook Setup
1. **Repository Settings** → **Webhooks** → **Add webhook**
2. **Payload URL**: `https://YOUR_JENKINS_URL/      /`
3. **Content type**: `application/json`
4. **Secret**: Paste the webhook secret you generated earlier
5. **Which events**: "Just the push event"
6. **Active**: ✅

### Jenkins Webhook Processing
1. **Install GitHub Plugin** in Jenkins (if not installed)
2. **Configure Global Security**:
   - **CSRF Protection**: Enable proxy compatibility if needed
3. **Test webhook** by pushing a commit

## 5. 🔒 Branch Protection & Deployment Strategy

### Main Branch Protection
1. **Repository Settings** → **Branches**
2. **Add rule** for `main` branch:
   - ✅ **Require pull request reviews**
   - ✅ **Require status checks** (Jenkins build)
   - ✅ **Include administrators**
   - ✅ **Restrict pushes** (optional)

### Deployment Branches Strategy
```
main (protected)
├── develop (staging deployments)
└── feature/* (development only)
```

### Environment-Specific Configuration
```bash
# For different environments, use different config files:
# main → production config
# develop → staging config
# feature/* → development config
```

## 6. 🚀 Deployment Workflow

### Development Workflow
```bash
# Create feature branch
git checkout -b feature/new-functionality

# Develop and test locally
./cryptool migrate status  # Test with local config

# Commit and push
git add .
git commit -m "feat: add new functionality"
git push origin feature/new-functionality

# Create Pull Request
# Jenkins will run tests on PR
# Merge to main when approved
```

### Automatic Deployment
1. **Push to `main`** branch
2. **GitHub webhook** triggers Jenkins
3. **Jenkins builds** and tests the code
4. **Deploys to 'pinky'** if build succeeds
5. **Service restarts** automatically

## 7. 🔍 Monitoring & Troubleshooting

### 4. Configure Systemd Service

**Install the daemon service**:
```bash
# Copy service file and enable
sudo cp devops/systemd/crypto-thing-daemon.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable crypto-thing-daemon

# Start the service
sudo systemctl start crypto-thing-daemon
```

**Verify daemon operation**:
```bash
# Check service status
sudo systemctl status crypto-thing-daemon

# Check health endpoint
curl http://localhost:40000/health

# View logs
sudo journalctl -u crypto-thing-daemon -f
```

**Alternative: Use daemon manager script**:
```bash
chmod +x devops/systemd/daemon-manager.sh
sudo devops/systemd/daemon-manager.sh install
sudo devops/systemd/daemon-manager.sh start
```

### Check Deployment Status
```bash
# On Jenkins
# Go to job → Console Output

# On target host (pinky)
ssh jenkins@pinky
sudo systemctl status crypto-thing
sudo journalctl -u crypto-thing -f
```

### Common Issues & Solutions

** "Permission denied" errors**
- Verify SSH key is correctly configured in Jenkins
- Ensure deployment user has sudo privileges on target host

**❌ "Webhook not triggering"**
- Check webhook URL is correct
- Verify webhook secret matches in both GitHub and Jenkins
- Check Jenkins GitHub plugin is installed

**❌ "Build fails"**
- Check Jenkins has Go installed
- Verify all dependencies are available
- Check file permissions in workspace

## 8. 🔐 Security Best Practices

### Repository Security
- ✅ **Keep repository private** for sensitive credentials
- ✅ **Use environment-specific config files**
- ✅ **Rotate secrets regularly**

### Jenkins Security
- ✅ **Use SSH keys instead of passwords**
- ✅ **Limit Jenkins user permissions**
- ✅ **Enable CSRF protection**

### Deployment Security
- ✅ **Use non-root deployment user**
- ✅ **Restrict file permissions**
- ✅ **Audit deployment logs**

## 🎯 Next Steps

1. **Create your GitHub repository** (if needed)
2. **Configure Jenkins job** with the provided pipeline
3. **Set up SSH credentials** in Jenkins
4. **Configure GitHub webhook**
5. **Test deployment** with a small change
6. **Set up branch protection** rules

Your crypto tool will now deploy automatically whenever you merge code to the main branch! 🚀
