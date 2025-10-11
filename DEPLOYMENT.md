# Crypto Thing Deployment Guide

This guide explains how to deploy the crypto tool using Jenkins CI/CD pipeline or manual deployment scripts.

## 🚀 Quick Deployment Options

### Option 1: Jenkins Pipeline (Recommended)

1. **Create a Jenkins Job**:
   - Create a new Pipeline job in Jenkins
   - Point it to this repository
   - Use `Jenkinsfile` as the pipeline definition

2. **Configure Jenkins Credentials**:
   - **SSH Key Setup**: Create an SSH credential in Jenkins for connecting to 'pinky'
     - Go to Jenkins → Manage Credentials
     - Add a new SSH Username with private key credential
     - **ID**: `pinky-deploy-key` (must match the `SSH_KEY_ID` in Jenkinsfile)
     - **Username**: `jenkins` (or your deployment user on pinky)
     - **Private Key**: Paste your private SSH key content
     - **Passphrase**: Leave empty if no passphrase, or enter passphrase
   - The pipeline uses `sshagent` to authenticate with the remote host

3. **Configure Jenkins Job**:
   - Repository URL: `https://github.com/your-repo/crypto-thing`
   - Branch: `main` (or your deployment branch)
   - Build Triggers: Configure as needed (SCM polling, webhooks, etc.)

4. **Run the Pipeline**:
   - Trigger the build manually or via webhook
   - The pipeline will automatically:
     - Build the Go binary on Jenkins
     - Deploy to `pinky:/opt/crypto-thing/`
     - Create configuration in `pinky:/etc/crypto-thing/`
     - Set up systemd service on the remote host

### Option 2: Manual Deployment

```bash
# Run the deployment script (requires SSH access to pinky)
./deploy.sh
```

## 📋 Post-Deployment Configuration

### 1. Update Database Configuration

SSH into pinky and edit `/etc/crypto-thing/crypto.ini`:
```bash
ssh jenkins@pinky
sudo nano /etc/crypto-thing/crypto.ini
```

Update the database settings:
```ini
[default]
DB_HOST=your-database-host
DB_PORT=5432
DB_NAME=your-database-name
DB_USER=your-username
DB_PASSWORD=your-secure-password
DB_SSLMODE=disable
```

### 2. Configure Coinbase API

Update the Coinbase credentials in `/etc/crypto-thing/crypto.ini`:
```ini
COINBASE_CLOUD_API_KEY_NAME="organizations/YOUR_ORG_ID/apiKeys/YOUR_API_KEY_ID"
COINBASE_CLOUD_API_SECRET="-----BEGIN EC PRIVATE KEY-----\nYOUR_PRIVATE_KEY\n-----END EC PRIVATE KEY-----\n"
```

### 3. Start the Service on Pinky

```bash
# SSH into pinky and manage the service
ssh jenkins@pinky
sudo systemctl enable crypto-thing
sudo systemctl start crypto-thing
sudo systemctl status crypto-thing
sudo journalctl -u crypto-thing -f
```

## 🏗️ Deployment Structure

Remote deployment structure on `pinky`:
```
/opt/crypto-thing/          # Main application directory
├── cryptool              # Built binary
├── migrations/           # Database migrations
├── .env                  # Environment variables (CRYPTO_CONFIG_FILE=/etc/crypto-thing/crypto.ini)
└── tmp/                  # Temporary files

/etc/crypto-thing/         # System configuration
└── crypto.ini           # Application configuration
```

## 🔧 Jenkins Pipeline Features

The Jenkins pipeline (`Jenkinsfile`) includes:

- **Go Environment Setup**: Installs Go 1.22 if not present on Jenkins
- **Application Build**: Builds the crypto tool binary on Jenkins
- **SSH Deployment**: Uses `sshagent` to securely deploy to 'pinky'
- **Remote Directory Creation**: Creates deployment directories with proper permissions
- **File Transfer**: Uses `scp` to copy binary, migrations, and configuration files
- **Environment Setup**: Creates `.env` file with `CRYPTO_CONFIG_FILE=/etc/crypto-thing/crypto.ini`
- **Systemd Service**: Creates and configures systemd service for production use
- **Verification**: Tests SSH connection and binary functionality on remote host

## 📊 Remote Monitoring and Logs

SSH into pinky to monitor the deployment:

```bash
# SSH into pinky
ssh jenkins@pinky

# Check service status
sudo systemctl status crypto-thing

# View application logs
sudo journalctl -u crypto-thing -f

# Manual run for testing
/opt/crypto-thing/cryptool --help
```

## 🔒 Security Considerations

- **SSH Authentication**: Uses Jenkins SSH credentials for secure authentication
- **Sudo Privileges**: Remote commands use `sudo` only when necessary for system directories
- **File Permissions**: Proper ownership and permissions set for security
- **Service User**: Systemd service runs as `jenkins` user (non-root)
- **Network Security**: SSH connection uses `StrictHostKeyChecking=no` for CI/CD automation

## 🛠️ Troubleshooting

### Common Issues

1. **SSH Connection Failed**:
   ```bash
   # Test SSH connection manually
   ssh jenkins@pinky
   # Verify Jenkins credential ID matches SSH_KEY_ID in Jenkinsfile
   ```

2. **Permission Denied on Remote Host**:
   ```bash
   # Ensure jenkins user has sudo privileges on pinky
   ssh jenkins@pinky 'sudo -l'
   ```

3. **Service Won't Start**:
   ```bash
   ssh jenkins@pinky 'sudo systemctl daemon-reload'
   ssh jenkins@pinky 'sudo systemctl reset-failed crypto-thing'
   ```

4. **Configuration Issues**:
   ```bash
   # Test configuration loading on remote host
   ssh jenkins@pinky '/opt/crypto-thing/cryptool --config /etc/crypto-thing/crypto.ini --help'
   ```

## 🔄 Updates and Rollbacks

### Updates
1. Update your code in the repository
2. Trigger the Jenkins pipeline
3. The pipeline will automatically rebuild and redeploy
4. Restart the remote service: `ssh jenkins@pinky 'sudo systemctl restart crypto-thing'`

### Rollbacks
1. Stop the remote service: `ssh jenkins@pinky 'sudo systemctl stop crypto-thing'`
2. Replace binary: `scp /path/to/backup/cryptool jenkins@pinky:/opt/crypto-thing/`
3. Start service: `ssh jenkins@pinky 'sudo systemctl start crypto-thing'`

## 📞 Support

For issues with the deployment process:
1. Check Jenkins console output for pipeline errors
2. Verify SSH connectivity: `ssh jenkins@pinky`
3. Review systemd logs: `ssh jenkins@pinky 'journalctl -u crypto-thing'`
4. Ensure Jenkins credentials are properly configured
5. Verify file permissions and paths on remote host
