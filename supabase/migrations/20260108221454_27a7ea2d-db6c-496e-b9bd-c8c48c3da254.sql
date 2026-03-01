-- Create storage bucket for file transfers
INSERT INTO storage.buckets (id, name, public)
VALUES ('file-transfers', 'file-transfers', false)
ON CONFLICT (id) DO NOTHING;

-- Create storage bucket for agent installers
INSERT INTO storage.buckets (id, name, public)
VALUES ('installers', 'installers', false)
ON CONFLICT (id) DO NOTHING;

-- RLS policies for file-transfers bucket
CREATE POLICY "Users can view their org files"
ON storage.objects
FOR SELECT
USING (
  bucket_id = 'file-transfers' AND
  (storage.foldername(name))[1] = 'transfers' AND
  EXISTS (
    SELECT 1 FROM public.profiles
    WHERE profiles.user_id = auth.uid()
    AND profiles.organization_id::text = (storage.foldername(name))[2]
  )
);

CREATE POLICY "Users can upload to their org"
ON storage.objects
FOR INSERT
WITH CHECK (
  bucket_id = 'file-transfers' AND
  (storage.foldername(name))[1] = 'transfers' AND
  EXISTS (
    SELECT 1 FROM public.profiles
    WHERE profiles.user_id = auth.uid()
    AND profiles.organization_id::text = (storage.foldername(name))[2]
  )
);

CREATE POLICY "Users can delete their org files"
ON storage.objects
FOR DELETE
USING (
  bucket_id = 'file-transfers' AND
  (storage.foldername(name))[1] = 'transfers' AND
  EXISTS (
    SELECT 1 FROM public.profiles
    WHERE profiles.user_id = auth.uid()
    AND profiles.organization_id::text = (storage.foldername(name))[2]
  )
);

-- RLS policies for installers bucket
CREATE POLICY "Users can view their org installers"
ON storage.objects
FOR SELECT
USING (
  bucket_id = 'installers' AND
  EXISTS (
    SELECT 1 FROM public.profiles
    WHERE profiles.user_id = auth.uid()
    AND profiles.organization_id::text = (storage.foldername(name))[2]
  )
);

CREATE POLICY "Users can download their org installers"
ON storage.objects
FOR SELECT
USING (
  bucket_id = 'installers' AND
  (storage.foldername(name))[1] = 'installers' AND
  EXISTS (
    SELECT 1 FROM public.profiles
    WHERE profiles.user_id = auth.uid()
    AND profiles.organization_id::text = (storage.foldername(name))[2]
  )
);