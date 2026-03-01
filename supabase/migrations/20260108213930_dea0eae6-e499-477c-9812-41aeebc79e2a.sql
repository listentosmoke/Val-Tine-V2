
-- ============================================
-- NodePulse RMM Platform - Complete Database Schema
-- ============================================

-- 1. Create custom types/enums
CREATE TYPE public.app_role AS ENUM ('admin', 'technician', 'viewer');
CREATE TYPE public.device_status AS ENUM ('online', 'offline', 'pending', 'error');
CREATE TYPE public.task_status AS ENUM ('pending', 'queued', 'running', 'completed', 'failed', 'cancelled');
CREATE TYPE public.task_type AS ENUM ('terminal', 'file_upload', 'file_download', 'screenshot', 'system_info', 'network_scan', 'custom');
CREATE TYPE public.transfer_direction AS ENUM ('upload', 'download');
CREATE TYPE public.transfer_status AS ENUM ('pending', 'in_progress', 'completed', 'failed');
CREATE TYPE public.activity_event_type AS ENUM ('device_registered', 'device_online', 'device_offline', 'task_created', 'task_completed', 'task_failed', 'file_uploaded', 'file_downloaded', 'user_login', 'user_logout', 'settings_changed', 'installer_built');

-- 2. Organizations table
CREATE TABLE public.organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3. User profiles table
CREATE TABLE public.profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE NOT NULL UNIQUE,
    organization_id UUID REFERENCES public.organizations(id) ON DELETE CASCADE NOT NULL,
    email TEXT NOT NULL,
    full_name TEXT,
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4. User roles table (separate for security)
CREATE TABLE public.user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES auth.users(id) ON DELETE CASCADE NOT NULL,
    role public.app_role NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, role)
);

-- 5. Agent profiles (connection configurations)
CREATE TABLE public.agent_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES public.organizations(id) ON DELETE CASCADE NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    communication_key TEXT NOT NULL,
    sync_interval_seconds INTEGER NOT NULL DEFAULT 30,
    jitter_percent INTEGER NOT NULL DEFAULT 10,
    max_retries INTEGER NOT NULL DEFAULT 3,
    enabled_modules TEXT[] DEFAULT ARRAY['terminal', 'files', 'screenshot', 'system_info'],
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 6. Devices table
CREATE TABLE public.devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES public.organizations(id) ON DELETE CASCADE NOT NULL,
    agent_profile_id UUID REFERENCES public.agent_profiles(id) ON DELETE SET NULL,
    device_id TEXT NOT NULL,
    hostname TEXT NOT NULL,
    os_type TEXT NOT NULL,
    os_version TEXT,
    architecture TEXT,
    internal_ip TEXT,
    external_ip TEXT,
    mac_address TEXT,
    status public.device_status NOT NULL DEFAULT 'pending',
    last_seen_at TIMESTAMPTZ,
    first_seen_at TIMESTAMPTZ DEFAULT now(),
    system_info JSONB DEFAULT '{}',
    geo_location JSONB DEFAULT '{}',
    tags TEXT[] DEFAULT '{}',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(organization_id, device_id)
);

-- 7. Remote tasks table
CREATE TABLE public.remote_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES public.organizations(id) ON DELETE CASCADE NOT NULL,
    device_id UUID REFERENCES public.devices(id) ON DELETE CASCADE NOT NULL,
    created_by UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    task_type public.task_type NOT NULL,
    status public.task_status NOT NULL DEFAULT 'pending',
    command TEXT,
    payload JSONB DEFAULT '{}',
    priority INTEGER NOT NULL DEFAULT 5,
    timeout_seconds INTEGER DEFAULT 300,
    scheduled_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 8. Task results table
CREATE TABLE public.task_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID REFERENCES public.remote_tasks(id) ON DELETE CASCADE NOT NULL,
    exit_code INTEGER,
    stdout TEXT,
    stderr TEXT,
    output_data JSONB DEFAULT '{}',
    execution_time_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 9. File transfers table
CREATE TABLE public.file_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES public.organizations(id) ON DELETE CASCADE NOT NULL,
    device_id UUID REFERENCES public.devices(id) ON DELETE CASCADE NOT NULL,
    initiated_by UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    direction public.transfer_direction NOT NULL,
    status public.transfer_status NOT NULL DEFAULT 'pending',
    local_path TEXT NOT NULL,
    remote_path TEXT NOT NULL,
    file_name TEXT NOT NULL,
    file_size BIGINT,
    mime_type TEXT,
    storage_path TEXT,
    checksum TEXT,
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 10. Agent installers table
CREATE TABLE public.agent_installers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES public.organizations(id) ON DELETE CASCADE NOT NULL,
    agent_profile_id UUID REFERENCES public.agent_profiles(id) ON DELETE CASCADE NOT NULL,
    built_by UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    target_os TEXT NOT NULL,
    target_arch TEXT NOT NULL,
    build_config JSONB NOT NULL DEFAULT '{}',
    file_size BIGINT,
    storage_path TEXT,
    checksum TEXT,
    download_count INTEGER DEFAULT 0,
    build_status TEXT DEFAULT 'pending',
    build_log TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ
);

-- 11. Monitoring data table (screenshots, metrics)
CREATE TABLE public.monitoring_data (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID REFERENCES public.devices(id) ON DELETE CASCADE NOT NULL,
    data_type TEXT NOT NULL,
    data JSONB NOT NULL DEFAULT '{}',
    storage_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 12. Activity log table
CREATE TABLE public.activity_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES public.organizations(id) ON DELETE CASCADE NOT NULL,
    user_id UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    device_id UUID REFERENCES public.devices(id) ON DELETE SET NULL,
    event_type public.activity_event_type NOT NULL,
    resource_type TEXT,
    resource_id UUID,
    details JSONB DEFAULT '{}',
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 13. Task templates table
CREATE TABLE public.task_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES public.organizations(id) ON DELETE CASCADE NOT NULL,
    created_by UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    description TEXT,
    task_type public.task_type NOT NULL,
    command TEXT,
    payload JSONB DEFAULT '{}',
    is_global BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================
-- Indexes for performance
-- ============================================
CREATE INDEX idx_profiles_user_id ON public.profiles(user_id);
CREATE INDEX idx_profiles_organization_id ON public.profiles(organization_id);
CREATE INDEX idx_user_roles_user_id ON public.user_roles(user_id);
CREATE INDEX idx_devices_organization_id ON public.devices(organization_id);
CREATE INDEX idx_devices_status ON public.devices(status);
CREATE INDEX idx_devices_last_seen ON public.devices(last_seen_at DESC);
CREATE INDEX idx_devices_hostname ON public.devices(hostname);
CREATE INDEX idx_remote_tasks_device_id ON public.remote_tasks(device_id);
CREATE INDEX idx_remote_tasks_status ON public.remote_tasks(status);
CREATE INDEX idx_remote_tasks_created_at ON public.remote_tasks(created_at DESC);
CREATE INDEX idx_task_results_task_id ON public.task_results(task_id);
CREATE INDEX idx_file_transfers_device_id ON public.file_transfers(device_id);
CREATE INDEX idx_file_transfers_status ON public.file_transfers(status);
CREATE INDEX idx_activity_log_organization_id ON public.activity_log(organization_id);
CREATE INDEX idx_activity_log_created_at ON public.activity_log(created_at DESC);
CREATE INDEX idx_monitoring_data_device_id ON public.monitoring_data(device_id);
CREATE INDEX idx_monitoring_data_type ON public.monitoring_data(data_type);

-- ============================================
-- Enable Row Level Security
-- ============================================
ALTER TABLE public.organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.agent_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.devices ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.remote_tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.task_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.file_transfers ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.agent_installers ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.monitoring_data ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.activity_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.task_templates ENABLE ROW LEVEL SECURITY;

-- ============================================
-- Security Definer Functions
-- ============================================

-- Function to check if user has a specific role
CREATE OR REPLACE FUNCTION public.has_role(_user_id UUID, _role public.app_role)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT EXISTS (
        SELECT 1
        FROM public.user_roles
        WHERE user_id = _user_id
          AND role = _role
    )
$$;

-- Function to get user's organization ID
CREATE OR REPLACE FUNCTION public.get_user_organization_id(_user_id UUID)
RETURNS UUID
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT organization_id
    FROM public.profiles
    WHERE user_id = _user_id
    LIMIT 1
$$;

-- Function to check if user belongs to organization
CREATE OR REPLACE FUNCTION public.user_belongs_to_org(_user_id UUID, _org_id UUID)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT EXISTS (
        SELECT 1
        FROM public.profiles
        WHERE user_id = _user_id
          AND organization_id = _org_id
    )
$$;

-- ============================================
-- RLS Policies
-- ============================================

-- Organizations policies
CREATE POLICY "Users can view their organization"
ON public.organizations FOR SELECT
TO authenticated
USING (public.user_belongs_to_org(auth.uid(), id));

CREATE POLICY "Admins can update their organization"
ON public.organizations FOR UPDATE
TO authenticated
USING (public.user_belongs_to_org(auth.uid(), id) AND public.has_role(auth.uid(), 'admin'));

-- Profiles policies
CREATE POLICY "Users can view profiles in their organization"
ON public.profiles FOR SELECT
TO authenticated
USING (public.user_belongs_to_org(auth.uid(), organization_id));

CREATE POLICY "Users can update their own profile"
ON public.profiles FOR UPDATE
TO authenticated
USING (user_id = auth.uid());

CREATE POLICY "Users can insert their own profile"
ON public.profiles FOR INSERT
TO authenticated
WITH CHECK (user_id = auth.uid());

-- User roles policies
CREATE POLICY "Users can view roles in their organization"
ON public.user_roles FOR SELECT
TO authenticated
USING (
    EXISTS (
        SELECT 1 FROM public.profiles p
        WHERE p.user_id = public.user_roles.user_id
        AND public.user_belongs_to_org(auth.uid(), p.organization_id)
    )
);

CREATE POLICY "Admins can manage roles"
ON public.user_roles FOR ALL
TO authenticated
USING (
    EXISTS (
        SELECT 1 FROM public.profiles p
        WHERE p.user_id = public.user_roles.user_id
        AND public.user_belongs_to_org(auth.uid(), p.organization_id)
        AND public.has_role(auth.uid(), 'admin')
    )
);

-- Agent profiles policies
CREATE POLICY "Users can view agent profiles in their organization"
ON public.agent_profiles FOR SELECT
TO authenticated
USING (public.user_belongs_to_org(auth.uid(), organization_id));

CREATE POLICY "Admins and technicians can manage agent profiles"
ON public.agent_profiles FOR ALL
TO authenticated
USING (
    public.user_belongs_to_org(auth.uid(), organization_id)
    AND (public.has_role(auth.uid(), 'admin') OR public.has_role(auth.uid(), 'technician'))
);

-- Devices policies
CREATE POLICY "Users can view devices in their organization"
ON public.devices FOR SELECT
TO authenticated
USING (public.user_belongs_to_org(auth.uid(), organization_id));

CREATE POLICY "Admins and technicians can manage devices"
ON public.devices FOR ALL
TO authenticated
USING (
    public.user_belongs_to_org(auth.uid(), organization_id)
    AND (public.has_role(auth.uid(), 'admin') OR public.has_role(auth.uid(), 'technician'))
);

-- Remote tasks policies
CREATE POLICY "Users can view tasks in their organization"
ON public.remote_tasks FOR SELECT
TO authenticated
USING (public.user_belongs_to_org(auth.uid(), organization_id));

CREATE POLICY "Admins and technicians can create tasks"
ON public.remote_tasks FOR INSERT
TO authenticated
WITH CHECK (
    public.user_belongs_to_org(auth.uid(), organization_id)
    AND (public.has_role(auth.uid(), 'admin') OR public.has_role(auth.uid(), 'technician'))
);

CREATE POLICY "Admins and technicians can update tasks"
ON public.remote_tasks FOR UPDATE
TO authenticated
USING (
    public.user_belongs_to_org(auth.uid(), organization_id)
    AND (public.has_role(auth.uid(), 'admin') OR public.has_role(auth.uid(), 'technician'))
);

-- Task results policies
CREATE POLICY "Users can view task results in their organization"
ON public.task_results FOR SELECT
TO authenticated
USING (
    EXISTS (
        SELECT 1 FROM public.remote_tasks t
        WHERE t.id = public.task_results.task_id
        AND public.user_belongs_to_org(auth.uid(), t.organization_id)
    )
);

-- File transfers policies
CREATE POLICY "Users can view file transfers in their organization"
ON public.file_transfers FOR SELECT
TO authenticated
USING (public.user_belongs_to_org(auth.uid(), organization_id));

CREATE POLICY "Admins and technicians can manage file transfers"
ON public.file_transfers FOR ALL
TO authenticated
USING (
    public.user_belongs_to_org(auth.uid(), organization_id)
    AND (public.has_role(auth.uid(), 'admin') OR public.has_role(auth.uid(), 'technician'))
);

-- Agent installers policies
CREATE POLICY "Users can view installers in their organization"
ON public.agent_installers FOR SELECT
TO authenticated
USING (public.user_belongs_to_org(auth.uid(), organization_id));

CREATE POLICY "Admins and technicians can manage installers"
ON public.agent_installers FOR ALL
TO authenticated
USING (
    public.user_belongs_to_org(auth.uid(), organization_id)
    AND (public.has_role(auth.uid(), 'admin') OR public.has_role(auth.uid(), 'technician'))
);

-- Monitoring data policies
CREATE POLICY "Users can view monitoring data in their organization"
ON public.monitoring_data FOR SELECT
TO authenticated
USING (
    EXISTS (
        SELECT 1 FROM public.devices d
        WHERE d.id = public.monitoring_data.device_id
        AND public.user_belongs_to_org(auth.uid(), d.organization_id)
    )
);

-- Activity log policies
CREATE POLICY "Users can view activity in their organization"
ON public.activity_log FOR SELECT
TO authenticated
USING (public.user_belongs_to_org(auth.uid(), organization_id));

CREATE POLICY "System can insert activity logs"
ON public.activity_log FOR INSERT
TO authenticated
WITH CHECK (public.user_belongs_to_org(auth.uid(), organization_id));

-- Task templates policies
CREATE POLICY "Users can view templates in their organization or global"
ON public.task_templates FOR SELECT
TO authenticated
USING (
    is_global = true
    OR public.user_belongs_to_org(auth.uid(), organization_id)
);

CREATE POLICY "Admins and technicians can manage templates"
ON public.task_templates FOR ALL
TO authenticated
USING (
    public.user_belongs_to_org(auth.uid(), organization_id)
    AND (public.has_role(auth.uid(), 'admin') OR public.has_role(auth.uid(), 'technician'))
);

-- ============================================
-- Triggers for updated_at
-- ============================================
CREATE OR REPLACE FUNCTION public.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_organizations_updated_at
    BEFORE UPDATE ON public.organizations
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

CREATE TRIGGER update_profiles_updated_at
    BEFORE UPDATE ON public.profiles
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

CREATE TRIGGER update_agent_profiles_updated_at
    BEFORE UPDATE ON public.agent_profiles
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

CREATE TRIGGER update_devices_updated_at
    BEFORE UPDATE ON public.devices
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

CREATE TRIGGER update_remote_tasks_updated_at
    BEFORE UPDATE ON public.remote_tasks
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

CREATE TRIGGER update_file_transfers_updated_at
    BEFORE UPDATE ON public.file_transfers
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

CREATE TRIGGER update_task_templates_updated_at
    BEFORE UPDATE ON public.task_templates
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

-- ============================================
-- Enable Realtime for key tables
-- ============================================
ALTER PUBLICATION supabase_realtime ADD TABLE public.devices;
ALTER PUBLICATION supabase_realtime ADD TABLE public.remote_tasks;
ALTER PUBLICATION supabase_realtime ADD TABLE public.task_results;
ALTER PUBLICATION supabase_realtime ADD TABLE public.activity_log;

-- Set REPLICA IDENTITY FULL for realtime tables
ALTER TABLE public.devices REPLICA IDENTITY FULL;
ALTER TABLE public.remote_tasks REPLICA IDENTITY FULL;
ALTER TABLE public.task_results REPLICA IDENTITY FULL;
ALTER TABLE public.activity_log REPLICA IDENTITY FULL;

-- ============================================
-- Storage buckets
-- ============================================
INSERT INTO storage.buckets (id, name, public)
VALUES ('agent-installers', 'agent-installers', false);

INSERT INTO storage.buckets (id, name, public)
VALUES ('file-transfers', 'file-transfers', false);

INSERT INTO storage.buckets (id, name, public)
VALUES ('screenshots', 'screenshots', false);

-- Storage policies for agent-installers bucket
CREATE POLICY "Users can view installers from their org"
ON storage.objects FOR SELECT
TO authenticated
USING (
    bucket_id = 'agent-installers'
    AND EXISTS (
        SELECT 1 FROM public.agent_installers ai
        WHERE ai.storage_path = name
        AND public.user_belongs_to_org(auth.uid(), ai.organization_id)
    )
);

-- Storage policies for file-transfers bucket
CREATE POLICY "Users can access file transfers from their org"
ON storage.objects FOR SELECT
TO authenticated
USING (
    bucket_id = 'file-transfers'
    AND EXISTS (
        SELECT 1 FROM public.file_transfers ft
        WHERE ft.storage_path = name
        AND public.user_belongs_to_org(auth.uid(), ft.organization_id)
    )
);

-- Storage policies for screenshots bucket
CREATE POLICY "Users can view screenshots from their org"
ON storage.objects FOR SELECT
TO authenticated
USING (
    bucket_id = 'screenshots'
    AND EXISTS (
        SELECT 1 FROM public.monitoring_data md
        JOIN public.devices d ON d.id = md.device_id
        WHERE md.storage_path = name
        AND public.user_belongs_to_org(auth.uid(), d.organization_id)
    )
);
