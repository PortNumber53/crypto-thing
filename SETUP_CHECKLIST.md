# ⚡ Quick Setup Checklist

Follow these steps to get your GitHub + Jenkins deployment running:

## 📋 Step 1: Repository Setup
- [ ] Create GitHub repository (private)
- [ ] Push your crypto-thing code to GitHub
- [ ] Generate webhook secret: `openssl rand -hex 32`

## 🔧 Step 2: Jenkins Configuration
- [ ] Create new Pipeline job: "crypto-thing-deploy"
- [ ] Copy the pipeline code from `GITHUB_JENKINS_SETUP.md`
- [ ] Update environment variables (DEPLOY_USER, etc.)
- [ ] Add SSH credential: ID `pinky-deploy-key`

## 🌐 Step 3: GitHub Webhook
- [ ] Repository Settings → Webhooks → Add webhook
- [ ] Payload URL: `https://YOUR_JENKINS/github-webhook/`
- [ ] Secret: Use the generated webhook secret
- [ ] Events: "Just the push event"

## 🔒 Step 4: Branch Protection
- [ ] Settings → Branches → Add rule for `main`
- [ ] Require PR reviews ✅
- [ ] Require status checks ✅
- [ ] Restrict pushes (optional) ✅

## 🚀 Step 5: Test Deployment
- [ ] Push a small change to main branch
- [ ] Verify Jenkins job triggers automatically
- [ ] Check deployment on pinky host
- [ ] Confirm service is running

## 🎯 You're Done!

Your crypto tool will now deploy automatically whenever you merge to main! 🚀

**Need help?** Check the detailed guide in `GITHUB_JENKINS_SETUP.md`
