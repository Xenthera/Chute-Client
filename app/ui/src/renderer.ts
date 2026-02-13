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

