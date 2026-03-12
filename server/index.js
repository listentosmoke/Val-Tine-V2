import express from "express";
import { WebSocketServer } from "ws";
import { createServer } from "http";
import { readFileSync, existsSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";
import { startTunnel, stopTunnel, getTunnelUrl } from "./tunnel.js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const PORT = 3001;

// Load auth token (written by setup.py)
import crypto from "crypto";

function loadToken() {
  const tokenFile = join(__dirname, "vnc_token.txt");
  if (existsSync(tokenFile)) {
    return readFileSync(tokenFile, "utf-8").trim();
  }
  return crypto.randomBytes(16).toString("hex");
}

// ============================================================
// STATE
// ============================================================

let authToken = null;
const commandQueue = [];       // Commands from dashboard → agent
let lastScreenshot = null;     // Latest JPEG screenshot bytes
let agentConnected = false;
let agentLastSeen = 0;
const dashboardClients = new Set(); // WebSocket connections from dashboard

// ============================================================
// EXPRESS APP
// ============================================================

const app = express();
app.use(express.json({ limit: "10mb" }));
app.use(express.raw({ type: "image/*", limit: "10mb" }));

// Auth middleware for agent endpoints
function agentAuth(req, res, next) {
  const token = req.headers.authorization?.replace("Bearer ", "");
  if (token !== authToken) {
    return res.status(401).json({ error: "unauthorized" });
  }
  next();
}

// --- Agent endpoints (exposed via tunnel) ---

// Agent polls for pending commands
app.get("/api/commands", agentAuth, (req, res) => {
  agentConnected = true;
  agentLastSeen = Date.now();

  const cmds = commandQueue.splice(0);
  res.json({ commands: cmds });
});

// Agent uploads a screenshot
app.post("/api/screenshot", agentAuth, (req, res) => {
  agentConnected = true;
  agentLastSeen = Date.now();

  let data;
  if (Buffer.isBuffer(req.body)) {
    data = req.body;
  } else if (req.body?.data) {
    data = Buffer.from(req.body.data, "base64");
  } else {
    return res.status(400).json({ error: "no screenshot data" });
  }

  lastScreenshot = data;

  // Push to all connected dashboard clients
  const b64 = data.toString("base64");
  broadcastToDashboard({
    type: "screenshot",
    data: b64,
    timestamp: Date.now(),
    size: data.length,
  });

  res.json({ ok: true });
});

// Agent heartbeat
app.post("/api/heartbeat", agentAuth, (req, res) => {
  const wasConnected = agentConnected;
  agentConnected = true;
  agentLastSeen = Date.now();

  if (!wasConnected) {
    broadcastToDashboard({ type: "status", agentConnected: true });
  }

  res.json({ ok: true });
});

// --- Internal endpoints (for dashboard) ---

app.get("/api/vnc/status", (req, res) => {
  res.json({
    tunnelUrl: getTunnelUrl(),
    agentConnected,
    agentLastSeen,
    queueLength: commandQueue.length,
    authToken: authToken,
  });
});

// ============================================================
// HTTP + WEBSOCKET SERVER
// ============================================================

const server = createServer(app);

const wss = new WebSocketServer({ noServer: true });

server.on("upgrade", (request, socket, head) => {
  const url = new URL(request.url, `http://${request.headers.host}`);
  if (url.pathname === "/ws") {
    wss.handleUpgrade(request, socket, head, (ws) => {
      wss.emit("connection", ws, request);
    });
  } else {
    socket.destroy();
  }
});

wss.on("connection", (ws) => {
  console.log("[vnc] Dashboard client connected");
  dashboardClients.add(ws);

  // Send current status
  ws.send(JSON.stringify({
    type: "status",
    agentConnected,
    tunnelUrl: getTunnelUrl(),
  }));

  // If we have a recent screenshot, send it
  if (lastScreenshot) {
    ws.send(JSON.stringify({
      type: "screenshot",
      data: lastScreenshot.toString("base64"),
      timestamp: Date.now(),
      size: lastScreenshot.length,
    }));
  }

  ws.on("message", (raw) => {
    try {
      const msg = JSON.parse(raw.toString());

      if (msg.type === "input") {
        // Queue input command for agent
        commandQueue.push({
          action: msg.action,
          x: msg.x,
          y: msg.y,
          button: msg.button,
          double: msg.double,
          delta: msg.delta,
          text: msg.text,
          vk: msg.vk,
          modifiers: msg.modifiers,
          timestamp: Date.now(),
        });
      } else if (msg.type === "config") {
        // Queue config update for agent
        commandQueue.push({
          action: "config",
          quality: msg.quality,
          scale: msg.scale,
          fps: msg.fps,
        });
      }
    } catch (e) {
      console.error("[vnc] Bad WebSocket message:", e.message);
    }
  });

  ws.on("close", () => {
    console.log("[vnc] Dashboard client disconnected");
    dashboardClients.delete(ws);
  });
});

function broadcastToDashboard(msg) {
  const data = JSON.stringify(msg);
  for (const ws of dashboardClients) {
    if (ws.readyState === 1) {
      ws.send(data);
    }
  }
}

// Check agent timeout (mark disconnected after 15s no heartbeat)
setInterval(() => {
  if (agentConnected && Date.now() - agentLastSeen > 15000) {
    agentConnected = false;
    broadcastToDashboard({ type: "status", agentConnected: false });
  }
}, 5000);

// ============================================================
// STARTUP
// ============================================================

async function start() {
  // Load auth token
  authToken = loadToken();
  if (!existsSync(join(__dirname, "vnc_token.txt"))) {
    console.log("[vnc] Generated temporary auth token:", authToken);
    console.log("[vnc] Run setup.py to generate a permanent token");
  }

  server.listen(PORT, () => {
    console.log(`[vnc] Backend server listening on http://127.0.0.1:${PORT}`);
  });

  // Start localtunnel with auto-reconnect
  try {
    const url = await startTunnel(PORT, (newUrl) => {
      // Tunnel reconnected with a new URL — notify dashboard
      console.log(`[vnc] Tunnel reconnected: ${newUrl}`);
      broadcastToDashboard({ type: "status", tunnelUrl: newUrl, agentConnected });
    });
    console.log(`[vnc] Tunnel URL: ${url}`);
    broadcastToDashboard({ type: "status", tunnelUrl: url, agentConnected });
  } catch (err) {
    console.error("[vnc] Failed to start tunnel:", err.message);
    console.log("[vnc] Backend will still work on localhost.");
  }
}

// Graceful shutdown
process.on("SIGINT", () => {
  console.log("[vnc] Shutting down...");
  stopTunnel();
  server.close();
  process.exit(0);
});

process.on("SIGTERM", () => {
  stopTunnel();
  server.close();
  process.exit(0);
});

start();
