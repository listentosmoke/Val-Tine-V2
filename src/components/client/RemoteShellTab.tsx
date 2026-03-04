import { useState, useRef, useEffect, useCallback } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Send,
  Loader2,
  Trash2,
  Download,
  ListTree,
  Search,
  X,
} from "lucide-react";
import { dispatchCommand, waitForResult } from "@/lib/commands";
import { toast } from "sonner";

interface ShellLine {
  id: string;
  type: "input" | "output" | "error" | "info";
  content: string;
  timestamp: Date;
}

interface Process {
  pid: number;
  ppid: number;
  name: string;
}

const RemoteShellTab = ({
  machineId,
  machineName,
}: {
  machineId: string;
  machineName: string;
}) => {
  const [lines, setLines] = useState<ShellLine[]>([
    {
      id: "welcome",
      type: "info",
      content: `Connected to ${machineName} (${machineId})`,
      timestamp: new Date(),
    },
  ]);
  const [input, setInput] = useState("");
  const [isRunning, setIsRunning] = useState(false);
  const [history, setHistory] = useState<string[]>([]);
  const [historyIdx, setHistoryIdx] = useState(-1);
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Process manager state
  const [processes, setProcesses] = useState<Process[]>([]);
  const [procLoading, setProcLoading] = useState(false);
  const [procSearch, setProcSearch] = useState("");
  const [showProcs, setShowProcs] = useState(false);

  useEffect(() => {
    scrollRef.current?.scrollTo(0, scrollRef.current.scrollHeight);
  }, [lines]);

  const addLine = useCallback(
    (type: ShellLine["type"], content: string) => {
      setLines((prev) => [
        ...prev,
        { id: crypto?.randomUUID?.() ?? Math.random().toString(36).slice(2), type, content, timestamp: new Date() },
      ]);
    },
    []
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isRunning) return;

    const cmd = input.trim();
    setInput("");
    setHistory((prev) => [cmd, ...prev.slice(0, 49)]);
    setHistoryIdx(-1);

    if (cmd === "clear") {
      setLines([]);
      return;
    }

    addLine("input", `$ ${cmd}`);
    setIsRunning(true);

    const commandId = await dispatchCommand(machineId, "shell", { cmd });
    if (!commandId) {
      addLine("error", "Failed to dispatch command");
      setIsRunning(false);
      return;
    }

    const result = await waitForResult(commandId);
    if (!result) {
      addLine("error", "Failed to get result");
    } else if (result.status === "timeout") {
      addLine("error", "Command timed out (agent may still be processing)");
    } else if (result.status === "error" || result.status === "failed") {
      addLine("error", result.result || "Command failed");
    } else {
      addLine("output", result.result || "(no output)");
    }

    setIsRunning(false);
    inputRef.current?.focus();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (history.length > 0) {
        const nextIdx = Math.min(historyIdx + 1, history.length - 1);
        setHistoryIdx(nextIdx);
        setInput(history[nextIdx]);
      }
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      if (historyIdx > 0) {
        const nextIdx = historyIdx - 1;
        setHistoryIdx(nextIdx);
        setInput(history[nextIdx]);
      } else {
        setHistoryIdx(-1);
        setInput("");
      }
    }
  };

  const downloadOutput = () => {
    const text = lines.map((l) => l.content).join("\n");
    const blob = new Blob([text], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `shell-${machineName}-${Date.now()}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const fetchProcesses = async () => {
    setProcLoading(true);
    const cmdId = await dispatchCommand(machineId, "processes");
    if (!cmdId) {
      toast.error("Failed to request process list");
      setProcLoading(false);
      return;
    }
    const result = await waitForResult(cmdId);
    setProcLoading(false);
    if (result?.result) {
      try {
        const parsed = JSON.parse(result.result);
        setProcesses(parsed);
        setShowProcs(true);
      } catch {
        toast.error("Failed to parse process list");
      }
    }
  };

  const killProcess = async (pid: number) => {
    const cmdId = await dispatchCommand(machineId, "kill", { pid });
    if (cmdId) {
      toast.success(`Kill signal sent for PID ${pid}`);
      setTimeout(fetchProcesses, 1000);
    }
  };

  const filteredProcs = processes.filter((p) =>
    procSearch ? p.name.toLowerCase().includes(procSearch.toLowerCase()) : true
  );

  return (
    <div className="space-y-3">
      {/* Process Manager */}
      <Card className="border-border/30">
        <CardHeader className="py-2 px-4">
          <div className="flex items-center justify-between">
            <CardTitle className="text-xs flex items-center gap-2 text-muted-foreground">
              <ListTree className="w-3.5 h-3.5 text-primary" /> Process Manager
            </CardTitle>
            <Button
              variant="outline"
              size="sm"
              className="h-6 text-xs gap-1"
              onClick={fetchProcesses}
              disabled={procLoading}
            >
              {procLoading ? (
                <Loader2 className="w-3 h-3 animate-spin" />
              ) : (
                <ListTree className="w-3 h-3" />
              )}
              {showProcs ? "Refresh" : "Load Processes"}
            </Button>
          </div>
        </CardHeader>
        {showProcs && (
          <CardContent className="pt-0 pb-3 px-4">
            <div className="flex items-center gap-2 mb-2">
              <Search className="w-3.5 h-3.5 text-muted-foreground/40" />
              <Input
                placeholder="Filter processes..."
                value={procSearch}
                onChange={(e) => setProcSearch(e.target.value)}
                className="h-7 text-xs bg-transparent border-none focus-visible:ring-0"
              />
              <Badge variant="secondary" className="text-[10px] flex-shrink-0">
                {filteredProcs.length}
              </Badge>
            </div>
            <div className="max-h-40 overflow-auto space-y-0.5 text-xs">
              {filteredProcs.slice(0, 100).map((p) => (
                <div
                  key={`${p.pid}-${p.name}`}
                  className="flex items-center justify-between px-2 py-1 rounded hover:bg-muted/30 group"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <span className="font-mono text-muted-foreground w-12 text-right flex-shrink-0">
                      {p.pid}
                    </span>
                    <span className="truncate">{p.name}</span>
                  </div>
                  <Button
                    size="icon"
                    variant="ghost"
                    className="h-5 w-5 opacity-0 group-hover:opacity-100 text-destructive"
                    onClick={() => killProcess(p.pid)}
                    title="Kill Process"
                  >
                    <X className="w-3 h-3" />
                  </Button>
                </div>
              ))}
            </div>
          </CardContent>
        )}
      </Card>

      {/* Terminal */}
      <Card className="overflow-hidden border-border/30">
        <CardContent className="p-0">
          <div className="flex items-center justify-between px-3 py-1.5 border-b border-border/20 bg-card/50">
            <span className="text-[10px] text-muted-foreground/50 font-mono">
              {machineName} - Remote Shell
            </span>
            <div className="flex items-center gap-1">
              <Button
                size="icon"
                variant="ghost"
                className="h-6 w-6 text-muted-foreground/50 hover:text-foreground"
                onClick={downloadOutput}
                title="Download Output"
              >
                <Download className="w-3 h-3" />
              </Button>
              <Button
                size="icon"
                variant="ghost"
                className="h-6 w-6 text-muted-foreground/50 hover:text-foreground"
                onClick={() => setLines([])}
                title="Clear"
              >
                <Trash2 className="w-3 h-3" />
              </Button>
            </div>
          </div>
          <div
            ref={scrollRef}
            className="bg-[hsl(222,47%,4%)] h-[420px] overflow-auto p-4 font-mono text-sm terminal-scrollbar"
          >
            {lines.map((line) => (
              <div
                key={line.id}
                className={`whitespace-pre-wrap mb-0.5 ${
                  line.type === "input"
                    ? "text-emerald-400"
                    : line.type === "error"
                    ? "text-red-400"
                    : line.type === "info"
                    ? "text-primary"
                    : "text-gray-300"
                }`}
              >
                {line.content}
              </div>
            ))}
            {isRunning && (
              <div className="flex items-center gap-2 text-muted-foreground">
                <Loader2 className="w-3 h-3 animate-spin" /> Waiting for
                response...
              </div>
            )}
          </div>
          <form
            onSubmit={handleSubmit}
            className="flex items-center border-t border-border/20 bg-[hsl(222,47%,6%)] p-2 gap-2"
          >
            <span className="text-emerald-400 font-mono text-sm pl-2">$</span>
            <Input
              ref={inputRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Type a command..."
              className="flex-1 bg-transparent border-none focus-visible:ring-0 font-mono text-sm text-gray-200 placeholder:text-muted-foreground/30"
              disabled={isRunning}
              autoFocus
            />
            <Button
              type="submit"
              size="icon"
              variant="ghost"
              disabled={isRunning || !input.trim()}
              className="text-muted-foreground hover:text-emerald-400"
            >
              <Send className="w-4 h-4" />
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
};

export default RemoteShellTab;
