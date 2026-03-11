import { execFile, spawn } from "child_process";
import { createWriteStream, existsSync, mkdirSync, readFileSync, writeFileSync, unlinkSync } from "fs";
import { join, dirname } from "path";
import { pipeline } from "stream/promises";
import { createGunzip } from "zlib";
import { extract } from "tar";
import { fileURLToPath } from "url";
import https from "https";
import http from "http";

const __dirname = dirname(fileURLToPath(import.meta.url));
const TOR_DATA_DIR = join(__dirname, "tor_data");
const TOR_BIN_DIR = join(TOR_DATA_DIR, "bin");
const HIDDEN_SERVICE_DIR = join(TOR_DATA_DIR, "hidden_service");
const DATA_DIR = join(TOR_DATA_DIR, "data");
const ONION_FILE = join(TOR_DATA_DIR, "onion_address.txt");
const TORRC_PATH = join(TOR_DATA_DIR, "torrc");

// Tor Expert Bundle URL — update version as needed
const TOR_VERSION = "14.0.4";
const TOR_BUNDLE_URL = `https://archive.torproject.org/tor-package-archive/torbrowser/${TOR_VERSION}/tor-expert-bundle-windows-x86_64-${TOR_VERSION}.tar.gz`;

let torProcess = null;

function getTorExePath() {
  if (process.platform === "win32") {
    return join(TOR_BIN_DIR, "tor", "tor.exe");
  }
  return join(TOR_BIN_DIR, "tor", "tor");
}

function download(url) {
  return new Promise((resolve, reject) => {
    const get = url.startsWith("https") ? https.get : http.get;
    get(url, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        return download(res.headers.location).then(resolve).catch(reject);
      }
      if (res.statusCode !== 200) {
        return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
      }
      resolve(res);
    }).on("error", reject);
  });
}

export async function downloadTor() {
  const torExe = getTorExePath();
  if (existsSync(torExe)) {
    console.log("[tor] Tor binary already exists:", torExe);
    return torExe;
  }

  console.log("[tor] Downloading Tor Expert Bundle...");
  mkdirSync(TOR_BIN_DIR, { recursive: true });

  const archivePath = join(TOR_DATA_DIR, "tor-bundle.tar.gz");

  // Download
  const res = await download(TOR_BUNDLE_URL);
  const ws = createWriteStream(archivePath);
  await pipeline(res, ws);
  console.log("[tor] Download complete, extracting...");

  // Extract tar.gz
  await extract({
    file: archivePath,
    cwd: TOR_BIN_DIR,
  });

  // Clean up archive
  try { unlinkSync(archivePath); } catch {}

  if (!existsSync(torExe)) {
    throw new Error(`Tor binary not found after extraction at ${torExe}`);
  }

  console.log("[tor] Tor extracted to:", torExe);
  return torExe;
}

export async function startTorHiddenService(localPort) {
  const torExe = await downloadTor();

  mkdirSync(HIDDEN_SERVICE_DIR, { recursive: true });
  mkdirSync(DATA_DIR, { recursive: true });

  // Write torrc
  const torrc = [
    `SocksPort 0`,
    `HiddenServiceDir ${HIDDEN_SERVICE_DIR.replace(/\\/g, "/")}`,
    `HiddenServicePort 80 127.0.0.1:${localPort}`,
    `DataDirectory ${DATA_DIR.replace(/\\/g, "/")}`,
  ].join("\n");

  writeFileSync(TORRC_PATH, torrc, "utf-8");
  console.log("[tor] torrc written, starting Tor...");

  // Start tor process
  torProcess = spawn(torExe, ["-f", TORRC_PATH], {
    stdio: ["ignore", "pipe", "pipe"],
  });

  torProcess.stdout.on("data", (d) => {
    const line = d.toString().trim();
    if (line) console.log("[tor]", line);
  });

  torProcess.stderr.on("data", (d) => {
    const line = d.toString().trim();
    if (line) console.log("[tor:err]", line);
  });

  torProcess.on("exit", (code) => {
    console.log("[tor] Process exited with code:", code);
    torProcess = null;
  });

  // Wait for hostname file
  const hostnameFile = join(HIDDEN_SERVICE_DIR, "hostname");
  const onion = await waitForFile(hostnameFile, 120000);
  const address = onion.trim();

  // Save for setup.py
  writeFileSync(ONION_FILE, address, "utf-8");
  console.log("[tor] Hidden service ready:", address);

  return address;
}

function waitForFile(path, timeoutMs) {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const check = () => {
      if (existsSync(path)) {
        const content = readFileSync(path, "utf-8");
        if (content.trim().length > 0) {
          return resolve(content);
        }
      }
      if (Date.now() - start > timeoutMs) {
        return reject(new Error(`Timeout waiting for ${path}`));
      }
      setTimeout(check, 2000);
    };
    check();
  });
}

export function stopTor() {
  if (torProcess) {
    console.log("[tor] Stopping Tor...");
    torProcess.kill();
    torProcess = null;
  }
}

export function getOnionAddress() {
  if (existsSync(ONION_FILE)) {
    return readFileSync(ONION_FILE, "utf-8").trim();
  }
  return null;
}
