import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, join, normalize } from "node:path";
import { fileURLToPath } from "node:url";
import { randomUUID } from "node:crypto";
import WebSocket, { WebSocketServer } from "ws";

const root = fileURLToPath(new URL(".", import.meta.url));
const port = Number(process.env.PORT || 8787);
const androidClients = new Set();
const waitingAndroidClients = new Set();
const viewers = new Map();
let activeAndroid = null;
let streamInfo = null;

const iceServers = parseIceServers();
const relayMode = process.env.SFU_URL ? "sfu" : "turn";
const sfuUrl = process.env.SFU_URL || "";

const mimeTypes = new Map([
  [".html", "text/html; charset=utf-8"],
  [".js", "text/javascript; charset=utf-8"],
  [".css", "text/css; charset=utf-8"],
]);

const server = createServer(async (req, res) => {
  try {
    const requestPath = req.url === "/" ? "/index.html" : new URL(req.url, "http://localhost").pathname;
    if (requestPath === "/favicon.ico") {
      res.writeHead(204);
      res.end();
      return;
    }
    const normalized = normalize(requestPath).replace(/^(\.\.[/\\])+/, "");
    const filePath = join(root, normalized);
    const body = await readFile(filePath);
    res.writeHead(200, { "content-type": mimeTypes.get(extname(filePath)) || "application/octet-stream" });
    res.end(body);
  } catch {
    res.writeHead(404);
    res.end("Not found");
  }
});

const wss = new WebSocketServer({ server, path: "/ws" });

wss.on("connection", (socket) => {
  socket.id = randomUUID();
  socket.role = "unknown";

  socket.on("message", (message, isBinary) => {
    if (isBinary) return;
    handleControl(socket, message.toString());
  });

  socket.on("close", () => {
    if (socket.role === "android") {
      androidClients.delete(socket);
      if (activeAndroid === socket) {
        activeAndroid = [...androidClients][0] || null;
        streamInfo = null;
        broadcastToViewers({ type: "android-disconnected" });
      }
      return;
    }

    if (socket.role === "android-waiting") {
      waitingAndroidClients.delete(socket);
      return;
    }

    if (socket.role === "viewer") {
      viewers.delete(socket.id);
      sendToAndroid({ type: "viewer-left", viewerId: socket.id });
    }
  });
});

function handleControl(socket, text) {
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    return;
  }

  if (data.type === "hello" && data.role === "android") {
    socket.role = "android";
    androidClients.add(socket);
    waitingAndroidClients.delete(socket);
    activeAndroid = socket;
    streamInfo = data;
    socket.send(JSON.stringify({ type: "config", iceServers, relayMode, sfuUrl }));
    broadcastToViewers({ type: "stream-info", ...streamInfo, iceServers, relayMode, sfuUrl });
    for (const viewer of viewers.values()) {
      sendToAndroid({ type: "viewer-joined", viewerId: viewer.id });
    }
    return;
  }

  if (data.type === "hello" && data.role === "android-waiting") {
    socket.role = "android-waiting";
    waitingAndroidClients.add(socket);
    socket.send(JSON.stringify({ type: "config", iceServers, relayMode, sfuUrl }));
    if (!activeAndroid && viewers.size > 0) {
      socket.send(JSON.stringify({ type: "remote-request" }));
    }
    return;
  }

  if (data.type === "hello" && data.role === "viewer") {
    socket.role = "viewer";
    viewers.set(socket.id, socket);
    socket.send(JSON.stringify(
      activeAndroid
        ? { type: "stream-info", ...streamInfo, iceServers, relayMode, sfuUrl }
        : { type: "waiting", iceServers, relayMode, sfuUrl },
    ));
    if (activeAndroid) sendToAndroid({ type: "viewer-joined", viewerId: socket.id });
    if (!activeAndroid) notifyWaitingAndroid();
    return;
  }

  if (socket.role === "android") {
    const viewerId = data.viewerId;
    if (!viewerId && data.type === "stream-info") {
      streamInfo = { ...streamInfo, ...data };
      broadcastToViewers({ ...data, iceServers, relayMode, sfuUrl });
      return;
    }
    const viewer = viewers.get(viewerId);
    if (viewer?.readyState === WebSocket.OPEN) {
      viewer.send(JSON.stringify({ ...data, viewerId }));
    }
    return;
  }

  if (socket.role === "viewer") {
    if (data.type === "ping") {
      socket.send(JSON.stringify({ type: "pong", clientTimeMs: data.clientTimeMs, serverTimeMs: Date.now() }));
      return;
    }
    sendToAndroid({ ...data, viewerId: socket.id });
  }
}

function sendToAndroid(data) {
  if (activeAndroid?.readyState === WebSocket.OPEN) {
    activeAndroid.send(JSON.stringify(data));
  }
}

function notifyWaitingAndroid() {
  const message = JSON.stringify({ type: "remote-request" });
  for (const socket of waitingAndroidClients) {
    if (socket.readyState === WebSocket.OPEN) socket.send(message);
  }
}

function broadcastToViewers(data) {
  const message = JSON.stringify(data);
  for (const viewer of viewers.values()) {
    if (viewer.readyState === WebSocket.OPEN) viewer.send(message);
  }
}

function parseIceServers() {
  if (process.env.ICE_SERVERS_JSON) {
    try {
      return JSON.parse(process.env.ICE_SERVERS_JSON);
    } catch (error) {
      console.warn(`Invalid ICE_SERVERS_JSON: ${error.message}`);
    }
  }

  const servers = [{ urls: "stun:stun.l.google.com:19302" }];
  if (process.env.TURN_URL) {
    servers.push({
      urls: process.env.TURN_URL,
      username: process.env.TURN_USERNAME || "",
      credential: process.env.TURN_CREDENTIAL || "",
    });
  }
  return servers;
}

server.listen(port, "0.0.0.0", () => {
  console.log(`Remote assist signaling server running at http://localhost:${port}`);
});
