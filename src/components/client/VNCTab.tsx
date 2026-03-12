import { useState, useRef, useEffect, useCallback } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { MonitorPlay, Power, PowerOff, MousePointer, Eye, Wifi, WifiOff, Maximize2 } from "lucide-react";
import { toast } from "sonner";
import { dispatchCommand, waitForResult } from "@/lib/commands";

interface VNCTabProps {
  machineId: string;
  machineName: string;
}

const VNCTab = ({ machineId, machineName }: VNCTabProps) => {
  const [vncStatus, setVncStatus] = useState<"idle" | "starting" | "connected" | "disconnected">("idle");
  const [agentConnected, setAgentConnected] = useState(false);
  const [tunnelUrl, setTunnelUrl] = useState<string | null>(null);
  const [interactive, setInteractive] = useState(true);
  const [quality, setQuality] = useState("50");
  const [fps, setFps] = useState("2");
  const [screenshotSize, setScreenshotSize] = useState(0);
  const [frameCount, setFrameCount] = useState(0);

  const canvasRef = useRef<HTMLCanvasElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const lastMoveRef = useRef(0);
  const screenDimsRef = useRef({ width: 1920, height: 1080 });

  // Connect WebSocket to backend
  const connectWs = useCallback(() => {
    const wsProto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${wsProto}//${window.location.host}/ws`);

    ws.onopen = () => {
      console.log("[vnc] WebSocket connected");
      setVncStatus("connected");
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);

        if (msg.type === "screenshot" && msg.data) {
          const img = new Image();
          img.onload = () => {
            const canvas = canvasRef.current;
            if (!canvas) return;
            const ctx = canvas.getContext("2d");
            if (!ctx) return;

            // Update canvas size to match image aspect ratio
            const container = canvas.parentElement;
            if (container) {
              const availW = container.clientWidth;
              const scale = availW / img.width;
              canvas.width = availW;
              canvas.height = img.height * scale;
              canvas.style.width = `${availW}px`;
              canvas.style.height = `${img.height * scale}px`;
              screenDimsRef.current = { width: img.width, height: img.height };
            }

            ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
            setScreenshotSize(msg.size || 0);
            setFrameCount((prev) => prev + 1);
          };
          img.src = `data:image/jpeg;base64,${msg.data}`;
        } else if (msg.type === "status") {
          setAgentConnected(msg.agentConnected ?? false);
          if (msg.tunnelUrl) setTunnelUrl(msg.tunnelUrl);
        }
      } catch {}
    };

    ws.onclose = () => {
      console.log("[vnc] WebSocket disconnected");
      if (vncStatus === "connected") {
        setVncStatus("disconnected");
        // Reconnect after 2s
        setTimeout(connectWs, 2000);
      }
    };

    ws.onerror = () => {
      ws.close();
    };

    wsRef.current = ws;
  }, [vncStatus]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, []);

  const sendInput = useCallback((data: Record<string, unknown>) => {
    if (!interactive || !wsRef.current || wsRef.current.readyState !== 1) return;
    wsRef.current.send(JSON.stringify({ type: "input", ...data }));
  }, [interactive]);

  // Map canvas coordinates to screen coordinates
  const screenCoords = useCallback((e: React.MouseEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current;
    if (!canvas) return { x: 0, y: 0 };
    const rect = canvas.getBoundingClientRect();
    const dims = screenDimsRef.current;
    const scaleX = dims.width / canvas.width;
    const scaleY = dims.height / canvas.height;
    return {
      x: Math.round((e.clientX - rect.left) * scaleX),
      y: Math.round((e.clientY - rect.top) * scaleY),
    };
  }, []);

  // Mouse handlers
  const handleMouseMove = useCallback((e: React.MouseEvent<HTMLCanvasElement>) => {
    const now = Date.now();
    if (now - lastMoveRef.current < 30) return; // Throttle to ~33fps
    lastMoveRef.current = now;
    const { x, y } = screenCoords(e);
    sendInput({ action: "mousemove", x, y });
  }, [screenCoords, sendInput]);

  const handleMouseDown = useCallback((e: React.MouseEvent<HTMLCanvasElement>) => {
    const { x, y } = screenCoords(e);
    const btn = ["left", "middle", "right"][e.button] || "left";
    sendInput({ action: "mousemove", x, y });
    sendInput({ action: "mousedown", button: btn });
    e.preventDefault();
  }, [screenCoords, sendInput]);

  const handleMouseUp = useCallback((e: React.MouseEvent<HTMLCanvasElement>) => {
    const btn = ["left", "middle", "right"][e.button] || "left";
    sendInput({ action: "mouseup", button: btn });
    e.preventDefault();
  }, [sendInput]);

  const handleWheel = useCallback((e: React.WheelEvent<HTMLCanvasElement>) => {
    sendInput({ action: "scroll", delta: e.deltaY > 0 ? -120 : 120 });
    e.preventDefault();
  }, [sendInput]);

  const handleContextMenu = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
  }, []);

  // Keyboard handler
  useEffect(() => {
    if (vncStatus !== "connected" || !interactive) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't capture if focus is on an input/button
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;

      if (e.key.length === 1 && !e.ctrlKey && !e.altKey) {
        sendInput({ action: "type", text: e.key });
      } else {
        const vkMap: Record<string, number> = {
          Enter: 13, Backspace: 8, Tab: 9, Escape: 27, Delete: 46,
          ArrowLeft: 37, ArrowUp: 38, ArrowRight: 39, ArrowDown: 40,
          Home: 36, End: 35, PageUp: 33, PageDown: 34,
          F1: 112, F2: 113, F3: 114, F4: 115, F5: 116, F6: 117,
          F7: 118, F8: 119, F9: 120, F10: 121, F11: 122, F12: 123,
        };
        const vk = vkMap[e.key];
        if (vk) {
          const modifiers: number[] = [];
          if (e.ctrlKey) modifiers.push(17);
          if (e.altKey) modifiers.push(18);
          if (e.shiftKey) modifiers.push(16);
          sendInput({ action: "key", vk, modifiers });
        }
      }
      e.preventDefault();
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [vncStatus, interactive, sendInput]);

  // Fetch tunnel URL + auth token from backend, then send vnc_start to agent
  const handleStart = async () => {
    setVncStatus("starting");

    // Connect WebSocket first
    connectWs();

    // Get tunnel URL and auth token from the backend
    let backendTunnelUrl: string;
    let backendAuthToken: string;
    try {
      const statusRes = await fetch(`/api/vnc/status`);
      const statusData = await statusRes.json();
      backendTunnelUrl = statusData.tunnelUrl;
      backendAuthToken = statusData.authToken;
      if (backendTunnelUrl) setTunnelUrl(backendTunnelUrl);
    } catch (err) {
      setVncStatus("idle");
      toast.error("Failed to reach VNC backend on port 3001. Is it running?");
      return;
    }

    if (!backendTunnelUrl) {
      setVncStatus("idle");
      toast.error("No tunnel URL available. Backend may still be starting.");
      return;
    }

    // Send vnc_start command to agent with the tunnel URL + token
    const cmdId = await dispatchCommand(machineId, "vnc_start", {
      tunnel_url: backendTunnelUrl,
      auth_token: backendAuthToken,
    });
    if (!cmdId) {
      setVncStatus("idle");
      toast.error("Failed to dispatch VNC start command");
      return;
    }

    const result = await waitForResult(cmdId, 30000);
    if (result?.status === "complete") {
      toast.success("VNC client started on agent");
    } else {
      toast.error("VNC start failed: " + (result?.result || "timeout"));
    }
  };

  // Stop VNC
  const handleStop = async () => {
    const cmdId = await dispatchCommand(machineId, "vnc_stop", {});
    if (cmdId) {
      await waitForResult(cmdId, 10000);
    }

    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    setVncStatus("idle");
    setAgentConnected(false);
    toast.success("VNC stopped");
  };

  // Send config update
  useEffect(() => {
    if (wsRef.current && wsRef.current.readyState === 1) {
      wsRef.current.send(JSON.stringify({
        type: "config",
        quality: parseInt(quality),
        scale: 0.75,
        fps: parseInt(fps),
      }));
    }
  }, [quality, fps]);

  // Fullscreen
  const handleFullscreen = () => {
    canvasRef.current?.requestFullscreen?.();
  };

  return (
    <div className="space-y-4">
      {/* Control Bar */}
      <Card className="border-border/30">
        <CardHeader className="py-2 px-4">
          <div className="flex items-center justify-between flex-wrap gap-3">
            <div className="flex items-center gap-2">
              <MonitorPlay className="w-4 h-4 text-primary" />
              <CardTitle className="text-sm">Remote Desktop (VNC via Tunnel)</CardTitle>
              <Badge
                className={`text-[10px] ${
                  agentConnected
                    ? "bg-emerald-500/15 text-emerald-400 border-emerald-500/20"
                    : vncStatus === "starting"
                    ? "bg-amber-500/15 text-amber-400 border-amber-500/20"
                    : "bg-gray-500/15 text-gray-400 border-gray-500/20"
                }`}
              >
                <span
                  className={`w-1.5 h-1.5 rounded-full mr-1 ${
                    agentConnected
                      ? "bg-emerald-500"
                      : vncStatus === "starting"
                      ? "bg-amber-500 animate-pulse"
                      : "bg-gray-500"
                  }`}
                />
                {agentConnected ? "Agent Connected" : vncStatus === "starting" ? "Starting..." : vncStatus === "connected" ? "Waiting for Agent" : "Offline"}
              </Badge>
              {tunnelUrl && vncStatus !== "idle" && (
                <span className="text-[10px] text-muted-foreground font-mono truncate max-w-[200px]">{tunnelUrl}</span>
              )}
            </div>

            <div className="flex items-center gap-2">
              {vncStatus === "idle" ? (
                <Button size="sm" className="gap-1.5 h-7 text-xs" onClick={handleStart}>
                  <Power className="w-3 h-3" /> Start VNC
                </Button>
              ) : (
                <Button size="sm" variant="destructive" className="gap-1.5 h-7 text-xs" onClick={handleStop}>
                  <PowerOff className="w-3 h-3" /> Stop
                </Button>
              )}
            </div>
          </div>
        </CardHeader>
      </Card>

      {/* Toolbar */}
      {vncStatus !== "idle" && (
        <Card className="border-border/30">
          <CardContent className="py-2 px-4">
            <div className="flex items-center gap-4 flex-wrap">
              <div className="flex items-center gap-2">
                {interactive ? (
                  <MousePointer className="w-3.5 h-3.5 text-emerald-400" />
                ) : (
                  <Eye className="w-3.5 h-3.5 text-gray-400" />
                )}
                <Label htmlFor="interactive" className="text-xs">Interactive</Label>
                <Switch
                  id="interactive"
                  checked={interactive}
                  onCheckedChange={setInteractive}
                />
              </div>

              <div className="flex items-center gap-2">
                <Label className="text-xs text-muted-foreground">Quality</Label>
                <Select value={quality} onValueChange={setQuality}>
                  <SelectTrigger className="h-7 w-24 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="30">Low</SelectItem>
                    <SelectItem value="50">Medium</SelectItem>
                    <SelectItem value="75">High</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="flex items-center gap-2">
                <Label className="text-xs text-muted-foreground">FPS</Label>
                <Select value={fps} onValueChange={setFps}>
                  <SelectTrigger className="h-7 w-20 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1">1</SelectItem>
                    <SelectItem value="2">2</SelectItem>
                    <SelectItem value="4">4</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <Button size="sm" variant="ghost" className="h-7 text-xs gap-1" onClick={handleFullscreen}>
                <Maximize2 className="w-3 h-3" /> Fullscreen
              </Button>

              <div className="flex items-center gap-3 ml-auto text-[10px] text-muted-foreground font-mono">
                {agentConnected ? (
                  <span className="flex items-center gap-1">
                    <Wifi className="w-3 h-3 text-emerald-400" />
                    {screenshotSize > 0 && `${Math.round(screenshotSize / 1024)}KB`}
                    {frameCount > 0 && ` | ${frameCount} frames`}
                  </span>
                ) : (
                  <span className="flex items-center gap-1">
                    <WifiOff className="w-3 h-3 text-gray-500" /> No data
                  </span>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Canvas / Screen */}
      {vncStatus !== "idle" ? (
        <Card className="border-border/30 overflow-hidden">
          <CardContent className="p-0">
            <div className="bg-black min-h-[400px] flex items-center justify-center">
              {agentConnected ? (
                <canvas
                  ref={canvasRef}
                  className={interactive ? "cursor-crosshair" : "cursor-default"}
                  onMouseMove={handleMouseMove}
                  onMouseDown={handleMouseDown}
                  onMouseUp={handleMouseUp}
                  onWheel={handleWheel}
                  onContextMenu={handleContextMenu}
                  tabIndex={0}
                />
              ) : (
                <div className="text-center text-muted-foreground py-16">
                  <MonitorPlay className="w-12 h-12 mx-auto mb-3 opacity-20" />
                  <p className="text-sm">
                    {vncStatus === "starting"
                      ? "Starting VNC... Connecting agent to tunnel."
                      : "Waiting for agent to connect..."}
                  </p>
                  <p className="text-xs mt-1 text-muted-foreground/60">
                    The agent connects directly via HTTPS tunnel — should be ready in seconds
                  </p>
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      ) : (
        <Card className="border-border/30">
          <CardContent className="py-8">
            <div className="text-center text-muted-foreground">
              <MonitorPlay className="w-12 h-12 mx-auto mb-3 opacity-20" />
              <h3 className="text-sm font-medium mb-2">Remote Desktop via Tunnel</h3>
              <p className="text-xs max-w-md mx-auto mb-4">
                Control {machineName}'s screen remotely over an encrypted tunnel connection.
                Mouse, keyboard, and screen capture are relayed through a public HTTPS tunnel.
              </p>
              <div className="text-xs text-left max-w-sm mx-auto space-y-1.5 text-muted-foreground/70">
                <p>1. Ensure the backend server is running (<code className="bg-muted/50 px-1 rounded">npm run dev</code>)</p>
                <p>2. Click <strong>Start VNC</strong> above</p>
                <p>3. The tunnel URL is sent to the agent which connects directly via HTTPS</p>
                <p>4. Once connected, the screen will appear and you can interact with it</p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
};

export default VNCTab;
