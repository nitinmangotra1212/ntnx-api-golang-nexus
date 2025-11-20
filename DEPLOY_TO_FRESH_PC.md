# Deploy to Fresh PC (10.112.90.239) - Complete Manual Steps

**Clean deployment on a fresh PC to avoid environment issues**

---

## ✅ Prerequisites Check

From your Mac, verify you can SSH to the new PC:

```bash
ssh nutanix@10.112.90.239
# Enter password
# If connected, type: exit
```

---

## 📦 Step 1: Prepare Files on PC

SSH to the fresh PC:

```bash
ssh nutanix@10.112.90.239
```

Once logged in, run:

```bash
# Create golang-mock service directory
mkdir -p ~/golang-mock-service

# Create API artifacts directory
mkdir -p ~/api_artifacts/mock/v4.r0.a1/golang-mock-api-definitions-1.0.0-SNAPSHOT

# Backup existing Adonis JAR (if it exists)
if [ -f /usr/local/nutanix/adonis/lib/prism-service-17.6.0-SNAPSHOT.jar ]; then
    sudo cp /usr/local/nutanix/adonis/lib/prism-service-17.6.0-SNAPSHOT.jar \
         /usr/local/nutanix/adonis/lib/prism-service-backup-$(date +%Y%m%d-%H%M%S).jar
    echo "✅ Adonis JAR backed up"
else
    echo "⚠️  No existing Adonis JAR to backup (fresh PC)"
fi

echo "✅ Directories created"

# Exit PC for now
exit
```

---

## 📤 Step 2: Copy Files from Mac to PC

**Run these commands from your Mac terminal:**

### 2a. Copy gRPC Server Binary

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock
scp -O bin/grpc-server-linux nutanix@10.112.90.239:~/golang-mock-service/grpc-server
```

**Expected:** File transfer progress, then complete.

---

### 2b. Copy Adonis JAR

```bash
scp -O /Users/nitin.mangotra/ntnx-api-prism-service/target/prism-service-17.6.0-SNAPSHOT.jar \
  nutanix@10.112.90.239:/tmp/prism-service-17.6.0-SNAPSHOT.jar
```

**Note:** We copy to /tmp first, then will move it with sudo.

---

### 2c. Copy API Artifacts

```bash
cd /Users/nitin.mangotra/ntnx-api-golang-mock-pc/golang-mock-api-definitions
scp -O -r target/generated-api-artifacts/* \
  nutanix@10.112.90.239:~/api_artifacts/mock/v4.r0.a1/golang-mock-api-definitions-1.0.0-SNAPSHOT/
```

**Expected:** Multiple YAML files being copied (api-manifest, swagger files, etc.)

---

## ⚙️ Step 3: Configure PC

SSH back to PC:

```bash
ssh nutanix@10.112.90.239
```

### 3a. Move Adonis JAR to Correct Location

```bash
# Move JAR from /tmp to adonis/lib
sudo mv /tmp/prism-service-17.6.0-SNAPSHOT.jar /usr/local/nutanix/adonis/lib/

# Verify it's there
ls -lh /usr/local/nutanix/adonis/lib/prism-service-17.6.0-SNAPSHOT.jar

# Verify it contains mock classes
jar tf /usr/local/nutanix/adonis/lib/prism-service-17.6.0-SNAPSHOT.jar | grep "com/nutanix/mock" | head -10
```

**Expected:** Should show mock classes like:
```
com/nutanix/mock/controller/
com/nutanix/mock/client/
```

---

### 3b. Make gRPC Server Executable

```bash
chmod +x ~/golang-mock-service/grpc-server
ls -lh ~/golang-mock-service/grpc-server
```

**Expected:** File shows `-rwxr-xr-x` (executable)

---

### 3c. Configure lookup_cache.json

**Check if file exists:**

```bash
ls -la ~/api_artifacts/lookup_cache.json
```

**If file EXISTS, edit it:**

```bash
nano ~/api_artifacts/lookup_cache.json
```

Add this entry (append with a comma if other entries exist):

```json
,
{
  "apiPath": "/mock/v4.0.a1/config",
  "artifactPath": "mock/v4.r0.a1/golang-mock-api-definitions-1.0.0-SNAPSHOT"
}
```

Press `Ctrl+O` to save, `Enter`, then `Ctrl+X` to exit.

---

**If file DOES NOT EXIST, create it:**

```bash
cat > ~/api_artifacts/lookup_cache.json << 'EOF'
[
  {
    "apiPath": "/mock/v4.0.a1/config",
    "artifactPath": "mock/v4.r0.a1/golang-mock-api-definitions-1.0.0-SNAPSHOT"
  }
]
EOF
```
    
**Verify:**

```bash
cat ~/api_artifacts/lookup_cache.json
```

---

### 3d. Verify API Artifacts Copied

```bash
ls -la ~/api_artifacts/mock/v4.r0.a1/golang-mock-api-definitions-1.0.0-SNAPSHOT/

# Should see:
# - api-manifest-1.0.0-SNAPSHOT.json (CRITICAL!)
# - swagger-all-1.0.0-SNAPSHOT.yaml
# - swagger-mock-v4.r1-all.yaml
# - object-type-mapping-1.0.0-SNAPSHOT.yaml
# And others...
```

---

### 3e. Create Mercury Config

```bash
sudo mkdir -p /home/nutanix/config/mercury

sudo tee /home/nutanix/config/mercury/mercury_request_handler_config_apimock_golang.json > /dev/null << 'EOF'
{
  "api_path_config_list" : [
    {
      "api_path" : "/api/mock/v4.0.a1",
      "handler_list" : [
        {
          "priority" : 1,
          "port" : 8888,
          "transport_options" : "kHttp",
          "external_request_auth_options" : "kAllowAnyAuthenticatedUserExt",
          "internal_request_auth_options" : "kAllowAnyAuthenticatedUserInt"
        }
      ]
    }
  ]
}
EOF

# Verify
cat /home/nutanix/config/mercury/mercury_request_handler_config_apimock_golang.json
```

---

### 3f. Check /etc/hosts for ZooKeeper

```bash
cat /etc/hosts | grep -E "zk[123]"
```

**If zk entries are MISSING:**

```bash
sudo tee -a /etc/hosts > /dev/null << 'EOF'
127.0.0.1   zk1
127.0.0.1   zk2
127.0.0.1   zk3
EOF

# Verify
cat /etc/hosts | grep zk
```

---

## 🚀 Step 4: Start Services

### 4a. Start gRPC Server

```bash
cd ~/golang-mock-service
nohup ./grpc-server > grpc-server.log 2>&1 &

# Wait 3 seconds
sleep 3

# Check if running
ps aux | grep grpc-server | grep -v grep

# Check logs
tail -20 grpc-server.log

# Check port
netstat -tuln | grep 50051
```

**Expected:**
- Process should be running
- Logs should show: `✅ gRPC server listening on [::]:50051`
- Port 50051 should be `LISTEN`

---

### 4b. Check Genesis Status

```bash
genesis status
```

**Ensure these are RUNNING:**
- `zookeeper`
- `ergon`
- `insights_server`

**If any are DOWN:**

```bash
cluster start
```

Wait 2-3 minutes for services to start.

---

### 4c. Restart Adonis and Mercury

```bash
genesis stop adonis mercury

# Wait for full stop
sleep 10

# Start services
cluster start

# This will take 2-3 minutes!
```

---

### 4d. Monitor Adonis Startup

```bash
# Watch logs (this will keep updating)
tail -f ~/data/logs/adonis.out

# Look for: "Started PrismServiceApplication in XX.XXX seconds"
# Press Ctrl+C when you see it
```

**Or check periodically:**

```bash
tail -100 ~/data/logs/adonis.out | grep "Started PrismServiceApplication"
```

---

## 🧪 Step 5: Test Deployment

### 5a. Test from PC itself (optional)

```bash
# Test gRPC locally
grpcurl -plaintext localhost:50051 list

# If grpcurl not installed, skip this
```

---

### 5b. Exit PC and Test from Mac

```bash
exit
```

From your Mac:

**Test 1: Direct gRPC**

```bash
grpcurl -plaintext 10.112.90.239:50051 mock.v4.config.CatService/ListCats
```

**Expected:** JSON response with list of cats

---

**Test 2: List all gRPC services**

```bash
grpcurl -plaintext 10.112.90.239:50051 list
```

**Expected:** Should show `mock.v4.config.CatService`

---

**Test 3: REST via Adonis**

```bash
curl -k https://10.112.90.239/api/mock/v4.0.a1/config/cats
```

**Expected:** JSON response with:
- `$objectType: "mock.v4.config.Cat"`
- `$reserved` object
- `metadata` object  
- `links` array

---

**Test 4: Create Cat**

```bash
curl -k -X POST https://10.112.90.239/api/mock/v4.0.a1/config/cats \
  -H "Content-Type: application/json" \
  -d '{"catName":"Fluffy","catType":"TYPE1","description":"Test via fresh PC"}'
```

---

**Test 5: Get Specific Cat**

```bash
curl -k https://10.112.90.239/api/mock/v4.0.a1/config/cats/5
```

---

## 🎉 Success Criteria

After deployment, you should see:

1. ✅ gRPC server running on PC
2. ✅ `grpcurl -plaintext 10.112.90.239:50051 list` shows services
3. ✅ `grpcurl ... ListCats` returns cat data
4. ✅ Adonis logs show "Started PrismServiceApplication"
5. ✅ `curl https://10.112.90.239/api/mock/v4.0.a1/config/cats` works
6. ✅ Response includes Nutanix v4 fields ($objectType, $reserved, etc.)

---

## 🔍 Troubleshooting

### Issue: gRPC server not starting

```bash
ssh nutanix@10.112.90.239
cd ~/golang-mock-service
cat grpc-server.log
```

**Common fix:** Wrong architecture binary

```bash
file grpc-server
# Should show: "ELF 64-bit LSB executable, x86-64"
```

---

### Issue: Adonis not starting

```bash
ssh nutanix@10.112.90.239
tail -200 ~/data/logs/adonis.out | grep -i "error\|exception"
```

**Check ZooKeeper:**

```bash
genesis status zookeeper
cat /etc/hosts | grep zk
```

---

### Issue: Port 50051 not accessible from Mac

```bash
ssh nutanix@10.112.90.239

# Check firewall
sudo iptables -L INPUT -n | grep 50051

# Open port if needed
sudo iptables -I INPUT -p tcp --dport 50051 -j ACCEPT
```

---

## 🔄 UPDATE STEPS (After Initial Deployment)

**Use this section when you've made code changes and need to update an existing deployment.**

### Scenario 1: Go gRPC Service Code Changed

**When:** You modified Go code in `ntnx-api-golang-mock` (e.g., `grpc/cat_grpc_service.go`)

**Steps:**

```bash
# 1. Rebuild Linux binary locally (on Mac)
cd /Users/nitin.mangotra/ntnx-api-golang-mock
GOOS=linux GOARCH=amd64 go build -o bin/grpc-server-linux ./cmd/grpc-server/main.go

# 2. Stop gRPC server on PC
ssh nutanix@10.112.90.239 "pkill -f grpc-server"

# 3. Copy new binary to PC
scp -O bin/grpc-server-linux nutanix@10.112.90.239:~/golang-mock-service/grpc-server

# 4. Start gRPC server on PC
ssh nutanix@10.112.90.239 "cd ~/golang-mock-service && chmod +x grpc-server && nohup ./grpc-server > grpc-server.log 2>&1 &"

# 5. Verify it's running
ssh nutanix@10.112.90.239 "sleep 3 && ps aux | grep grpc-server | grep -v grep && netstat -tuln | grep 50051"
```

**✅ Expected:** gRPC server restarted with new code. **No Adonis restart needed!**

---

### Scenario 2: Adonis/Java Code Changed

**When:** You modified Java code in `ntnx-api-prism-service` or rebuilt the JAR with new generated code

**Steps:**

```bash
# 1. Rebuild Adonis JAR locally (on Mac)
cd /Users/nitin.mangotra/ntnx-api-prism-service
mvn clean package -DskipTests

# 2. Backup old JAR on PC
ssh nutanix@10.112.90.239 "sudo cp /usr/local/nutanix/adonis/lib/prism-service-17.6.0-SNAPSHOT.jar /usr/local/nutanix/adonis/lib/prism-service-backup-\$(date +%Y%m%d-%H%M%S).jar"

# 3. Copy new JAR to PC
scp -O target/prism-service-17.6.0-SNAPSHOT.jar nutanix@10.112.90.239:/tmp/prism-service-17.6.0-SNAPSHOT.jar

# 4. Move JAR to Adonis lib directory
ssh nutanix@10.112.90.239 "sudo mv /tmp/prism-service-17.6.0-SNAPSHOT.jar /usr/local/nutanix/adonis/lib/"

# 5. Restart Adonis and Mercury
ssh nutanix@10.112.90.239 "genesis stop adonis mercury && sleep 10 && cluster start"

# 6. Monitor Adonis startup (wait 2-3 minutes)
ssh nutanix@10.112.90.239 "tail -f ~/data/logs/adonis.out | grep 'Started PrismServiceApplication'"
```

**✅ Expected:** Adonis restarted with new JAR. **Press Ctrl+C when you see "Started PrismServiceApplication"**

---

### Scenario 3: API Definitions Changed (YAML/Proto)

**When:** You modified YAML files in `ntnx-api-golang-mock-pc` or regenerated proto files

**Steps:**

```bash
# 1. Regenerate code locally (on Mac)
cd /Users/nitin.mangotra/ntnx-api-golang-mock-pc

# Rebuild Maven modules to regenerate Java code
mvn clean install -DskipTests

# Regenerate Go proto files (if proto changed)
./generate-grpc.sh

# 2. Rebuild Adonis JAR (includes new generated code)
cd /Users/nitin.mangotra/ntnx-api-prism-service
mvn clean package -DskipTests

# 3. Copy new API artifacts to PC
ssh nutanix@10.112.90.239 "mkdir -p ~/api_artifacts/mock/v4.r0.a1/golang-mock-api-definitions-1.0.0-SNAPSHOT"
cd /Users/nitin.mangotra/ntnx-api-golang-mock-pc/golang-mock-api-definitions
scp -O -r target/generated-api-artifacts/* nutanix@10.112.90.239:~/api_artifacts/mock/v4.r0.a1/golang-mock-api-definitions-1.0.0-SNAPSHOT/

# 4. Update Adonis JAR (follow Scenario 2 steps above)
scp -O /Users/nitin.mangotra/ntnx-api-prism-service/target/prism-service-17.6.0-SNAPSHOT.jar nutanix@10.112.90.239:/tmp/
ssh nutanix@10.112.90.239 "sudo mv /tmp/prism-service-17.6.0-SNAPSHOT.jar /usr/local/nutanix/adonis/lib/"

# 5. Restart Adonis
ssh nutanix@10.112.90.239 "genesis stop adonis mercury && sleep 10 && cluster start"
```

**✅ Expected:** New API definitions deployed. Adonis restarted with new generated code.

---

### Scenario 4: Both Go Code AND Adonis Code Changed

**When:** You modified both Go service and Java/Adonis code

**Steps:**

```bash
# Follow Scenario 1 (Go code update) - NO Adonis restart needed
# Then follow Scenario 2 (Adonis code update) - This will restart Adonis

# OR combine into one session:

# 1. Rebuild both locally
cd /Users/nitin.mangotra/ntnx-api-golang-mock
GOOS=linux GOARCH=amd64 go build -o bin/grpc-server-linux ./cmd/grpc-server/main.go

cd /Users/nitin.mangotra/ntnx-api-prism-service
mvn clean package -DskipTests

# 2. Stop services on PC
ssh nutanix@10.112.90.239 "pkill -f grpc-server"

# 3. Copy Go binary
scp -O /Users/nitin.mangotra/ntnx-api-golang-mock/bin/grpc-server-linux nutanix@10.112.90.239:~/golang-mock-service/grpc-server

# 4. Copy Adonis JAR
scp -O /Users/nitin.mangotra/ntnx-api-prism-service/target/prism-service-17.6.0-SNAPSHOT.jar nutanix@10.112.90.239:/tmp/
ssh nutanix@10.112.90.239 "sudo mv /tmp/prism-service-17.6.0-SNAPSHOT.jar /usr/local/nutanix/adonis/lib/"

# 5. Start gRPC server
ssh nutanix@10.112.90.239 "cd ~/golang-mock-service && chmod +x grpc-server && nohup ./grpc-server > grpc-server.log 2>&1 &"

# 6. Restart Adonis
ssh nutanix@10.112.90.239 "genesis stop adonis mercury && sleep 10 && cluster start"
```

---

### Quick Update Checklist

**For Go code changes only:**
- [ ] Rebuild `grpc-server-linux` binary
- [ ] Stop old gRPC server on PC
- [ ] Copy new binary to PC
- [ ] Start gRPC server
- [ ] ✅ **DONE** (no Adonis restart needed)

**For Adonis/Java changes only:**
- [ ] Rebuild `prism-service-17.6.0-SNAPSHOT.jar`
- [ ] Backup old JAR on PC
- [ ] Copy new JAR to PC
- [ ] Restart Adonis and Mercury
- [ ] Wait 2-3 minutes for startup
- [ ] ✅ **DONE**

**For API definition changes:**
- [ ] Rebuild Maven modules (`mvn clean install`)
- [ ] Rebuild Adonis JAR
- [ ] Copy new API artifacts to PC
- [ ] Copy new Adonis JAR to PC
- [ ] Restart Adonis
- [ ] ✅ **DONE**

---

### Testing After Update

```bash
# Test gRPC directly
grpcurl -plaintext 10.112.90.239:50051 mock.v4.config.CatService/ListCats

# Test REST via Adonis
curl -k https://10.112.90.239/api/mock/v4.0.a1/config/cats

# Check logs if issues
ssh nutanix@10.112.90.239 "tail -50 ~/golang-mock-service/grpc-server.log"
ssh nutanix@10.112.90.239 "tail -100 ~/data/logs/adonis.out | grep -i error"
```

---

## 📋 Quick Reference

**PC IP:** 10.112.90.239

**gRPC Access:** `grpcurl -plaintext 10.112.90.239:50051 mock.v4.config.CatService/ListCats`

**REST Access:** `curl -k https://10.112.90.239/api/mock/v4.0.a1/config/cats`

**Check Adonis:** `ssh nutanix@10.112.90.239 "tail -50 ~/data/logs/adonis.out"`

**Check gRPC:** `ssh nutanix@10.112.90.239 "ps aux | grep grpc-server"`

---

## ✨ Summary

This fresh PC should work much better! The steps are:

1. Create directories on PC
2. Copy files (gRPC binary, Adonis JAR, API artifacts)
3. Configure PC (lookup_cache.json, Mercury config, /etc/hosts)
4. Start gRPC server
5. Restart Adonis
6. Test!

**Total time:** ~15-20 minutes

**Good luck!** 🚀

