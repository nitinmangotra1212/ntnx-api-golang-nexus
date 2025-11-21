# Complete Deployment Steps for Golang Mock Service

This document provides step-by-step instructions for deploying the Golang Mock Service to Prism Central (PC).

## Prerequisites

- Access to Nutanix internal Maven repositories
- Go 1.21+ installed
- Maven 3.6+ installed
- Git access to push code
- Access to PC deployment environment

---

## Step 1: Prepare Code for Git Push

### 1.1 Clean Up Unnecessary Files

Remove any temporary documentation files and ensure `.gitignore` is properly configured:

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock
# Remove temporary files if any
rm -f ALIGNMENT_SUMMARY.md golang-mock-server
```

### 1.2 Verify Git Status

```bash
git status
```

Ensure only necessary files are staged for commit.

### 1.3 Commit and Push Changes

**For `ntnx-api-golang-mock`:**
```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock

# Add all changes
git add .

# Commit with descriptive message
git commit -m "Align golang-mock structure with az-manager standards"

# Push to remote repository
git push origin <your-branch>
```

**For `ntnx-api-golang-mock-pc`:**
```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock-pc

# Add all changes including generated-code
git add .

# IMPORTANT: Ensure generated-code is included
git add generated-code/

# Verify generated-code files are staged
git status --short generated-code/

# Commit with descriptive message
git commit -m "Align golang-mock-pc with az-manager-pc standards - include generated-code"

# Push to remote repository
git push origin <your-branch>
```

**Note**: The `generated-code` directory contains:
- Generated Go DTOs (`dto/src/models/`)
- Generated protobuf files (`protobuf/swagger/` and `protobuf/mock/`)
- Go module files (`go.mod`)

These files **must be committed** to the repository as they are required for the Go service build.

---

## Step 2: Build Artifacts

### 2.1 Build golang-mock-pc (Code Generation)

This generates all necessary DTOs, protobuf files, and gRPC client code:

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock-pc
mvn clean install -DskipTests -s settings.xml
```

**Expected Output**: 
- Generated code in `generated-code/` directory
- JAR files in `golang-mock-api-codegen/golang-mock-grpc-client/target/`
- Build should complete with `BUILD SUCCESS`

**Verify**:
```bash
ls -lh golang-mock-api-codegen/golang-mock-grpc-client/target/*.jar
```

### 2.2 Build golang-mock Service (Go Binary)

Build the Go service binary for Linux:

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock
make build
```

**Expected Output**: 
- Binary file: `golang-mock-server` in the root directory
- Binary should be compiled for `linux/amd64`

**Verify**:
```bash
ls -lh golang-mock-server
file golang-mock-server  # Should show: ELF 64-bit LSB executable, x86-64
```

---

## Step 3: Build prism-service (Adonis)

### 3.1 Verify Configuration

Ensure `prism-service/pom.xml` has the correct dependency:

```xml
<golang-mock-grpc-client.version>17.0.0-SNAPSHOT</golang-mock-grpc-client.version>
```

And the dependency block:
```xml
<dependency>
  <artifactId>golang-mock-grpc-client</artifactId>
  <groupId>com.nutanix.nutanix-core.ntnx-api.golang-mock-pc</groupId>
  <version>${golang-mock-grpc-client.version}</version>
</dependency>
```

### 3.2 Verify application.yaml

Check `prism-service/src/main/resources/application.yaml` has:

```yaml
golang-mock:
  port: 9090
```

And controller packages:
```yaml
controller:
  packages:
    - mock.v4.config.server.controllers
    - mock.v4.config.server.services
    - mock.v4.server.configuration
```

### 3.3 Build Adonis

```bash
cd /Users/nitin.mangotra/ntnx-api-prism-service
mvn clean install -DskipTests -s settings.xml
```

**Expected Output**: 
- JAR file: `target/prism-service-17.6.0-SNAPSHOT.jar`
- Build should complete with `BUILD SUCCESS`

**Verify**:
```bash
ls -lh target/prism-service*.jar
```

---

## Step 4: Deploy to Prism Central

### 4.1 Deploy golang-mock-pc Artifacts to Maven Repository

The generated JAR files need to be deployed to Nutanix internal Maven repository so that `prism-service` can resolve the dependency:

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock-pc
mvn deploy -DskipTests -s settings.xml
```

**Note**: This requires:
- Proper Maven credentials configured in `settings.xml`
- Access to Nutanix internal Nexus repository
- Appropriate permissions to deploy artifacts

**Verify Deployment**:
- Check Nexus repository for `golang-mock-grpc-client-17.0.0-SNAPSHOT.jar`
- Ensure the artifact is accessible from the build environment

### 4.2 Package golang-mock-server Binary

The Go service binary needs to be packaged for deployment:

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock

# Create deployment package (example structure)
mkdir -p deploy/package
cp golang-mock-server deploy/package/
cp config/config.yaml deploy/package/  # If needed
cp serviceInfo.yaml deploy/package/     # If exists

# Create tarball (example)
tar -czf golang-mock-server-17.0.0-SNAPSHOT.tar.gz -C deploy/package .
```

**Note**: The exact packaging format depends on your deployment pipeline requirements.

### 4.3 Deploy golang-mock-server to PC

Deploy the binary to the target PC environment:

**Option A: Direct Deployment**
```bash
# Copy binary to PC node
scp golang-mock-server <pc-node>:/opt/nutanix/bin/

# SSH into PC node and start service
ssh <pc-node>
sudo systemctl start golang-mock-service  # Or appropriate service manager
```

**Option B: Using Deployment Pipeline**
- Follow your organization's standard deployment process
- Upload the binary package to the deployment system
- Configure service to run on port `9090`
- Ensure service starts automatically on boot

**Service Configuration**:
- Port: `9090` (as configured in `application.yaml`)
- Protocol: gRPC
- Health Check: Configure appropriate health check endpoint

---

## Step 5: Deploy prism-service (Adonis) to PC

### 5.1 Deploy Adonis JAR

Deploy the built Adonis JAR to PC:

```bash
cd /Users/nitin.mangotra/ntnx-api-prism-service

# Copy JAR to PC deployment location
scp target/prism-service-17.6.0-SNAPSHOT.jar <pc-node>:/opt/nutanix/prism-service/

# Or use your deployment pipeline
```

### 5.2 Restart Adonis Service

Restart the Adonis service to load the new configuration:

```bash
ssh <pc-node>
sudo systemctl restart prism-service  # Or appropriate service manager
```

### 5.3 Verify Service Status

Check that both services are running:

```bash
# Check golang-mock-server
ps aux | grep golang-mock-server
netstat -tlnp | grep 9090

# Check Adonis
ps aux | grep prism-service
curl http://localhost:9440/actuator/health  # Or appropriate health endpoint
```

---

## Step 6: Verification and Testing

### 6.1 Verify gRPC Service

Test the gRPC service directly:

```bash
# Using grpcurl (if available)
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext localhost:9090 mock.v4.config.CatService/ListCats
```

### 6.2 Verify REST API via Adonis

Test the REST API endpoint through Adonis:

```bash
# Get authentication token first
TOKEN=$(curl -X POST https://<pc-ip>:9440/api/nutanix/v3/users/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<password>"}' | jq -r '.access_token')

# Test the Cat endpoint
curl -X GET "https://<pc-ip>:9440/api/mock/v4/config/cats" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json"
```

### 6.3 Check Logs

Monitor logs for any errors:

```bash
# golang-mock-server logs
tail -f /var/log/golang-mock-server.log  # Or appropriate log location

# Adonis logs
tail -f /var/log/prism-service.log  # Or appropriate log location
```

---

## Troubleshooting

### Issue: Maven Build Fails with "artifact not found"

**Solution**: Ensure `golang-mock-pc` artifacts are deployed to Maven repository (Step 4.1)

### Issue: Go Build Fails with Import Errors

**Solution**: 
- Verify `go.mod` has correct `replace` directives pointing to `golang-mock-pc/generated-code`
- Run `go mod tidy` to resolve dependencies

### Issue: Service Not Starting

**Solution**:
- Check port `9090` is not already in use: `netstat -tlnp | grep 9090`
- Verify binary has execute permissions: `chmod +x golang-mock-server`
- Check service logs for specific error messages

### Issue: Adonis Cannot Connect to gRPC Service

**Solution**:
- Verify `golang-mock-server` is running on port `9090`
- Check `application.yaml` has correct port configuration
- Verify network connectivity between Adonis and gRPC service

---

## Summary Checklist

- [ ] Code committed and pushed to Git
- [ ] `golang-mock-pc` built successfully
- [ ] `golang-mock-server` binary created
- [ ] `prism-service` built successfully
- [ ] `golang-mock-pc` artifacts deployed to Maven
- [ ] `golang-mock-server` deployed to PC
- [ ] `prism-service` deployed to PC
- [ ] Services running and healthy
- [ ] REST API endpoints accessible
- [ ] Logs show no errors

---

## Next Steps

After successful deployment:

1. **Monitor**: Set up monitoring and alerting for the service
2. **Documentation**: Update API documentation if needed
3. **Testing**: Run integration tests against deployed services
4. **Performance**: Monitor performance metrics and optimize if needed

---

## Important Notes

- **Version Alignment**: Ensure all versions are aligned:
  - `golang-mock-pc`: `17.0.0-SNAPSHOT`
  - `golang-mock-grpc-client.version` in `prism-service/pom.xml`: `17.0.0-SNAPSHOT`
  - `prism-service`: `17.6.0-SNAPSHOT`

- **Port Configuration**: 
  - gRPC service runs on port `9090`
  - Adonis REST API runs on port `9440` (default)

- **Service Dependencies**: 
  - Adonis depends on `golang-mock-grpc-client` JAR
  - Adonis acts as REST-to-gRPC gateway
  - gRPC service must be running before Adonis can route requests

---

**Last Updated**: 2025-11-21
**Version**: 1.0

