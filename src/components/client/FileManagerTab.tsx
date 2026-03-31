import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  FolderOpen,
  Download,
  Loader2,
  RefreshCw,
  ArrowUp,
  File,
  Folder,
  Upload,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { dispatchCommand, waitForResult } from "@/lib/commands";
import { toast } from "sonner";

interface FileEntry {
  name: string;
  path: string;
  is_dir: boolean;
  size?: number;
}

const FileManagerTab = ({ machineId, clientOs }: { machineId: string; clientOs?: string | null }) => {
  const isAndroid = clientOs?.toLowerCase().includes("android") ?? false;
  const defaultPath = isAndroid ? "/sdcard" : "C:\\";
  const sep = isAndroid ? "/" : "\\";
  const [path, setPath] = useState(defaultPath);
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [uploadPath, setUploadPath] = useState("");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);

  const listDirectory = async (dirPath?: string) => {
    const targetPath = dirPath || path;
    setLoading(true);

    // Use the agent's "list" command
    const cmdId = await dispatchCommand(machineId, "list", { path: targetPath });
    if (!cmdId) {
      toast.error("Failed to send command");
      setLoading(false);
      return;
    }

    const result = await waitForResult(cmdId);
    setLoading(false);

    if (!result || result.status !== "complete" || !result.result) {
      toast.error("Failed to list directory");
      return;
    }

    // Try JSON parse first (Android agent returns JSON), fall back to line-based
    let entries: FileEntry[] = [];
    try {
      const parsed = JSON.parse(result.result);
      if (Array.isArray(parsed)) {
        entries = parsed.map((item: any) => ({
          name: item.name,
          path: item.path || (targetPath.replace(/[\\\/]+$/, "") + sep + item.name),
          is_dir: item.is_dir ?? !item.name.includes("."),
          size: item.size,
        }));
      }
    } catch {
      // Fallback: line-based parsing for Windows agents
      const lines = result.result
        .split("\n")
        .map((l: string) => l.trim())
        .filter(Boolean);
      const normalizedPath = targetPath.endsWith(sep)
        ? targetPath
        : targetPath + sep;

      entries = lines
        .filter((name: string) => !name.startsWith("Error"))
        .map((name: string) => {
          const hasExt = name.includes(".");
          return {
            name,
            path: normalizedPath + name,
            is_dir: !hasExt,
          };
        });
    }

    // Sort: directories first, then files
    entries.sort((a, b) => {
      if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
      return a.name.localeCompare(b.name);
    });

    setFiles(entries);
    if (dirPath) setPath(dirPath);
  };

  const downloadFile = async (filePath: string, fileName: string) => {
    const cmdId = await dispatchCommand(machineId, "download", {
      path: filePath,
    });
    if (cmdId) toast.success(`Download requested: ${fileName}`);
  };

  const joinPath = (dir: string, name: string) => {
    const cleanDir = dir.replace(/[\\\/]+$/, "");
    return `${cleanDir}${sep}${name}`;
  };

  const getBaseName = (p: string) => {
    const trimmed = p.trim().replace(/[\\\/]+$/, "");
    const parts = trimmed.split(/\\|\//);
    return parts[parts.length - 1] || "";
  };

  const hasFileName = (p: string) => {
    const base = getBaseName(p);
    if (!base || base.endsWith(":")) return false;
    return base.includes(".");
  };

  const handleUpload = async () => {
    if (!uploadFile || !uploadPath) return;
    setUploading(true);

    const reader = new FileReader();
    reader.onload = async () => {
      const base64 = (reader.result as string).split(",")[1];
      const cmdId = await dispatchCommand(machineId, "upload", {
        path: uploadPath,
        data: base64,
      });
      if (cmdId) toast.success("File upload sent");
      setUploading(false);
      setUploadOpen(false);
      setUploadFile(null);
      setUploadPath("");
    };
    reader.readAsDataURL(uploadFile);
  };

  const goUp = () => {
    if (isAndroid) {
      const parent = path.replace(/\/[^/]+\/?$/, "") || "/";
      listDirectory(parent);
    } else {
      const parent = path.replace(/\\[^\\]+\\?$/, "\\") || "C:\\";
      listDirectory(parent);
    }
  };

  return (
    <div className="space-y-3">
      {/* Path Bar */}
      <Card className="border-border/30">
        <CardContent className="p-3">
          <div className="flex gap-2">
            <Button
              size="icon"
              variant="outline"
              className="h-8 w-8 flex-shrink-0"
              onClick={goUp}
              disabled={loading}
              title="Go Up"
            >
              <ArrowUp className="w-3.5 h-3.5" />
            </Button>
            <Input
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="Directory path..."
              className="font-mono text-sm h-8"
              onKeyDown={(e) => e.key === "Enter" && listDirectory()}
            />
            <Button
              onClick={() => listDirectory()}
              disabled={loading}
              size="sm"
              className="gap-1.5 h-8"
            >
              {loading ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : (
                <RefreshCw className="w-3.5 h-3.5" />
              )}
              List
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5 h-8"
              onClick={() => {
                setUploadPath(path);
                setUploadOpen(true);
              }}
            >
              <Upload className="w-3.5 h-3.5" /> Upload
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* File Listing */}
      <Card className="border-border/30">
        <CardContent className="p-0">
          {files.length > 0 ? (
            <div className="divide-y divide-border/10">
              {/* Parent */}
              <div
                className="flex items-center gap-3 px-4 py-2 text-sm cursor-pointer hover:bg-muted/20 text-primary"
                onClick={goUp}
              >
                <Folder className="w-4 h-4 text-amber-500/70" />
                <span>..</span>
              </div>
              {files.map((file, i) => (
                <div
                  key={i}
                  className={`flex items-center justify-between px-4 py-2 text-sm hover:bg-muted/20 ${
                    file.is_dir ? "cursor-pointer" : ""
                  }`}
                  onClick={() => file.is_dir && listDirectory(file.path)}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    {file.is_dir ? (
                      <Folder className="w-4 h-4 text-amber-500/70 flex-shrink-0" />
                    ) : (
                      <File className="w-4 h-4 text-muted-foreground/50 flex-shrink-0" />
                    )}
                    <span
                      className={`truncate font-mono text-xs ${
                        file.is_dir ? "text-primary" : "text-foreground"
                      }`}
                    >
                      {file.name}
                    </span>
                  </div>
                  <div className="flex items-center gap-2 flex-shrink-0">
                    {file.size !== undefined && (
                      <span className="text-[10px] text-muted-foreground/50">
                        {(file.size / 1024).toFixed(1)} KB
                      </span>
                    )}
                    {!file.is_dir && (
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-6 w-6 text-muted-foreground hover:text-primary"
                        onClick={(e) => {
                          e.stopPropagation();
                          downloadFile(file.path, file.name);
                        }}
                      >
                        <Download className="w-3 h-3" />
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          ) : !loading ? (
            <div className="flex flex-col items-center justify-center py-12">
              <FolderOpen className="w-10 h-10 mb-3 text-muted-foreground/20" />
              <p className="text-sm text-muted-foreground">
                Click "List" to browse the file system
              </p>
            </div>
          ) : (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="w-6 h-6 animate-spin text-primary" />
            </div>
          )}
        </CardContent>
      </Card>

      {/* Upload Dialog */}
      <Dialog open={uploadOpen} onOpenChange={setUploadOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Upload File</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <Input
              placeholder={isAndroid ? "Remote path (e.g. /sdcard/Download/file.txt)" : "Remote path (e.g. C:\\Users\\admin\\file.txt)"}
              value={uploadPath}
              onChange={(e) => setUploadPath(e.target.value)}
              className="font-mono text-sm"
            />
            <input
              type="file"
              onChange={(e) => {
                const f = e.target.files?.[0] || null;
                setUploadFile(f);
                if (!f) return;
                setUploadPath((prev) => {
                  if (prev && hasFileName(prev)) return prev;
                  const dir = (prev || path).trim() || path;
                  return joinPath(dir, f.name);
                });
              }}
              className="block w-full text-sm text-muted-foreground file:mr-4 file:py-2 file:px-4 file:rounded-md file:border-0 file:text-sm file:bg-primary file:text-primary-foreground hover:file:bg-primary/90"
            />
            <Button
              onClick={handleUpload}
              disabled={uploading || !uploadFile || !uploadPath}
              className="w-full"
            >
              {uploading ? "Uploading..." : "Upload"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default FileManagerTab;
