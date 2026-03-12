import localtunnel from "localtunnel";

let activeTunnel = null;
let tunnelUrl = null;
let reconnecting = false;
let localPort = null;
let onUrlChange = null; // callback when tunnel URL changes

const MAX_RETRIES = 10;
const BASE_DELAY = 3000; // 3s initial delay
const MAX_DELAY = 60000; // 60s max delay

/**
 * Start a localtunnel with auto-reconnect on failure.
 * @param {number} port - local port to tunnel
 * @param {function} urlChangeCallback - called with new URL on reconnect
 */
export async function startTunnel(port, urlChangeCallback) {
  localPort = port;
  onUrlChange = urlChangeCallback || null;
  return await connectTunnel(0);
}

async function connectTunnel(attempt) {
  if (activeTunnel) {
    try { activeTunnel.close(); } catch (_) {}
    activeTunnel = null;
    tunnelUrl = null;
  }

  const label = attempt === 0 ? "" : ` (attempt ${attempt + 1}/${MAX_RETRIES})`;
  console.log(`[tunnel] Opening tunnel to localhost:${localPort}...${label}`);

  try {
    const tunnel = await localtunnel({ port: localPort });

    activeTunnel = tunnel;
    tunnelUrl = tunnel.url;

    console.log("[tunnel] Tunnel ready:", tunnelUrl);

    tunnel.on("close", () => {
      console.log("[tunnel] Tunnel closed");
      const wasActive = !!activeTunnel;
      activeTunnel = null;
      tunnelUrl = null;
      // Auto-reconnect if not intentionally stopped
      if (wasActive && !reconnecting) {
        scheduleReconnect(0);
      }
    });

    tunnel.on("error", (err) => {
      console.error("[tunnel] Tunnel error:", err.message);
      // Connection errors trigger reconnect
      if (activeTunnel && !reconnecting) {
        activeTunnel = null;
        tunnelUrl = null;
        scheduleReconnect(0);
      }
    });

    return tunnelUrl;
  } catch (err) {
    console.error(`[tunnel] Failed to connect:`, err.message);
    if (attempt + 1 < MAX_RETRIES) {
      const delay = Math.min(BASE_DELAY * Math.pow(2, attempt), MAX_DELAY);
      console.log(`[tunnel] Retrying in ${(delay / 1000).toFixed(0)}s...`);
      await sleep(delay);
      return await connectTunnel(attempt + 1);
    }
    throw new Error(`Tunnel failed after ${MAX_RETRIES} attempts: ${err.message}`);
  }
}

function scheduleReconnect(attempt) {
  if (reconnecting) return;
  reconnecting = true;

  const delay = Math.min(BASE_DELAY * Math.pow(2, attempt), MAX_DELAY);
  console.log(`[tunnel] Reconnecting in ${(delay / 1000).toFixed(0)}s...`);

  setTimeout(async () => {
    try {
      const url = await connectTunnel(0);
      reconnecting = false;
      if (onUrlChange) {
        onUrlChange(url);
      }
    } catch (err) {
      reconnecting = false;
      console.error("[tunnel] Reconnect failed:", err.message);
      // Try again after max delay
      scheduleReconnect(MAX_RETRIES - 1);
    }
  }, delay);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function stopTunnel() {
  reconnecting = true; // prevent auto-reconnect
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
