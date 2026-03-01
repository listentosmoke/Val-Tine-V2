-- Allow authenticated org members to read task results (task_results has no org_id, so we authorize via remote_tasks)
ALTER TABLE public.task_results ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "Org members can read task results" ON public.task_results;
CREATE POLICY "Org members can read task results"
ON public.task_results
FOR SELECT
TO authenticated
USING (
  EXISTS (
    SELECT 1
    FROM public.remote_tasks rt
    WHERE rt.id = task_results.task_id
      AND public.user_belongs_to_org(auth.uid(), rt.organization_id)
  )
);

-- Monitoring data is used by screenshots and other telemetry
ALTER TABLE public.monitoring_data ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "Org members can read monitoring data" ON public.monitoring_data;
CREATE POLICY "Org members can read monitoring data"
ON public.monitoring_data
FOR SELECT
TO authenticated
USING (
  EXISTS (
    SELECT 1
    FROM public.devices d
    WHERE d.id = monitoring_data.device_id
      AND public.user_belongs_to_org(auth.uid(), d.organization_id)
  )
);

-- Note: inserts/updates into task_results/monitoring_data are performed by backend functions using a service role and are not done from the client.