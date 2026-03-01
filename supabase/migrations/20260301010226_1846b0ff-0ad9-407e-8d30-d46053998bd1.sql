
-- Create installers bucket
INSERT INTO storage.buckets (id, name, public)
VALUES ('installers', 'installers', false)
ON CONFLICT (id) DO NOTHING;

-- Create agent-stubs bucket (public for agent downloads)
INSERT INTO storage.buckets (id, name, public)
VALUES ('agent-stubs', 'agent-stubs', true)
ON CONFLICT (id) DO NOTHING;

-- Installers bucket policies
CREATE POLICY "Users can download installers"
ON storage.objects FOR SELECT
USING (bucket_id = 'installers');

CREATE POLICY "Authenticated users can upload installers"
ON storage.objects FOR INSERT
WITH CHECK (bucket_id = 'installers');

CREATE POLICY "Users can delete installers"
ON storage.objects FOR DELETE
USING (bucket_id = 'installers');

-- Agent stubs bucket policies
CREATE POLICY "Public read access for agent stubs"
ON storage.objects FOR SELECT
USING (bucket_id = 'agent-stubs');

CREATE POLICY "Authenticated users can upload stubs"
ON storage.objects FOR INSERT
WITH CHECK (bucket_id = 'agent-stubs' AND auth.role() = 'authenticated');
