type StatusResponse = {
  client_id: string;
  server_addr: string;
  connected: boolean;
  peer_id: string;
  rendezvous_healthy: boolean;
  rendezvous_checked: boolean;
};

type MessageResponse = {
  message: string;
};

const backendURL = "http://127.0.0.1:8787";

const statusDot = document.getElementById("status-dot") as HTMLSpanElement | null;
const statusText = document.getElementById("status-text") as HTMLSpanElement | null;
const clientIdLabel = document.getElementById("client-id") as HTMLSpanElement | null;
const rendezvousStatus = document.getElementById("rendezvous-status") as HTMLDivElement | null;
const peerInput = document.getElementById("peer-id") as HTMLInputElement | null;
const connectButton = document.getElementById("connect") as HTMLButtonElement | null;
const messageInput = document.getElementById("message-input") as HTMLInputElement | null;
const messagesBox = document.getElementById("messages") as HTMLDivElement | null;

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

const setConnectionStatus = (connected: boolean, peerId: string) => {
  if (statusDot) {
    statusDot.classList.toggle("connected", connected);
  }
  if (statusText) {
    statusText.textContent = connected && peerId ? `Connected to ${peerId}` : "Disconnected";
  }
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

const setClientId = (clientId: string) => {
  if (!clientIdLabel) {
    return;
  }
  const formatted = formatIdGroups(clientId);
  clientIdLabel.textContent = formatted ? `Your ID: ${formatted}` : "Your ID: --";
};

const postJSON = async <T>(path: string, payload: unknown): Promise<T> => {
  const resp = await fetch(`${backendURL}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(text || resp.statusText);
  }
  return resp.json() as Promise<T>;
};

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
    await postJSON("/connect", { target_id: targetId });
    appendMessage(`Connected to ${targetId}.`, "system");
  } catch (err) {
    appendMessage(`Connect failed: ${(err as Error).message}`);
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
    await postJSON("/send", { message: text });
  } catch (err) {
    appendMessage(`Send failed: ${(err as Error).message}`);
  }
};

const pollStatus = async () => {
  try {
    const resp = await fetch(`${backendURL}/status`);
    if (!resp.ok) {
      return;
    }
    const status = (await resp.json()) as StatusResponse;
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
    const resp = await fetch(`${backendURL}/messages`);
    if (resp.status === 204) {
      return;
    }
    if (!resp.ok) {
      return;
    }
    const payload = (await resp.json()) as MessageResponse;
    if (payload.message) {
      appendMessage(payload.message, "remote");
    }
  } catch {
    return;
  }
};

const init = async () => {
  appendMessage("Chute GUI Running");
  connectButton?.addEventListener("click", connectToPeer);
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
};

window.addEventListener("DOMContentLoaded", init);

