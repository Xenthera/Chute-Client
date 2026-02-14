type StatusResponse = {
  client_id: string;
  server_addr: string;
  connected: boolean;
  peer_id: string;
  rendezvous_healthy: boolean;
  rendezvous_checked: boolean;
};

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          Status: () => Promise<StatusResponse>;
          Connect: (targetID: string) => Promise<void>;
          Disconnect: () => Promise<void>;
          Send: (message: string) => Promise<void>;
          Pending: () => Promise<string>;
          Accept: () => Promise<void>;
          Decline: () => Promise<void>;
          Messages: () => Promise<string[]>;
        };
      };
    };
  }
}

export {};

const statusDot = document.getElementById("status-dot") as HTMLSpanElement | null;
const statusText = document.getElementById("status-text") as HTMLSpanElement | null;
const clientIdLabel = document.getElementById("client-id") as HTMLSpanElement | null;
const rendezvousStatus = document.getElementById("rendezvous-status") as HTMLDivElement | null;
const peerInput = document.getElementById("peer-id") as HTMLInputElement | null;
const connectButton = document.getElementById("connect") as HTMLButtonElement | null;
const messageInput = document.getElementById("message-input") as HTMLInputElement | null;
const messagesBox = document.getElementById("messages") as HTMLDivElement | null;
const acceptModal = document.getElementById("accept-modal") as HTMLDivElement | null;
const pendingIdLabel = document.getElementById("pending-id") as HTMLParagraphElement | null;
const acceptButton = document.getElementById("accept-btn") as HTMLButtonElement | null;
const declineButton = document.getElementById("decline-btn") as HTMLButtonElement | null;
const funnelCanvas = document.getElementById("funnel-canvas") as HTMLCanvasElement | null;

const tunnelRadius = 18;

const appendMessage = (text: string, kind: "local" | "remote" | "system" = "system") => {
  if (!messagesBox) {
    return;
  }
  const line = document.createElement("div");
  line.textContent = text;
  line.dataset.kind = kind;
  messagesBox.appendChild(line);
  messagesBox.scrollTop = messagesBox.scrollHeight;
};

const resizeFunnelCanvas = () => {
  if (!funnelCanvas) {
    return;
  }
  const rect = funnelCanvas.getBoundingClientRect();
  const dpr = window.devicePixelRatio || 1;
  funnelCanvas.width = Math.max(1, Math.floor(rect.width * dpr));
  funnelCanvas.height = Math.max(1, Math.floor(rect.height * dpr));
  const ctx = funnelCanvas.getContext("2d");
  if (!ctx) {
    return;
  }
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, rect.width, rect.height);
};

const getSurfaceColor = () => {
  const root = getComputedStyle(document.documentElement);
  const value = root.getPropertyValue("--surface").trim();
  return value || "#e7edf4";
};

const drawRoundedRect = (
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number
) => {
  const r = Math.max(0, Math.min(radius, Math.min(width, height) / 2));
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.lineTo(x + width - r, y);
  ctx.quadraticCurveTo(x + width, y, x + width, y + r);
  ctx.lineTo(x + width, y + height - r);
  ctx.quadraticCurveTo(x + width, y + height, x + width - r, y + height);
  ctx.lineTo(x + r, y + height);
  ctx.quadraticCurveTo(x, y + height, x, y + height - r);
  ctx.lineTo(x, y + r);
  ctx.quadraticCurveTo(x, y, x + r, y);
  ctx.closePath();
};

const drawTunnel = (ctx: CanvasRenderingContext2D, width: number, height: number, phase: number) => {
  ctx.fillStyle = "#0a0f16";
  ctx.fillRect(0, 0, width, height);

  const rings = 20;
  const minSize = Math.min(width, height);
  const baseRings = 10;
  const baseGap = Math.max(6, minSize / (baseRings * 1.4));
  const desiredMaxInset = (baseRings - 1) * baseGap + 2;
  const gap = Math.max(4, (desiredMaxInset - 2) / (rings - 1));
  const baseColor = { r: 130, g: 130, b: 130 };
  const glowColor = { r: 80, g: 255, b: 255 };

  for (let i = 0; i < rings; i += 1) {
    const inset = i * gap + 2;
    const ringWidth = Math.max(0, width - inset * 2);
    const ringHeight = Math.max(0, height - inset * 2);
    if (ringWidth <= 0 || ringHeight <= 0) {
      continue;
    }
    const localPhase = (1 - phase + i * 0.12) % 1;
    const glow = Math.max(0, Math.sin(localPhase * Math.PI * 2));
    const fadeToCenter = Math.max(0, 1 - (i / (rings - 1)) * 1.5);
    if (fadeToCenter <= 0) {
      continue;
    }
    const r = Math.round(baseColor.r + (glowColor.r - baseColor.r) * glow);
    const g = Math.round(baseColor.g + (glowColor.g - baseColor.g) * glow);
    const b = Math.round(baseColor.b + (glowColor.b - baseColor.b) * glow);
    ctx.strokeStyle = `rgba(${r}, ${g}, ${b}, ${fadeToCenter})`;
    ctx.lineWidth = 2;
    drawRoundedRect(ctx, inset, inset, ringWidth, ringHeight, tunnelRadius);
    ctx.stroke();
  }
};

const drawFunnelDoors = (
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  doorProgress: number,
  tunnelPhase: number
) => {
  ctx.clearRect(0, 0, width, height);
  drawTunnel(ctx, width, height, tunnelPhase);

  const doorColor = getSurfaceColor();
  const borderColor = "#b6bcc4";
  const half = width / 2;
  const offset = half * doorProgress;
  const leftX = -offset;
  const rightX = half + offset;
  const doorY = -2;
  const doorHeight = height + 4;

  ctx.fillStyle = doorColor;
  ctx.fillRect(leftX, doorY, half, doorHeight);
  ctx.fillRect(rightX, doorY, half, doorHeight);

  ctx.strokeStyle = borderColor;
  ctx.lineWidth = 1;
  ctx.strokeRect(leftX + 0.5, doorY + 0.5, half - 1, doorHeight - 1);
  ctx.strokeRect(rightX + 0.5, doorY + 0.5, half - 1, doorHeight - 1);
};

let doorTarget = 0;
const setDoorTarget = (open: boolean) => {
  doorTarget = open ? 1 : 0;
};

const startFunnelAnimation = () => {
  if (!funnelCanvas) {
    return;
  }
  let doorProgress = 0;
  const lerpFactor = 0.08;
  const tunnelSpeed = 0.44;

  const tick = (now: number) => {
    if (!funnelCanvas) {
      return;
    }
    const ctx = funnelCanvas.getContext("2d");
    if (!ctx) {
      return;
    }
    doorProgress += (doorTarget - doorProgress) * lerpFactor;
    if (Math.abs(doorTarget - doorProgress) < 0.001) {
      doorProgress = doorTarget;
    }

    const rect = funnelCanvas.getBoundingClientRect();
    const tunnelPhase = ((now / 1000) * tunnelSpeed) % 1;
    drawFunnelDoors(ctx, rect.width, rect.height, doorProgress, tunnelPhase);
    requestAnimationFrame(tick);
  };

  requestAnimationFrame(tick);
};

const formatError = (err: unknown) => {
  if (err instanceof Error && err.message) {
    return err.message;
  }
  if (typeof err === "string" && err) {
    return err;
  }
  try {
    const serialized = JSON.stringify(err);
    if (serialized && serialized !== "{}") {
      return serialized;
    }
  } catch {
    // ignore
  }
  return "unknown error";
};

let pendingPeerId = "";

const showPendingModal = (peerId: string) => {
  if (!acceptModal || !pendingIdLabel) {
    return;
  }
  pendingPeerId = peerId;
  const displayId = formatIdGroups(peerId) || "--";
  pendingIdLabel.textContent = `Peer: ${displayId}`;
  acceptModal.style.display = "flex";
};

const hidePendingModal = () => {
  if (!acceptModal) {
    return;
  }
  acceptModal.style.display = "none";
  pendingPeerId = "";
};

const setConnectButtonState = (connected: boolean) => {
  if (!connectButton) {
    return;
  }
  connectButton.textContent = connected ? "Disconnect" : "Connect";
};

const setConnectionStatus = (connected: boolean, peerId: string) => {
  if (statusDot) {
    statusDot.classList.toggle("connected", connected);
  }
  if (statusText) {
    statusText.textContent = connected && peerId ? `Connected to ${peerId}` : "Disconnected";
  }
  setConnectButtonState(connected);
  currentPeerId = connected ? peerId : "";
  setDoorTarget(connected);
};

const setRendezvousStatus = (healthy: boolean, checked: boolean) => {
  if (!rendezvousStatus) {
    return;
  }
  if (!checked) {
    rendezvousStatus.textContent = "Rendezvous: Checking";
    return;
  }
  rendezvousStatus.textContent = healthy ? "Rendezvous: Online" : "Rendezvous: Offline";
};

const formatIdGroups = (value: string) => {
  const digits = value.replace(/\D/g, "");
  if (!digits) {
    return "";
  }
  let out = "";
  let count = 0;
  for (let i = digits.length - 1; i >= 0; i -= 1) {
    out = digits[i] + out;
    count += 1;
    if (count === 3 && i !== 0) {
      out = " " + out;
      count = 0;
    }
  }
  return out;
};

let currentClientId = "";
let currentPeerId = "";

const setClientId = (clientId: string) => {
  if (!clientIdLabel) {
    return;
  }
  const formatted = formatIdGroups(clientId);
  if (formatted) {
    clientIdLabel.innerHTML = `Your ID: <span class="client-id-number">${formatted}</span>`;
  } else {
    clientIdLabel.innerHTML = "Your ID: <span class=\"client-id-number\">--</span>";
  }
  currentClientId = clientId;
};

const getApp = () => window.go?.main?.App;

const connectToPeer = async () => {
  if (!peerInput) {
    return;
  }
  const raw = peerInput.value.trim();
  const targetId = raw.replace(/\D/g, "");
  if (!targetId) {
    appendMessage("Enter a peer ID before connecting.");
    return;
  }
  if (statusText?.textContent?.includes("Connected to") && statusText.textContent.includes(targetId)) {
    appendMessage("Already connected to that peer.");
    return;
  }
  appendMessage(`Connecting to ${targetId}...`);
  try {
    const app = getApp();
    if (!app) {
      appendMessage("App bridge not ready.");
      return;
    }
    await app.Connect(targetId);
    appendMessage(`Connected to ${targetId}.`, "system");
  } catch (err) {
    appendMessage(`Connect failed: ${formatError(err)}`);
  }
};

const disconnectFromPeer = async () => {
  appendMessage("Disconnecting...");
  try {
    const app = getApp();
    if (!app) {
      appendMessage("App bridge not ready.");
      return;
    }
    await app.Disconnect();
    appendMessage("Disconnected.", "system");
  } catch (err) {
    appendMessage(`Disconnect failed: ${formatError(err)}`);
  }
};

const acceptPending = async () => {
  if (!pendingPeerId) {
    hidePendingModal();
    return;
  }
  const peerId = pendingPeerId;
  hidePendingModal();
  try {
    const app = getApp();
    if (!app) {
      appendMessage("App bridge not ready.");
      return;
    }
    await app.Accept();
    appendMessage(`Accepted connection from ${formatIdGroups(peerId) || peerId}.`, "system");
  } catch (err) {
    appendMessage(`Accept failed: ${formatError(err)}`);
  }
};

const declinePending = async () => {
  if (!pendingPeerId) {
    hidePendingModal();
    return;
  }
  const peerId = pendingPeerId;
  hidePendingModal();
  try {
    const app = getApp();
    if (!app) {
      appendMessage("App bridge not ready.");
      return;
    }
    await app.Decline();
    appendMessage(`Declined connection from ${formatIdGroups(peerId) || peerId}.`, "system");
  } catch (err) {
    appendMessage(`Decline failed: ${formatError(err)}`);
  }
};

const sendMessage = async () => {
  if (!messageInput) {
    return;
  }
  const text = messageInput.value.trim();
  if (!text) {
    return;
  }
  messageInput.value = "";
  appendMessage(`You: ${text}`, "local");
  try {
    const app = getApp();
    if (!app) {
      appendMessage("App bridge not ready.");
      return;
    }
    await app.Send(text);
  } catch (err) {
    appendMessage(`Send failed: ${formatError(err)}`);
  }
};

const pollStatus = async () => {
  try {
    const app = getApp();
    if (!app) {
      return;
    }
    const status = await app.Status();
    setConnectionStatus(status.connected, status.peer_id);
    setRendezvousStatus(status.rendezvous_healthy, status.rendezvous_checked);
    setClientId(status.client_id);
  } catch {
    setConnectionStatus(false, "");
    setRendezvousStatus(false, false);
    setClientId("");
  }
};

const pollMessages = async () => {
  try {
    const app = getApp();
    if (!app) {
      return;
    }
    const messages = await app.Messages();
    if (messages.length === 0) {
      return;
    }
    const displayId = formatIdGroups(currentPeerId) || "Peer";
    for (const message of messages) {
      appendMessage(`${displayId}: ${message}`, "remote");
    }
  } catch {
    return;
  }
};

const pollPending = async () => {
  if (pendingPeerId) {
    return;
  }
  try {
    const app = getApp();
    if (!app) {
      return;
    }
    const pending = await app.Pending();
    if (pending) {
      showPendingModal(pending);
    }
  } catch {
    return;
  }
};

const init = async () => {
  appendMessage("Chute GUI Running");
  resizeFunnelCanvas();
  window.addEventListener("resize", resizeFunnelCanvas);
  startFunnelAnimation();
  acceptButton?.addEventListener("click", acceptPending);
  declineButton?.addEventListener("click", declinePending);
  connectButton?.addEventListener("click", () => {
    const isConnected = statusText?.textContent?.startsWith("Connected to");
    if (isConnected) {
      disconnectFromPeer();
      return;
    }
    connectToPeer();
  });
  peerInput?.addEventListener("input", () => {
    if (!peerInput) {
      return;
    }
    const cursorAtEnd = peerInput.selectionStart === peerInput.value.length;
    peerInput.value = formatIdGroups(peerInput.value);
    if (cursorAtEnd) {
      peerInput.setSelectionRange(peerInput.value.length, peerInput.value.length);
    }
  });
  messageInput?.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      sendMessage();
    }
  });
  pollStatus();
  setInterval(pollStatus, 1000);
  setInterval(pollMessages, 500);
  setInterval(pollPending, 1000);
};

window.addEventListener("DOMContentLoaded", init);

