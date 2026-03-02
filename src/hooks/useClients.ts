import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { supabase } from "@/integrations/supabase/client";

export interface Client {
  id: string;
  machine_id: string;
  machine_name: string | null;
  username: string | null;
  os: string | null;
  ip: string | null;
  is_admin: boolean;
  last_seen: string | null;
  created_at: string;
}

export function useClients() {
  return useQuery({
    queryKey: ["clients"],
    queryFn: async () => {
      const { data, error } = await supabase
        .from("clients")
        .select("*")
        .order("last_seen", { ascending: false });
      if (error) throw error;
      return data as Client[];
    },
    refetchInterval: 2000,
  });
}

export function useClient(machineId: string) {
  return useQuery({
    queryKey: ["client", machineId],
    queryFn: async () => {
      const { data, error } = await supabase
        .from("clients")
        .select("*")
        .eq("machine_id", machineId)
        .single();
      if (error) throw error;
      return data as Client;
    },
    enabled: !!machineId,
    refetchInterval: 2000,
  });
}

export function useRealtimeClients() {
  const queryClient = useQueryClient();

  useEffect(() => {
    const channel = supabase
      .channel("clients-realtime")
      .on("postgres_changes", { event: "*", schema: "public", table: "clients" }, () => {
        queryClient.invalidateQueries({ queryKey: ["clients"] });
      })
      .subscribe();

    return () => { supabase.removeChannel(channel); };
  }, [queryClient]);
}
