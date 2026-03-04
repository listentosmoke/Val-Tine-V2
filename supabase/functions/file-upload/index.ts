import { createClient } from "https://esm.sh/@supabase/supabase-js@2";

const corsHeaders = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Headers":
    "authorization, x-client-info, apikey, content-type, x-machine-id, x-filename, x-filepath, x-type",
};

Deno.serve(async (req) => {
  if (req.method === "OPTIONS") {
    return new Response(null, { headers: corsHeaders });
  }

  try {
    const machineId = req.headers.get("x-machine-id");
    const filename = req.headers.get("x-filename") || "unknown";
    const filepath = req.headers.get("x-filepath") || "";
    const uploadType = req.headers.get("x-type") || "file"; // "file" or "screenshot"

    if (!machineId) {
      return new Response(JSON.stringify({ error: "Missing x-machine-id" }), {
        status: 400,
        headers: { ...corsHeaders, "Content-Type": "application/json" },
      });
    }

    const body = await req.arrayBuffer();
    const fileSize = body.byteLength;

    if (fileSize === 0) {
      return new Response(JSON.stringify({ error: "Empty file body" }), {
        status: 400,
        headers: { ...corsHeaders, "Content-Type": "application/json" },
      });
    }

    const supabase = createClient(
      Deno.env.get("SUPABASE_URL")!,
      Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")!
    );

    const storagePath = `${machineId}/${Date.now()}_${filename}`;

    const { error: uploadError } = await supabase.storage
      .from("file-transfers")
      .upload(storagePath, body, {
        contentType: "application/octet-stream",
        upsert: false,
      });

    if (uploadError) {
      console.error("Upload error:", uploadError);
      return new Response(
        JSON.stringify({ error: "Storage upload failed", details: uploadError.message }),
        { status: 500, headers: { ...corsHeaders, "Content-Type": "application/json" } }
      );
    }

    if (uploadType === "screenshot") {
      // Insert into screenshots table with storage_path
      const { data: ssRecord, error: insertError } = await supabase
        .from("screenshots")
        .insert({
          machine_id: machineId,
          storage_path: storagePath,
        })
        .select("id")
        .single();

      if (insertError) {
        console.error("Screenshot insert error:", insertError);
        return new Response(
          JSON.stringify({ error: "Screenshot metadata insert failed", details: insertError.message }),
          { status: 500, headers: { ...corsHeaders, "Content-Type": "application/json" } }
        );
      }

      // Clean up old screenshots to stay within storage limits (keep latest 20)
      const { data: oldScreenshots } = await supabase
        .from("screenshots")
        .select("id, storage_path")
        .eq("machine_id", machineId)
        .order("created_at", { ascending: false })
        .range(20, 1000);

      if (oldScreenshots && oldScreenshots.length > 0) {
        const pathsToDelete = oldScreenshots
          .map((s: any) => s.storage_path)
          .filter(Boolean);
        const idsToDelete = oldScreenshots.map((s: any) => s.id);

        if (pathsToDelete.length > 0) {
          await supabase.storage.from("file-transfers").remove(pathsToDelete);
        }
        await supabase
          .from("screenshots")
          .delete()
          .in("id", idsToDelete);
      }

      return new Response(
        JSON.stringify({ id: ssRecord.id, storage_path: storagePath, type: "screenshot" }),
        { status: 200, headers: { ...corsHeaders, "Content-Type": "application/json" } }
      );
    } else {
      // Insert into files table
      const { data: fileRecord, error: insertError } = await supabase
        .from("files")
        .insert({
          machine_id: machineId,
          filename: filename,
          filepath: filepath,
          size: fileSize,
          storage_path: storagePath,
        })
        .select("id")
        .single();

      if (insertError) {
        console.error("Insert error:", insertError);
        return new Response(
          JSON.stringify({ error: "Metadata insert failed", details: insertError.message }),
          { status: 500, headers: { ...corsHeaders, "Content-Type": "application/json" } }
        );
      }

      return new Response(
        JSON.stringify({ id: fileRecord.id, storage_path: storagePath, size: fileSize }),
        { status: 200, headers: { ...corsHeaders, "Content-Type": "application/json" } }
      );
    }
  } catch (err) {
    console.error("Unhandled error:", err);
    return new Response(
      JSON.stringify({ error: "Internal error", details: String(err) }),
      { status: 500, headers: { ...corsHeaders, "Content-Type": "application/json" } }
    );
  }
});
