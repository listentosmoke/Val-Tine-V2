-- Drop existing policies on installers bucket that may be incorrectly configured
DROP POLICY IF EXISTS "Users can download their org installers" ON storage.objects;
DROP POLICY IF EXISTS "Users can view their org installers" ON storage.objects;
DROP POLICY IF EXISTS "Authenticated users can upload installers" ON storage.objects;
DROP POLICY IF EXISTS "Service role can upload installers" ON storage.objects;
DROP POLICY IF EXISTS "Service role can update installers" ON storage.objects;

-- Create correct SELECT policy for downloading installers
-- Path format: {org_id}/{installer_id}/{filename}
-- The org_id is at index [1] in the path
CREATE POLICY "Users can download installers from their org"
ON storage.objects FOR SELECT
USING (
  bucket_id = 'installers' 
  AND EXISTS (
    SELECT 1 FROM public.profiles
    WHERE profiles.user_id = auth.uid()
    AND profiles.organization_id::text = (storage.foldername(name))[1]
  )
);

-- Create INSERT policy for uploads (service role bypasses RLS, but good to have for completeness)
CREATE POLICY "Authenticated users can upload to their org folder"
ON storage.objects FOR INSERT
WITH CHECK (
  bucket_id = 'installers'
  AND EXISTS (
    SELECT 1 FROM public.profiles
    WHERE profiles.user_id = auth.uid()
    AND profiles.organization_id::text = (storage.foldername(name))[1]
  )
);

-- Create DELETE policy for cleanup
CREATE POLICY "Users can delete installers from their org"
ON storage.objects FOR DELETE
USING (
  bucket_id = 'installers' 
  AND EXISTS (
    SELECT 1 FROM public.profiles
    WHERE profiles.user_id = auth.uid()
    AND profiles.organization_id::text = (storage.foldername(name))[1]
  )
);