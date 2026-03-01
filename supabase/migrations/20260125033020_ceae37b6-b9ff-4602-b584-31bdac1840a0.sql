-- Create terminal_sessions table for realtime terminal communication
CREATE TABLE public.terminal_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  session_id TEXT NOT NULL,
  shell TEXT DEFAULT 'cmd',
  status TEXT DEFAULT 'active',
  created_at TIMESTAMPTZ DEFAULT now(),
  closed_at TIMESTAMPTZ
);

-- Enable Row Level Security
ALTER TABLE public.terminal_sessions ENABLE ROW LEVEL SECURITY;

-- RLS policies
CREATE POLICY "Users can manage terminal sessions for their org devices"
ON terminal_sessions FOR ALL TO authenticated
USING (user_belongs_to_org(auth.uid(), organization_id));

-- Enable realtime
ALTER PUBLICATION supabase_realtime ADD TABLE terminal_sessions;

-- Terminal messages table for bidirectional communication
CREATE TABLE public.terminal_messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id UUID NOT NULL REFERENCES terminal_sessions(id) ON DELETE CASCADE,
  direction TEXT NOT NULL CHECK (direction IN ('input', 'output')),
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);

-- Enable Row Level Security
ALTER TABLE public.terminal_messages ENABLE ROW LEVEL SECURITY;

-- RLS policies for messages
CREATE POLICY "Users can access terminal messages for their sessions"
ON terminal_messages FOR ALL TO authenticated
USING (
  EXISTS (
    SELECT 1 FROM terminal_sessions ts
    WHERE ts.id = terminal_messages.session_id
    AND user_belongs_to_org(auth.uid(), ts.organization_id)
  )
);

-- Enable realtime for messages
ALTER PUBLICATION supabase_realtime ADD TABLE terminal_messages;

-- Create index for faster lookups
CREATE INDEX idx_terminal_messages_session_id ON terminal_messages(session_id);
CREATE INDEX idx_terminal_sessions_device_id ON terminal_sessions(device_id);