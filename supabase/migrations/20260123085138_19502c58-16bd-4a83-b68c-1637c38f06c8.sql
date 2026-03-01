-- Create bucket for pre-compiled agent stubs
INSERT INTO storage.buckets (id, name, public)
VALUES ('agent-stubs', 'agent-stubs', true)
ON CONFLICT (id) DO NOTHING;

-- Public read access for agent stubs
CREATE POLICY "Public read access for agent stubs"
ON storage.objects FOR SELECT
USING (bucket_id = 'agent-stubs');

-- Authenticated users can upload stubs
CREATE POLICY "Authenticated users can upload stubs"
ON storage.objects FOR INSERT
WITH CHECK (bucket_id = 'agent-stubs' AND auth.role() = 'authenticated');