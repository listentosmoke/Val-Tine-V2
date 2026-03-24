import { useAuth } from "@/hooks/useAuth";
import { useNavigate, Link, useLocation } from "react-router-dom";
import { useClients } from "@/hooks/useClients";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Shield,
  LayoutDashboard,
  LogOut,
  Send,
  Search,
  PanelLeftClose,
  PanelLeftOpen,
  ChevronRight,
} from "lucide-react";
import { useState, useMemo, useCallback } from "react";
import { dispatchBatchCommand } from "@/lib/commands";
import { toast } from "sonner";
import { Textarea } from "@/components/ui/textarea";

interface AppLayoutProps {
  children: React.ReactNode;
}

const navItems = [
  { label: "Dashboard", icon: LayoutDashboard, path: "/" },
];

function isOnline(lastSeen: string | null) {
  if (!lastSeen) return false;
  return Date.now() - new Date(lastSeen).getTime() < 120 * 1000;
}

function isIdle(lastSeen: string | null) {
  if (!lastSeen) return false;
  const diff = Date.now() - new Date(lastSeen).getTime();
  return diff >= 120 * 1000 && diff < 5 * 60 * 1000;
}

const AppLayout = ({ children }: AppLayoutProps) => {
  const { user, signOut } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const { data: clients } = useClients();
  const [collapsed, setCollapsed] = useState(false);
  const [broadcastOpen, setBroadcastOpen] = useState(false);
  const [broadcastCmd, setBroadcastCmd] = useState("");
  const [broadcasting, setBroadcasting] = useState(false);
  const [globalSearch, setGlobalSearch] = useState("");

  const stats = useMemo(() => {
    if (!clients) return { online: 0, offline: 0, alerts: 0, total: 0 };
    const online = clients.filter((c) => isOnline(c.last_seen)).length;
    const idle = clients.filter((c) => isIdle(c.last_seen)).length;
    return {
      online,
      offline: clients.length - online - idle,
      alerts: idle,
      total: clients.length,
    };
  }, [clients]);

  const breadcrumb = useMemo(() => {
    const parts = location.pathname.split("/").filter(Boolean);
    if (parts.length === 0) return ["Dashboard"];
    if (parts[0] === "client" && parts[1]) {
      const client = clients?.find((c) => c.machine_id === parts[1]);
      return ["Dashboard", client?.machine_name || parts[1].slice(0, 8)];
    }
    return [parts[0]];
  }, [location.pathname, clients]);

  const handleBroadcast = useCallback(async () => {
    if (!broadcastCmd.trim() || !clients) return;
    setBroadcasting(true);
    const onlineIds = clients
      .filter((c) => isOnline(c.last_seen))
      .map((c) => c.machine_id);

    if (onlineIds.length === 0) {
      toast.error("No online machines to broadcast to");
      setBroadcasting(false);
      return;
    }

    const ids = await dispatchBatchCommand(onlineIds, "shell", {
      cmd: broadcastCmd.trim(),
    });
    toast.success(`Command broadcast to ${ids.length} machines`);
    setBroadcastCmd("");
    setBroadcasting(false);
    setBroadcastOpen(false);
  }, [broadcastCmd, clients]);

  const handleSignOut = async () => {
    await signOut();
    navigate("/login");
  };

  const filteredResults = useMemo(() => {
    if (!globalSearch.trim() || !clients) return [];
    const q = globalSearch.toLowerCase();
    return clients
      .filter(
        (c) =>
          c.machine_name?.toLowerCase().includes(q) ||
          c.username?.toLowerCase().includes(q) ||
          c.ip?.toLowerCase().includes(q) ||
          c.os?.toLowerCase().includes(q) ||
          c.machine_id.toLowerCase().includes(q)
      )
      .slice(0, 8);
  }, [globalSearch, clients]);

  return (
    <div className="min-h-screen bg-background flex flex-col md:flex-row">
      {/* Sidebar */}
<aside
  className={`
    ${collapsed ? "md:w-[60px]" : "md:w-[260px]"}
    w-full md:w-auto
    border-b md:border-b-0 md:border-r border-border/40
    flex flex-col
    md:h-auto
    transition-all duration-200
  `}
        style={{ background: "#0f0f12" }}
      >
        {/* Header */}
        <div className="p-3 border-b border-white/5 flex items-center gap-2.5 h-[60px]">
          <div className="w-8 h-8 rounded-lg bg-primary/15 flex items-center justify-center flex-shrink-0">
            <Shield className="w-4 h-4 text-primary" />
          </div>
          {!collapsed && (
            <div className="flex items-center gap-2 min-w-0">
              <span className="font-semibold text-sm text-foreground truncate">
                Val&Tine V2
              </span>
              <Badge
                variant="secondary"
                className="text-[10px] px-1.5 py-0 h-4 bg-primary/10 text-primary border-0 flex-shrink-0"
              >
                v1.6
              </Badge>
            </div>
          )}
        </div>

        {/* Quick Stats */}
        {!collapsed && (
          <div className="px-3 py-2.5 border-b border-white/5 space-y-1">
            <p className="text-[10px] uppercase tracking-wider text-muted-foreground/50 font-medium mb-1.5">
              Fleet Status
            </p>
            <div className="flex items-center gap-4 text-xs">
              <span className="flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-emerald-500 shadow-glow-green" />
                <span className="text-emerald-400 font-medium">
                  {stats.online}
                </span>
              </span>
              <span className="flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-gray-500" />
                <span className="text-muted-foreground">{stats.offline}</span>
              </span>
              {stats.alerts > 0 && (
                <span className="flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-amber-500 animate-pulse" />
                  <span className="text-amber-400">{stats.alerts}</span>
                </span>
              )}
            </div>
          </div>
        )}

        {/* Navigation */}
        <nav className="flex-1 p-2 space-y-0.5">
          {navItems.map((item) => {
            const isActive =
              item.path === "/"
                ? location.pathname === "/" ||
                  location.pathname.startsWith("/client")
                : location.pathname.startsWith(item.path);
            return (
              <Link
                key={item.path}
                to={item.path}
                className={`flex items-center gap-2.5 px-3 py-2 rounded-md text-sm transition-colors ${
                  isActive
                    ? "bg-primary/10 text-primary"
                    : "text-muted-foreground hover:bg-white/5 hover:text-foreground"
                } ${collapsed ? "justify-center px-2" : ""}`}
                title={collapsed ? item.label : undefined}
              >
                <item.icon className="w-4 h-4 flex-shrink-0" />
                {!collapsed && <span>{item.label}</span>}
              </Link>
            );
          })}
        </nav>

        {/* Bottom Actions */}
        <div className="p-2 border-t border-white/5 space-y-0.5">
          <Dialog open={broadcastOpen} onOpenChange={setBroadcastOpen}>
            <DialogTrigger asChild>
              <Button
                variant="ghost"
                size="sm"
                className={`w-full gap-2 text-muted-foreground hover:text-foreground hover:bg-white/5 ${
                  collapsed ? "justify-center px-2" : "justify-start"
                }`}
                title={collapsed ? "Broadcast Command" : undefined}
              >
                <Send className="w-4 h-4 flex-shrink-0" />
                {!collapsed && (
                  <span className="text-xs">Broadcast Command</span>
                )}
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>Broadcast Command</DialogTitle>
              </DialogHeader>
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  Send a shell command to all{" "}
                  <span className="text-emerald-400 font-medium">
                    {stats.online}
                  </span>{" "}
                  online machines.
                </p>
                <Textarea
                  placeholder="Enter command..."
                  value={broadcastCmd}
                  onChange={(e) => setBroadcastCmd(e.target.value)}
                  className="font-mono text-sm min-h-[80px]"
                />
                <Button
                  onClick={handleBroadcast}
                  disabled={broadcasting || !broadcastCmd.trim()}
                  className="w-full"
                >
                  {broadcasting
                    ? "Sending..."
                    : `Send to ${stats.online} machines`}
                </Button>
              </div>
            </DialogContent>
          </Dialog>

          <div
            className={`pt-1 border-t border-white/5 ${
              collapsed ? "text-center" : ""
            }`}
          >
            {!collapsed && (
              <div className="px-2 py-1 text-[11px] text-muted-foreground/50 truncate">
                {user?.email}
              </div>
            )}
            <Button
              variant="ghost"
              size="sm"
              className={`w-full gap-2 text-muted-foreground hover:text-red-400 hover:bg-red-500/5 ${
                collapsed ? "justify-center px-2" : "justify-start"
              }`}
              onClick={handleSignOut}
              title={collapsed ? "Sign Out" : undefined}
            >
              <LogOut className="w-4 h-4 flex-shrink-0" />
              {!collapsed && <span className="text-xs">Sign Out</span>}
            </Button>
          </div>

          <Button
            variant="ghost"
            size="sm"
            className={`w-full gap-2 text-muted-foreground/40 hover:text-muted-foreground hover:bg-white/5 ${
              collapsed ? "justify-center px-2" : "justify-start"
            }`}
            onClick={() => setCollapsed(!collapsed)}
          >
            {collapsed ? (
              <PanelLeftOpen className="w-4 h-4" />
            ) : (
              <>
                <PanelLeftClose className="w-4 h-4" />
                <span className="text-[10px]">Collapse</span>
              </>
            )}
          </Button>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-auto flex flex-col min-w-0">
        <header className="h-[60px] border-b border-border/40 flex items-center justify-between px-3 md:px-5 bg-card/20 flex-shrink-0 gap-2">
          <div className="flex items-center gap-1.5 text-sm min-w-0">
            {breadcrumb.map((crumb, i) => (
              <span key={i} className="flex items-center gap-1.5 min-w-0">
                {i > 0 && (
                  <ChevronRight className="w-3.5 h-3.5 text-muted-foreground/40 flex-shrink-0" />
                )}
                <span
                  className={`truncate ${
                    i === breadcrumb.length - 1
                      ? "text-foreground font-medium"
                      : "text-muted-foreground cursor-pointer hover:text-foreground"
                  }`}
                  onClick={
                    i === 0 && breadcrumb.length > 1
                      ? () => navigate("/")
                      : undefined
                  }
                >
                  {crumb}
                </span>
              </span>
            ))}
          </div>

          <div className="relative w-full md:max-w-sm md:mx-4">
            <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground/40" />
            <Input
              placeholder="Search machines..."
              value={globalSearch}
              onChange={(e) => setGlobalSearch(e.target.value)}
              className="pl-9 h-8 text-sm bg-muted/20 border-border/30 focus:bg-muted/40"
            />
            {globalSearch && filteredResults.length > 0 && (
              <div className="absolute top-full left-0 right-0 mt-1 bg-card border border-border rounded-lg shadow-xl z-50 py-1 max-h-64 overflow-auto">
                {filteredResults.map((c) => (
                  <div
                    key={c.id}
                    className="px-3 py-2 hover:bg-muted/50 cursor-pointer flex items-center gap-3 text-sm"
                    onClick={() => {
                      navigate(`/client/${c.machine_id}`);
                      setGlobalSearch("");
                    }}
                  >
                    <span
                      className={`w-2 h-2 rounded-full flex-shrink-0 ${
                        isOnline(c.last_seen)
                          ? "bg-emerald-500"
                          : isIdle(c.last_seen)
                          ? "bg-amber-500"
                          : "bg-gray-500"
                      }`}
                    />
                    <span className="font-medium truncate">
                      {c.machine_name || "Unknown"}
                    </span>
                    <span className="text-muted-foreground text-xs truncate">
                      {c.username}
                    </span>
                    <span className="text-muted-foreground/50 text-xs font-mono ml-auto flex-shrink-0">
                      {c.ip}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="flex items-center gap-2 text-xs text-muted-foreground/50 flex-shrink-0">
            <span className="font-mono">{stats.total} nodes</span>
          </div>
        </header>

        <div className="flex-1 overflow-auto p-5">{children}</div>
      </main>
    </div>
  );
};

export default AppLayout;
