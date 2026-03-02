import { supabase } from "@/integrations/supabase/client";

export async function dispatchCommand(
  machineId: string,
  commandType: string,
  args: Record<string, unknown> = {}
): Promise<string | null> {
  const { data, error } = await supabase
    .from("commands")
    .insert([
      {
        machine_id: machineId,
        command: commandType,
        args: JSON.stringify(args),
        status: "pending",
      },
    ])
    .select("id")
    .single();

  if (error) {
    console.error("Failed to dispatch command:", error);
    return null;
  }

  return data.id;
}

const TERMINAL = new Set(["complete", "error", "failed"]);

/**
 * Wait for a command result using polling.
 */
export function waitForResult(
  commandId: string,
  timeoutMs = 30000
): Promise<{ status: string; result: string | null } | null> {
  return new Promise((resolve) => {
    let resolved = false;

    const finish = (result: { status: string; result: string | null } | null) => {
      if (resolved) return;
      resolved = true;
      clearInterval(pollInterval);
      clearTimeout(timer);
      resolve(result);
    };

    const timer = setTimeout(() => {
      finish({ status: "timeout", result: null });
    }, timeoutMs);

    const checkResult = async () => {
      if (resolved) return;
      const { data, error } = await supabase
        .from("commands")
        .select("status, result")
        .eq("id", commandId)
        .single();

      if (error) return;
      if (data && TERMINAL.has(data.status || "")) {
        finish({ status: data.status!, result: data.result });
      }
    };

    const pollInterval = setInterval(checkResult, 250);
    checkResult();
  });
}

/**
 * Fire-and-forget command dispatch with optional toast.
 */
export async function dispatchCommandFast(
  machineId: string,
  commandType: string,
  args: Record<string, unknown> = {}
): Promise<string | null> {
  return dispatchCommand(machineId, commandType, args);
}

/**
 * Dispatch to multiple machines at once (batch).
 */
export async function dispatchBatchCommand(
  machineIds: string[],
  commandType: string,
  args: Record<string, unknown> = {}
): Promise<string[]> {
  const rows = machineIds.map((mid) => ({
    machine_id: mid,
    command: commandType,
    args: JSON.stringify(args),
    status: "pending",
  }));

  const { data, error } = await supabase
    .from("commands")
    .insert(rows)
    .select("id");

  if (error) {
    console.error("Batch dispatch failed:", error);
    return [];
  }

  return (data || []).map((d) => d.id);
}
