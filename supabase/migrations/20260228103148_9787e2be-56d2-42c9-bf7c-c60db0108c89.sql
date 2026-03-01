
CREATE POLICY "Allow authenticated users to read file-transfers"
ON storage.objects
FOR SELECT
USING (bucket_id = 'file-transfers');

CREATE POLICY "Allow anon uploads to file-transfers"
ON storage.objects
FOR INSERT
WITH CHECK (bucket_id = 'file-transfers');
