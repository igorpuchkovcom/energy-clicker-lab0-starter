const sessionEl = document.querySelector("#session-id");
const pointsEl = document.querySelector("#points");
const statusEl = document.querySelector("#api-status");
const logEl = document.querySelector("#log");
const lossEl = document.querySelector("#simulate-loss");

let sessionId = localStorage.getItem("energy-session") || "";
let pendingSafeKey = localStorage.getItem("energy-pending-key") || "";
let lastUnsafeBody = null;

function log(message, data) {
  const timestamp = new Date().toISOString().slice(11, 23);
  const suffix = data === undefined ? "" : ` ${JSON.stringify(data)}`;
  logEl.textContent = `[${timestamp}] ${message}${suffix}\n` + logEl.textContent;
}

function setSession(id) {
  sessionId = id;
  localStorage.setItem("energy-session", id);
  sessionEl.textContent = id || "none";
  updateButtons();
}

function setPoints(points) {
  pointsEl.textContent = String(points);
}

function updateButtons() {
  document.querySelectorAll("#refresh-state,#collect-unsafe,#retry-unsafe,#collect-safe,#retry-safe")
    .forEach(button => button.disabled = !sessionId);
}

async function checkReady() {
  try {
    const response = await fetch("/readyz");
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    statusEl.textContent = "ready";
    statusEl.className = "pill ok";
  } catch (error) {
    statusEl.textContent = "not ready";
    statusEl.className = "pill bad";
  }
}

async function createSession() {
  const response = await fetch("/api/session", { method: "POST" });
  const body = await response.json();
  if (!response.ok) throw new Error(body.error || `HTTP ${response.status}`);

  setSession(body.session_id);
  setPoints(body.points);
  pendingSafeKey = "";
  localStorage.removeItem("energy-pending-key");
  lastUnsafeBody = null;
  log("Session created", body);
}

async function refreshState() {
  if (!sessionId) return;
  const response = await fetch(`/api/state/${encodeURIComponent(sessionId)}`);
  const body = await response.json();
  if (!response.ok) throw new Error(body.error || `HTTP ${response.status}`);
  setPoints(body.points);
  log("Authoritative state loaded", body);
}

async function requestWithOptionalLoss(url, options) {
  if (!lossEl.checked) return fetch(url, options);

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 500);
  options.signal = controller.signal;
  options.headers = {
    ...options.headers,
    "X-Debug-Delay-After-Commit-Ms": "2000",
  };

  try {
    return await fetch(url, options);
  } finally {
    clearTimeout(timeout);
  }
}

async function collectUnsafe(retry = false) {
  if (!sessionId) return;
  const body = lastUnsafeBody || { session_id: sessionId };
  lastUnsafeBody = body;

  try {
    const response = await requestWithOptionalLoss("/api/debug/collect-unsafe", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const result = await response.json();
    if (!response.ok) throw new Error(result.error || `HTTP ${response.status}`);
    setPoints(result.points);
    log(retry ? "Unsafe retry completed" : "Unsafe collect completed", result);
  } catch (error) {
    log("Unsafe response lost/failed. The database may already have incremented.", { error: error.message });
  }
}

async function collectSafe(retry = false) {
  if (!sessionId) return;

  if (!pendingSafeKey) {
    pendingSafeKey = crypto.randomUUID();
    localStorage.setItem("energy-pending-key", pendingSafeKey);
  }

  try {
    const response = await requestWithOptionalLoss("/api/collect", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Idempotency-Key": pendingSafeKey,
      },
      body: JSON.stringify({ session_id: sessionId }),
    });
    const result = await response.json();
    if (!response.ok) throw new Error(result.error || `HTTP ${response.status}`);
    setPoints(result.points);
    log(retry ? "Safe retry completed" : "Safe collect completed", result);
    pendingSafeKey = "";
    localStorage.removeItem("energy-pending-key");
  } catch (error) {
    log("Safe response lost/failed. The idempotency key is retained for retry.", {
      error: error.message,
      idempotency_key: pendingSafeKey,
    });
  }
}

function run(action) {
  return async () => {
    try {
      await action();
    } catch (error) {
      log("Operation failed", { error: error.message });
    }
  };
}

document.querySelector("#new-session").addEventListener("click", run(createSession));
document.querySelector("#refresh-state").addEventListener("click", run(refreshState));
document.querySelector("#collect-unsafe").addEventListener("click", run(() => collectUnsafe(false)));
document.querySelector("#retry-unsafe").addEventListener("click", run(() => collectUnsafe(true)));
document.querySelector("#collect-safe").addEventListener("click", run(() => collectSafe(false)));
document.querySelector("#retry-safe").addEventListener("click", run(() => collectSafe(true)));

setSession(sessionId);
if (sessionId) refreshState().catch(error => log("Initial state load failed", { error: error.message }));
checkReady();
setInterval(checkReady, 5000);
