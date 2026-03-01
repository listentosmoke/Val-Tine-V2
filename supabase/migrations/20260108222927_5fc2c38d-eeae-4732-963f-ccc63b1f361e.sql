-- Create function to handle new user registration automatically
CREATE OR REPLACE FUNCTION public.handle_new_user()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
  org_id uuid;
  org_name text;
  org_slug text;
  user_full_name text;
BEGIN
  -- Get organization info from metadata
  org_name := NEW.raw_user_meta_data->>'organization_name';
  user_full_name := NEW.raw_user_meta_data->>'full_name';
  
  IF org_name IS NOT NULL THEN
    -- Create slug from organization name
    org_slug := lower(regexp_replace(org_name, '[^a-zA-Z0-9]', '-', 'g'));
    org_slug := regexp_replace(org_slug, '-+', '-', 'g');
    org_slug := trim(both '-' from org_slug);
    
    -- Create organization
    INSERT INTO public.organizations (name, slug)
    VALUES (org_name, org_slug)
    RETURNING id INTO org_id;
    
    -- Create profile
    INSERT INTO public.profiles (user_id, email, full_name, organization_id)
    VALUES (NEW.id, NEW.email, user_full_name, org_id);
    
    -- Assign admin role (first user in org is admin)
    INSERT INTO public.user_roles (user_id, role)
    VALUES (NEW.id, 'admin');
  END IF;
  
  RETURN NEW;
END;
$$;

-- Create trigger on auth.users table
CREATE TRIGGER on_auth_user_created
  AFTER INSERT ON auth.users
  FOR EACH ROW EXECUTE FUNCTION public.handle_new_user();