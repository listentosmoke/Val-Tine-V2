import { useEffect, useState } from "react";
import { supabase } from "@/integrations/supabase/client";
import { Skeleton } from "@/components/ui/skeleton";

interface ScreenshotCardProps {
  storagePath?: string | null;
  fallbackSrc?: string;
  createdAt: string | null;
}

const ScreenshotCard = ({ storagePath, fallbackSrc, createdAt }: ScreenshotCardProps) => {
  const [src, setSrc] = useState<string | undefined>(fallbackSrc);
  const [loading, setLoading] = useState(!!storagePath && !fallbackSrc);

  useEffect(() => {
    if (!storagePath || fallbackSrc) return;
    let cancelled = false;

    (async () => {
      const { data, error } = await supabase.storage
        .from("file-transfers")
        .createSignedUrl(storagePath, 3600);
      if (!cancelled && data?.signedUrl) {
        setSrc(data.signedUrl);
      }
      setLoading(false);
    })();

    return () => { cancelled = true; };
  }, [storagePath, fallbackSrc]);

  if (loading) {
    return (
      <div className="border border-border/20 rounded-lg overflow-hidden bg-black/20">
        <Skeleton className="aspect-video" />
        <div className="px-2 py-1.5 text-[10px] text-muted-foreground/60">
          {createdAt ? new Date(createdAt).toLocaleString() : "—"}
        </div>
      </div>
    );
  }

  if (!src) {
    return (
      <div className="border border-border/20 rounded-lg overflow-hidden bg-black/20">
        <div className="aspect-video flex items-center justify-center text-xs text-muted-foreground/40">
          No image
        </div>
        <div className="px-2 py-1.5 text-[10px] text-muted-foreground/60">
          {createdAt ? new Date(createdAt).toLocaleString() : "—"}
        </div>
      </div>
    );
  }

  return (
    <div className="border border-border/20 rounded-lg overflow-hidden bg-black/20 group">
      <img
        src={src}
        alt="Screenshot"
        className="w-full h-auto cursor-pointer hover:opacity-90 transition-opacity"
        loading="lazy"
        onClick={() => {
          const w = window.open();
          if (w) {
            w.document.write(`<img src="${src}" style="max-width:100%">`);
          }
        }}
      />
      <div className="px-2 py-1.5 text-[10px] text-muted-foreground/60">
        {createdAt ? new Date(createdAt).toLocaleString() : "—"}
      </div>
    </div>
  );
};

export default ScreenshotCard;
