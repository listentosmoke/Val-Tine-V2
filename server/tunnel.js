import localtunnel from "localtunnel";

let activeTunnel = null;
let tunnelUrl = null;

/**
 * Start a localtunnel to expose the local server to a public URL.
 * Returns the public https URL immediately (typically < 2 seconds).
 */
export async function startTunnel(localPort) {
  if (activeTunnel) {
    console.log("[tunnel] Tunnel already running:", tunnelUrl);
    return tunnelUrl;
  }

  console.log("[tunnel] Opening tunnel to localhost:" + localPort + "...");

  const tunnel = await localtunnel({ port: localPort });

  activeTunnel = tunnel;
  tunnelUrl = tunnel.url;

  console.log("[tunnel] Tunnel ready:", tunnelUrl);

  tunnel.on("close", () => {
    console.log("[tunnel] Tunnel closed");
    activeTunnel = null;
    tunnelUrl = null;
  });

  tunnel.on("error", (err) => {
    console.error("[tunnel] Tunnel error:", err.message);
  });

  return tunnelUrl;
}

export function stopTunnel() {
  if (activeTunnel) {
    console.log("[tunnel] Closing tunnel...");
    activeTunnel.close();
    activeTunnel = null;
    tunnelUrl = null;
  }
}

export function getTunnelUrl() {
  return tunnelUrl;
}
