# Deployment Guide: golang-mock Service to PC

## Overview
This guide outlines the steps to deploy the golang-mock service to PC (Prism Central) environment.

---

## Prerequisites

1. ✅ Code is committed and pushed to Git repository
2. ✅ All builds are successful
3. ✅ Access to PC environment
4. ✅ Deployment credentials configured

---

## Step 1: Prepare Code for Git Push

### 1.1 Clean Up Unnecessary Files

**In `ntnx-api-golang-mock`:**
```bash
cd ntnx-api-golang-mock

# Remove temporary documentation
rm -f ALIGNMENT_SUMMARY.md

# Remove binary (will be regenerated)
rm -f golang-mock-server

# Ensure .gitignore exists (see .gitignore file)
```

**In `ntnx-api-golang-mock-pc`:**
```bash
cd ntnx-api-golang-mock-pc

# Remove any temporary files
# Ensure .gitignore exists
```

### 1.2 Verify .gitignore Files

Ensure both repositories have proper `.gitignore` files:
- `ntnx-api-golang-mock/.gitignore` - ignores binaries, IDE files, logs
- `ntnx-api-golang-mock-pc/.gitignore` - ignores Maven target/, IDE files

### 1.3 Commit and Push to Git

```bash
# For golang-mock service
cd ntnx-api-golang-mock
git add .
git commit -m "Align golang-mock structure with az-manager standards"
git push origin <branch-name>

# For golang-mock-pc (code generation)
cd ntnx-api-golang-mock-pc
git add .
git commit -m "Align golang-mock-pc with az-manager-pc standards"
git push origin <branch-name>
```

---

## Step 2: Build Artifacts

### 2.1 Build golang-mock-pc (Code Generation)

```bash
cd ntnx-api-golang-mock-pc
mvn clean install -DskipTests -s settings.xml
```

**Expected Output:**
- ✅ All 12 modules build successfully
- ✅ Proto files generated in `generated-code/protobuf/swagger/mock/v4/config/`
- ✅ Java DTOs and interfaces generated
- ✅ gRPC client code generated

### 2.2 Build golang-mock Service

```bash
cd ntnx-api-golang-mock
make build
```

**Expected Output:**
- ✅ Binary created: `golang-mock-server`
- ✅ Linux binary ready for deployment

---

## Step 3: Build prism-service (Adonis)

### 3.1 Update application.yaml

Ensure `application.yaml` includes all required packages:
```yaml
controller:
  packages: 
    ...
    com.nutanix.mock.controller, \
    mock.v4.config.server.controllers, \
    mock.v4.config.server.services, \
    mock.v4.server.configuration
```

### 3.2 Build Adonis

```bash
cd ntnx-api-prism-service
mvn clean install -DskipTests -s settings.xml
```

**Expected Output:**
- ✅ JAR file: `target/prism-service-*.jar`
- ✅ Contains golang-mock controllers and services

---

## Step 4: Deploy to PC

### 4.1 Deploy golang-mock-server Binary

1. **Copy binary to PC:**
   ```bash
   scp golang-mock-server <pc-user>@<pc-ip>:/usr/local/nutanix/golang-mock/
   ```

2. **Set permissions:**
   ```bash
   ssh <pc-user>@<pc-ip>
   chmod +x /usr/local/nutanix/golang-mock/golang-mock-server
   ```

3. **Create systemd service (optional):**
   ```bash
   # Create service file
   sudo nano /etc/systemd/system/golang-mock.service
   ```
   
   Service file content:
   ```ini
   [Unit]
   Description=Golang Mock gRPC Service
   After=network.target
   
   [Service]
   Type=simple
   User=nutanix
   WorkingDirectory=/usr/local/nutanix/golang-mock
   ExecStart=/usr/local/nutanix/golang-mock/golang-mock-server -port 9090
   Restart=always
   
   [Install]
   WantedBy=multi-user.target
   ```

4. **Start service:**
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable golang-mock
   sudo systemctl start golang-mock
   sudo systemctl status golang-mock
   ```

### 4.2 Deploy prism-service JAR

1. **Copy JAR to Adonis:**
   ```bash
   scp target/prism-service-*.jar <pc-user>@<pc-ip>:/usr/local/nutanix/adonis/lib/
   ```

2. **Restart Adonis:**
   ```bash
   ssh <pc-user>@<pc-ip>
   genesis stop adonis
   genesis start adonis
   ```

### 4.3 Configure Mercury (Nginx Gateway)

1. **Create Mercury config:**
   ```bash
   ssh <pc-user>@<pc-ip>
   sudo nano /home/nutanix/config/mercury/mercury_request_handler_config_apimock_golang.json
   ```

2. **Config content:**
   ```json
   {
     "routes": [
       {
         "path": "/api/mock/v4.0.a1/*",
         "backend": "adonis",
         "port": 8888
       }
     ]
   }
   ```

3. **Restart Mercury:**
   ```bash
   genesis stop mercury
   genesis start mercury
   ```

---

## Step 5: Verify Deployment

### 5.1 Check golang-mock Service

```bash
# Check if service is running
ssh <pc-user>@<pc-ip>
systemctl status golang-mock

# Check logs
journalctl -u golang-mock -f

# Test gRPC directly
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext localhost:9090 mock.v4.config.CatService/ListCats
```

### 5.2 Check Adonis

```bash
# Check Adonis logs
tail -f ~/data/logs/adonis.out

# Verify packages are loaded
grep "mock.v4.config" ~/data/logs/adonis.out
```

### 5.3 Test REST API

```bash
# Test via Mercury (external)
curl -k https://<pc-ip>/api/mock/v4.0.a1/config/cats

# Test via Adonis (internal)
curl -k https://<pc-ip>:9440/api/mock/v4.0.a1/config/cats
```

---

## API Version Format: v4.0.a1

### Why v4.0.a1 instead of v4?

The version format `v4.0.a1` follows Nutanix API versioning scheme:

- **v4**: Major API version (family)
- **0**: Minor version (revision)
- **a1**: Release type and revision
  - **a** = Alpha release
  - **1** = First alpha release
  - Other types: **b** (beta), **r** (release candidate), no letter = GA

**Version Progression Example:**
```
v4.0.a1 → v4.0.a2 → v4.0.b1 → v4.0.b2 → v4.0.r1 → v4.0.0 (GA)
```

**Why this format?**
1. **API Stability Tracking**: Different versions can coexist
2. **Backward Compatibility**: Older clients can use previous versions
3. **Release Management**: Clear progression from alpha → beta → GA
4. **Nutanix Standard**: Matches pattern used by az-manager, guru, and other services

**Example URLs:**
- `/api/mock/v4.0.a1/config/cats` - Alpha version (current)
- `/api/mock/v4.0.0/config/cats` - GA version (when released)

**Note:** The version is defined in:
- `golang-mock-api-definitions/defs/namespaces/mock/versioned/v4/metadata/info.yaml`
- This determines the URL path: `/api/mock/v4.0.a1/`

---

## Troubleshooting

### Issue: Service not starting
- Check logs: `journalctl -u golang-mock -n 50`
- Verify port 9090 is not in use: `netstat -tuln | grep 9090`
- Check binary permissions: `ls -l /usr/local/nutanix/golang-mock/golang-mock-server`

### Issue: Adonis not routing to golang-mock
- Verify application.yaml has correct packages
- Check Adonis logs for errors
- Verify golang-mock service is running on port 9090
- Check serviceInfo.yaml configuration

### Issue: REST API not accessible
- Verify Mercury configuration
- Check Adonis is running: `genesis status adonis`
- Verify routing: Check Mercury logs

---

## Files Modified for Deployment

### ntnx-api-golang-mock-pc:
- ✅ POM files updated to version 17.0.0-SNAPSHOT
- ✅ publishCode.sh scripts fixed
- ✅ application.yaml packages added
- ✅ Duplicate protobuf files removed

### ntnx-api-golang-mock:
- ✅ Structure aligned with az-manager
- ✅ Port changed to 9090
- ✅ Makefile updated
- ✅ Import paths fixed

### ntnx-api-prism-service:
- ✅ application.yaml updated with mock packages
- ✅ Backend service config: port 9090

---

## Summary

1. ✅ **Code Preparation**: Clean up, commit, push to Git
2. ✅ **Build**: golang-mock-pc → golang-mock → prism-service
3. ✅ **Deploy**: Binary + JAR + Mercury config
4. ✅ **Verify**: Service status + API testing

**Ready for deployment!** 🚀

