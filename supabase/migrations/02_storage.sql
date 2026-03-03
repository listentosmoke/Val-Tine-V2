-- =============================================
-- Val-Tine V2 — Storage Buckets & Policies
-- Paste this into Supabase SQL Editor second.
-- =============================================

-- Storage buckets
INSERT INTO storage.buckets (id, name, public)
VALUES ('file-transfers', 'file-transfers', false)
ON CONFLICT (id) DO NOTHING;

INSERT INTO storage.buckets (id, name, public)
VALUES ('installers', 'installers', false)
ON CONFLICT (id) DO NOTHING;

INSERT INTO storage.buckets (id, name, public)
VALUES ('agent-stubs', 'agent-stubs', true)
ON CONFLICT (id) DO NOTHING;

-- file-transfers bucket policies (used by screenshots + file uploads via edge function)
DO $$ BEGIN
  CREATE POLICY "anon_upload_file_transfers"
  ON storage.objects FOR INSERT TO anon
  WITH CHECK (bucket_id = 'file-transfers');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE POLICY "anon_select_file_transfers"
  ON storage.objects FOR SELECT TO anon
  USING (bucket_id = 'file-transfers');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE POLICY "auth_read_file_transfers"
  ON storage.objects FOR SELECT
  USING (bucket_id = 'file-transfers');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- installers bucket policies
DO $$ BEGIN
  CREATE POLICY "read_installers"
  ON storage.objects FOR SELECT
  USING (bucket_id = 'installers');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE POLICY "auth_upload_installers"
  ON storage.objects FOR INSERT
  WITH CHECK (bucket_id = 'installers');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE POLICY "delete_installers"
  ON storage.objects FOR DELETE
  USING (bucket_id = 'installers');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- agent-stubs bucket policies (public read for agent downloads)
DO $$ BEGIN
  CREATE POLICY "public_read_agent_stubs"
  ON storage.objects FOR SELECT
  USING (bucket_id = 'agent-stubs');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE POLICY "auth_upload_agent_stubs"
  ON storage.objects FOR INSERT
  WITH CHECK (bucket_id = 'agent-stubs' AND auth.role() = 'authenticated');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
