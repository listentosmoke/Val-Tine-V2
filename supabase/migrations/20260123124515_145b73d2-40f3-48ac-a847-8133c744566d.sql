-- Add organization-level communication key for agents
ALTER TABLE public.organizations 
ADD COLUMN IF NOT EXISTS communication_key TEXT;

-- Generate a default key for existing organizations
UPDATE public.organizations
SET communication_key = encode(gen_random_bytes(32), 'hex')
WHERE communication_key IS NULL;

-- Make it NOT NULL after setting defaults
ALTER TABLE public.organizations
ALTER COLUMN communication_key SET NOT NULL;

-- Add unique constraint
ALTER TABLE public.organizations
ADD CONSTRAINT organizations_communication_key_unique UNIQUE (communication_key);

-- Create a function to regenerate org communication key
CREATE OR REPLACE FUNCTION public.regenerate_org_communication_key(org_id uuid)
RETURNS text
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
  new_key text;
BEGIN
  new_key := encode(gen_random_bytes(32), 'hex');
  UPDATE public.organizations SET communication_key = new_key WHERE id = org_id;
  RETURN new_key;
END;
$$;