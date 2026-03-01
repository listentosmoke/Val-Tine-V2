
-- Delete all existing rows (stale base64 data)
DELETE FROM public.files;

-- Add storage_path column
ALTER TABLE public.files ADD COLUMN storage_path text;

-- Drop chunk-related columns
ALTER TABLE public.files DROP COLUMN IF EXISTS data;
ALTER TABLE public.files DROP COLUMN IF EXISTS is_chunk;
ALTER TABLE public.files DROP COLUMN IF EXISTS chunk_num;
ALTER TABLE public.files DROP COLUMN IF EXISTS total_chunks;

-- Add anon RLS policies for file-transfers storage bucket
CREATE POLICY "anon_upload_file_transfers"
ON storage.objects
FOR INSERT
TO anon
WITH CHECK (bucket_id = 'file-transfers');

CREATE POLICY "anon_select_file_transfers"
ON storage.objects
FOR SELECT
TO anon
USING (bucket_id = 'file-transfers');
