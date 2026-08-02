(() => {
  "use strict";

  if (window.__evolutionCallManagerLoaded) return;
  window.__evolutionCallManagerLoaded = true;

  const DATA_CHANNEL_LABEL = "evolution-call-pcm";
  const DATA_CHANNEL_PROTOCOL = "evcall.pcm.v1";
  const PCM_RATE = 16000;
  const PCM_FRAME_SAMPLES = 960;
  const MAX_BUFFERED_AMOUNT = 256 * 1024;
  const HEADER_BYTES = 16;
  const STORAGE_KEY = "evolution.callManager.config.v1";
  const SESSION_KEY = "evolution.callManager.session.v1";
  const POLL_INTERVAL_MS = 1800;

  const state = {
    open: false,
    loading: false,
    calls: [],
    selectedCallId: "",
    autoConnectCallId: "",
    pollTimer: null,
    peer: null,
    channel: null,
    sessionId: "",
    mediaCallId: "",
    audioContext: null,
    microphoneStream: null,
    captureSource: null,
    captureNode: null,
    playbackNode: null,
    captureResampler: null,
    playbackResampler: null,
    capturePending: new Float32Array(0),
    muted: false,
    sentFrames: 0,
    receivedFrames: 0,
    droppedFrames: 0,
  };

  const root = document.createElement("div");
  root.id = "evcall-root";
  root.innerHTML = `
    <button class="evcall-launcher" type="button" aria-label="Abrir chamadas" title="Chamadas">
      <span aria-hidden="true">☎</span>
      <span class="evcall-badge" hidden>0</span>
    </button>
    <section class="evcall-panel" hidden aria-label="Painel de chamadas">
      <header class="evcall-header">
        <div>
          <strong>Chamadas WhatsApp</strong>
          <small class="evcall-runtime">Configuração necessária</small>
        </div>
        <button class="evcall-icon evcall-close" type="button" aria-label="Fechar">×</button>
      </header>

      <div class="evcall-body">
        <details class="evcall-settings" open>
          <summary>Conexão da instância</summary>
          <div class="evcall-grid">
            <label>URL da API
              <input class="evcall-base-url" type="url" inputmode="url" autocomplete="url">
            </label>
            <label>API key da instância
              <input class="evcall-api-key" type="password" autocomplete="off">
            </label>
            <label class="evcall-check">
              <input class="evcall-remember" type="checkbox">
              Salvar chave neste navegador
            </label>
            <button class="evcall-secondary evcall-save" type="button">Salvar e consultar</button>
          </div>
        </details>

        <div class="evcall-dialer">
          <label>Número com DDI
            <input class="evcall-number" type="tel" inputmode="numeric" autocomplete="tel" placeholder="5511999999999">
          </label>
          <button class="evcall-primary evcall-start" type="button">Ligar</button>
        </div>

        <div class="evcall-current" hidden>
          <div class="evcall-current-main">
            <span class="evcall-direction"></span>
            <strong class="evcall-peer"></strong>
            <span class="evcall-state"></span>
          </div>
          <div class="evcall-actions">
            <button class="evcall-primary evcall-accept" type="button" hidden>Atender</button>
            <button class="evcall-danger evcall-reject" type="button" hidden>Recusar</button>
            <button class="evcall-secondary evcall-connect" type="button" hidden>Conectar áudio</button>
            <button class="evcall-secondary evcall-mute" type="button" hidden>Silenciar</button>
            <button class="evcall-danger evcall-hangup" type="button" hidden>Encerrar</button>
          </div>
          <div class="evcall-media-state"></div>
        </div>

        <div class="evcall-section-head">
          <strong>Chamadas da instância</strong>
          <button class="evcall-icon evcall-refresh" type="button" title="Atualizar" aria-label="Atualizar">↻</button>
        </div>
        <div class="evcall-list"><p class="evcall-empty">Informe a API key para consultar.</p></div>

        <details class="evcall-log-wrap">
          <summary>Diagnóstico</summary>
          <pre class="evcall-log"></pre>
        </details>
      </div>
    </section>
  `;
  document.body.appendChild(root);

  const ui = {
    launcher: root.querySelector(".evcall-launcher"),
    badge: root.querySelector(".evcall-badge"),
    panel: root.querySelector(".evcall-panel"),
    close: root.querySelector(".evcall-close"),
    runtime: root.querySelector(".evcall-runtime"),
    settings: root.querySelector(".evcall-settings"),
    baseUrl: root.querySelector(".evcall-base-url"),
    apiKey: root.querySelector(".evcall-api-key"),
    remember: root.querySelector(".evcall-remember"),
    save: root.querySelector(".evcall-save"),
    number: root.querySelector(".evcall-number"),
    start: root.querySelector(".evcall-start"),
    current: root.querySelector(".evcall-current"),
    direction: root.querySelector(".evcall-direction"),
    peer: root.querySelector(".evcall-peer"),
    callState: root.querySelector(".evcall-state"),
    accept: root.querySelector(".evcall-accept"),
    reject: root.querySelector(".evcall-reject"),
    connect: root.querySelector(".evcall-connect"),
    mute: root.querySelector(".evcall-mute"),
    hangup: root.querySelector(".evcall-hangup"),
    mediaState: root.querySelector(".evcall-media-state"),
    refresh: root.querySelector(".evcall-refresh"),
    list: root.querySelector(".evcall-list"),
    log: root.querySelector(".evcall-log"),
  };

  class StreamingLinearResampler {
    constructor(inputRate, outputRate) {
      this.step = inputRate / outputRate;
      this.position = 0;
      this.carry = new Float32Array(0);
    }

    push(input) {
      if (!(input instanceof Float32Array) || input.length === 0) return new Float32Array(0);
      const data = new Float32Array(this.carry.length + input.length);
      data.set(this.carry);
      data.set(input, this.carry.length);
      const output = [];
      let position = this.position;
      while (position + 1 < data.length) {
        const left = Math.floor(position);
        const fraction = position - left;
        output.push(data[left] + (data[left + 1] - data[left]) * fraction);
        position += this.step;
      }
      const consumed = Math.floor(position);
      this.carry = data.slice(Math.min(consumed, data.length));
      this.position = position - consumed;
      return Float32Array.from(output);
    }
  }

  function safeJSON(value, fallback = null) {
    try { return JSON.parse(value); } catch (_) { return fallback; }
  }

  function loadConfig() {
    const persistent = safeJSON(localStorage.getItem(STORAGE_KEY), null);
    const temporary = safeJSON(sessionStorage.getItem(SESSION_KEY), null);
    const config = persistent || temporary || {};
    ui.baseUrl.value = config.baseUrl || window.location.origin;
    ui.apiKey.value = config.apiKey || "";
    ui.remember.checked = Boolean(persistent);
    if (config.number) ui.number.value = config.number;
  }

  function saveConfig() {
    const config = {
      baseUrl: normalizedBaseURL(),
      apiKey: ui.apiKey.value.trim(),
      number: normalizeNumber(ui.number.value),
    };
    if (ui.remember.checked) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(config));
      sessionStorage.removeItem(SESSION_KEY);
    } else {
      sessionStorage.setItem(SESSION_KEY, JSON.stringify(config));
      localStorage.removeItem(STORAGE_KEY);
    }
  }

  function normalizedBaseURL() {
    return (ui.baseUrl.value.trim() || window.location.origin).replace(/\/+$/, "");
  }

  function normalizeNumber(value) {
    return String(value || "").replace(/\D/g, "");
  }

  function callByID(callId) {
    return state.calls.find(call => call.id === callId) || null;
  }

  function selectedCall() {
    return callByID(state.selectedCallId);
  }

  function isTerminal(call) {
    return !call || call.state === "ended" || call.state === "failed";
  }

  function formatPeer(peer) {
    const value = String(peer || "");
    return value.replace(/:\d+@/, "@").split("@")[0] || "Contato desconhecido";
  }

  function stateLabel(value) {
    return ({
      ringing: "Chamando",
      connecting: "Conectando",
      active: "Ativa",
      ended: "Encerrada",
      failed: "Falhou",
      idle: "Inativa",
    })[value] || value || "Desconhecido";
  }

  function log(message, details) {
    const suffix = details === undefined ? "" : ` ${typeof details === "string" ? details : JSON.stringify(details)}`;
    ui.log.textContent += `[${new Date().toLocaleTimeString()}] ${message}${suffix}\n`;
    const lines = ui.log.textContent.split("\n");
    if (lines.length > 180) ui.log.textContent = lines.slice(-160).join("\n");
    ui.log.scrollTop = ui.log.scrollHeight;
  }

  function setBusy(busy) {
    state.loading = busy;
    [ui.save, ui.start, ui.accept, ui.reject, ui.connect, ui.hangup, ui.refresh].forEach(button => {
      button.disabled = busy;
    });
  }

  async function api(path, options = {}) {
    const key = ui.apiKey.value.trim();
    if (!key) throw new Error("Informe a API key da instância");
    const headers = new Headers(options.headers || {});
    headers.set("apikey", key);
    if (options.body !== undefined && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
    const response = await fetch(`${normalizedBaseURL()}${path}`, { ...options, headers });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || body.message || `HTTP ${response.status}`);
    return body;
  }

  function chooseDefaultCall() {
    if (state.selectedCallId && callByID(state.selectedCallId)) return;
    const live = [...state.calls].reverse().find(call => !isTerminal(call));
    const latest = state.calls[state.calls.length - 1];
    state.selectedCallId = live?.id || latest?.id || "";
  }

  function render() {
    const incoming = state.calls.filter(call => call.direction === "incoming" && call.state === "ringing");
    ui.badge.hidden = incoming.length === 0;
    ui.badge.textContent = String(incoming.length);

    ui.list.replaceChildren();
    if (state.calls.length === 0) {
      const empty = document.createElement("p");
      empty.className = "evcall-empty";
      empty.textContent = ui.apiKey.value.trim() ? "Nenhuma chamada registrada." : "Informe a API key para consultar.";
      ui.list.appendChild(empty);
    } else {
      [...state.calls].reverse().forEach(call => {
        const button = document.createElement("button");
        button.type = "button";
        button.className = `evcall-list-item${call.id === state.selectedCallId ? " selected" : ""}`;
        const top = document.createElement("span");
        top.className = "evcall-list-top";
        const peer = document.createElement("strong");
        peer.textContent = formatPeer(call.peer);
        const status = document.createElement("span");
        status.className = `evcall-pill state-${call.state || "unknown"}`;
        status.textContent = stateLabel(call.state);
        top.append(peer, status);
        const meta = document.createElement("small");
        meta.textContent = `${call.direction === "incoming" ? "Recebida" : "Realizada"} · ${call.video ? "vídeo" : "voz"} · ${call.id}`;
        button.append(top, meta);
        button.addEventListener("click", () => {
          state.selectedCallId = call.id;
          render();
        });
        ui.list.appendChild(button);
      });
    }

    const call = selectedCall();
    ui.current.hidden = !call;
    if (!call) return;

    ui.direction.textContent = call.direction === "incoming" ? "Chamada recebida" : "Chamada realizada";
    ui.peer.textContent = formatPeer(call.peer);
    ui.callState.textContent = stateLabel(call.state);
    ui.callState.className = `evcall-state state-${call.state || "unknown"}`;

    const incomingRinging = call.direction === "incoming" && call.state === "ringing";
    const canTerminate = !isTerminal(call);
    const mediaConnected = state.mediaCallId === call.id && Boolean(state.peer);
    ui.accept.hidden = !incomingRinging;
    ui.reject.hidden = !incomingRinging;
    ui.connect.hidden = call.state !== "active" || mediaConnected;
    ui.mute.hidden = !mediaConnected;
    ui.hangup.hidden = !canTerminate;
    ui.mute.textContent = state.muted ? "Ativar microfone" : "Silenciar";

    if (mediaConnected) {
      ui.mediaState.textContent = `Áudio conectado · enviados ${state.sentFrames} · recebidos ${state.receivedFrames} · descartados ${state.droppedFrames}`;
    } else if (call.state === "active") {
      ui.mediaState.textContent = "Chamada ativa. Conecte o microfone e o alto-falante.";
    } else {
      ui.mediaState.textContent = "O áudio ficará disponível quando a chamada estiver ativa.";
    }
  }

  async function refreshStatus({ quiet = false } = {}) {
    if (!ui.apiKey.value.trim()) {
      ui.runtime.textContent = "Configuração necessária";
      render();
      return;
    }
    try {
      const snapshot = await api("/call/status");
      state.calls = Array.isArray(snapshot.calls) ? snapshot.calls : [];
      chooseDefaultCall();
      ui.runtime.textContent = `${snapshot.connected ? "WhatsApp conectado" : "WhatsApp desconectado"}${snapshot.instanceId ? ` · ${snapshot.instanceId}` : ""}`;
      ui.settings.open = !snapshot.connected;
      const mediaCall = callByID(state.mediaCallId);
      if (state.mediaCallId && (!mediaCall || isTerminal(mediaCall))) await disconnectMedia({ notifyServer: false });
      const autoCall = callByID(state.autoConnectCallId);
      if (autoCall?.state === "active" && state.mediaCallId !== autoCall.id) {
        state.autoConnectCallId = "";
        connectMedia(autoCall.id).catch(error => log("Conexão automática do áudio falhou", error.message));
      }
      render();
    } catch (error) {
      ui.runtime.textContent = "Falha ao consultar instância";
      if (!quiet) log("Falha ao consultar chamadas", error.message);
    }
  }

  async function startCall() {
    const number = normalizeNumber(ui.number.value);
    if (number.length < 8 || number.length > 20) throw new Error("Informe o número completo com DDI");
    ui.number.value = number;
    saveConfig();
    const call = await api("/call/start", {
      method: "POST",
      body: JSON.stringify({ number, video: false }),
    });
    state.selectedCallId = call.id;
    state.autoConnectCallId = call.id;
    log("Chamada iniciada", { callId: call.id, peer: call.peer });
    await refreshStatus({ quiet: true });
  }

  async function acceptCall() {
    const call = selectedCall();
    if (!call) throw new Error("Selecione uma chamada");
    await api(`/call/${encodeURIComponent(call.id)}/accept`, { method: "POST" });
    state.autoConnectCallId = call.id;
    log("Chamada aceita", call.id);
    await refreshStatus({ quiet: true });
  }

  async function rejectCall() {
    const call = selectedCall();
    if (!call) throw new Error("Selecione uma chamada");
    await api("/call/reject", {
      method: "POST",
      body: JSON.stringify({ callCreator: call.peer, callId: call.id }),
    });
    state.autoConnectCallId = "";
    log("Chamada recusada", call.id);
    await refreshStatus({ quiet: true });
  }

  async function terminateCall() {
    const call = selectedCall();
    if (!call) throw new Error("Selecione uma chamada");
    if (state.mediaCallId === call.id) await disconnectMedia();
    await api(`/call/${encodeURIComponent(call.id)}`, { method: "DELETE" });
    state.autoConnectCallId = "";
    log("Chamada encerrada", call.id);
    await refreshStatus({ quiet: true });
  }

  function encodePCM(samples) {
    const buffer = new ArrayBuffer(HEADER_BYTES + samples.length * 4);
    const bytes = new Uint8Array(buffer);
    bytes.set([0x45, 0x56, 0x50, 0x43], 0);
    const view = new DataView(buffer);
    view.setUint8(4, 1);
    view.setUint8(5, 1);
    view.setUint16(6, 0, true);
    view.setUint32(8, PCM_RATE, true);
    view.setUint32(12, samples.length, true);
    for (let index = 0; index < samples.length; index++) {
      const sample = Number.isFinite(samples[index]) ? Math.max(-1, Math.min(1, samples[index])) : 0;
      view.setFloat32(HEADER_BYTES + index * 4, sample, true);
    }
    return buffer;
  }

  function decodePCM(buffer) {
    if (!(buffer instanceof ArrayBuffer) || buffer.byteLength < HEADER_BYTES) throw new Error("frame PCM truncado");
    const bytes = new Uint8Array(buffer, 0, 4);
    if (bytes[0] !== 0x45 || bytes[1] !== 0x56 || bytes[2] !== 0x50 || bytes[3] !== 0x43) throw new Error("magic PCM inválido");
    const view = new DataView(buffer);
    if (view.getUint8(4) !== 1 || view.getUint8(5) !== 1 || view.getUint16(6, true) !== 0) throw new Error("versão PCM incompatível");
    if (view.getUint32(8, true) !== PCM_RATE) throw new Error("sample rate PCM incompatível");
    const count = view.getUint32(12, true);
    if (!count || count > PCM_FRAME_SAMPLES * 4 || buffer.byteLength !== HEADER_BYTES + count * 4) throw new Error("tamanho PCM inválido");
    const output = new Float32Array(count);
    for (let index = 0; index < count; index++) output[index] = view.getFloat32(HEADER_BYTES + index * 4, true);
    return output;
  }

  async function installAudioWorklet(context) {
    const source = `
      class EvolutionManagerPCMProcessor extends AudioWorkletProcessor {
        constructor(options) {
          super();
          this.mode = options.processorOptions.mode;
          this.queue = [];
          this.offset = 0;
          this.port.onmessage = event => {
            if (this.mode === 'playback' && event.data instanceof Float32Array) this.queue.push(event.data);
          };
        }
        process(inputs, outputs) {
          if (this.mode === 'capture') {
            const input = inputs[0] && inputs[0][0];
            if (input && input.length) this.port.postMessage(new Float32Array(input));
          } else {
            const output = outputs[0] && outputs[0][0];
            if (output) {
              output.fill(0);
              let written = 0;
              while (written < output.length && this.queue.length) {
                const chunk = this.queue[0];
                const count = Math.min(output.length - written, chunk.length - this.offset);
                output.set(chunk.subarray(this.offset, this.offset + count), written);
                written += count;
                this.offset += count;
                if (this.offset >= chunk.length) { this.queue.shift(); this.offset = 0; }
              }
            }
          }
          return true;
        }
      }
      registerProcessor('evolution-manager-pcm', EvolutionManagerPCMProcessor);
    `;
    const url = URL.createObjectURL(new Blob([source], { type: "text/javascript" }));
    try { await context.audioWorklet.addModule(url); } finally { URL.revokeObjectURL(url); }
  }

  function gatherComplete(connection) {
    if (connection.iceGatheringState === "complete") return Promise.resolve();
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        connection.removeEventListener("icegatheringstatechange", listener);
        reject(new Error("timeout ao coletar candidatos ICE"));
      }, 15000);
      const listener = () => {
        if (connection.iceGatheringState === "complete") {
          clearTimeout(timeout);
          connection.removeEventListener("icegatheringstatechange", listener);
          resolve();
        }
      };
      connection.addEventListener("icegatheringstatechange", listener);
    });
  }

  function appendCapture(samples) {
    const joined = new Float32Array(state.capturePending.length + samples.length);
    joined.set(state.capturePending);
    joined.set(samples, state.capturePending.length);
    let offset = 0;
    while (joined.length - offset >= PCM_FRAME_SAMPLES) {
      const frame = joined.slice(offset, offset + PCM_FRAME_SAMPLES);
      offset += PCM_FRAME_SAMPLES;
      if (!state.muted && state.channel?.readyState === "open" && state.channel.bufferedAmount <= MAX_BUFFERED_AMOUNT) {
        state.channel.send(encodePCM(frame));
        state.sentFrames++;
      } else if (!state.muted) {
        state.droppedFrames++;
      }
    }
    state.capturePending = joined.slice(offset);
  }

  async function startAudio() {
    const AudioContextCtor = window.AudioContext || window.webkitAudioContext;
    if (!AudioContextCtor || !window.AudioWorkletNode) throw new Error("Este navegador não suporta AudioWorklet");
    state.audioContext = new AudioContextCtor({ latencyHint: "interactive" });
    await installAudioWorklet(state.audioContext);
    await state.audioContext.resume();
    state.captureResampler = new StreamingLinearResampler(state.audioContext.sampleRate, PCM_RATE);
    state.playbackResampler = new StreamingLinearResampler(PCM_RATE, state.audioContext.sampleRate);

    state.playbackNode = new AudioWorkletNode(state.audioContext, "evolution-manager-pcm", {
      numberOfInputs: 0,
      numberOfOutputs: 1,
      outputChannelCount: [1],
      processorOptions: { mode: "playback" },
    });
    state.playbackNode.connect(state.audioContext.destination);

    state.microphoneStream = await navigator.mediaDevices.getUserMedia({
      audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true },
      video: false,
    });
    state.captureSource = state.audioContext.createMediaStreamSource(state.microphoneStream);
    state.captureNode = new AudioWorkletNode(state.audioContext, "evolution-manager-pcm", {
      numberOfInputs: 1,
      numberOfOutputs: 0,
      processorOptions: { mode: "capture" },
    });
    state.captureNode.port.onmessage = event => appendCapture(state.captureResampler.push(event.data));
    state.captureSource.connect(state.captureNode);
    log("Microfone e reprodução iniciados", { sampleRate: state.audioContext.sampleRate });
  }

  async function connectMedia(callId) {
    const call = callByID(callId);
    if (!call || call.state !== "active") throw new Error("A chamada precisa estar ativa");
    if (!window.isSecureContext && location.hostname !== "localhost") throw new Error("O microfone exige HTTPS");
    if (state.peer) await disconnectMedia();

    state.sentFrames = 0;
    state.receivedFrames = 0;
    state.droppedFrames = 0;
    state.capturePending = new Float32Array(0);
    state.mediaCallId = callId;
    state.peer = new RTCPeerConnection({ iceServers: [] });
    state.channel = state.peer.createDataChannel(DATA_CHANNEL_LABEL, { ordered: true, protocol: DATA_CHANNEL_PROTOCOL });
    state.channel.binaryType = "arraybuffer";
    state.channel.bufferedAmountLowThreshold = MAX_BUFFERED_AMOUNT / 2;

    state.channel.onopen = async () => {
      log("Canal de áudio aberto", callId);
      try {
        await startAudio();
        render();
      } catch (error) {
        log("Falha ao iniciar áudio", error.message);
        await disconnectMedia();
      }
    };
    state.channel.onclose = () => {
      log("Canal de áudio fechado", callId);
      if (state.mediaCallId === callId) disconnectMedia({ notifyServer: false }).catch(() => {});
    };
    state.channel.onerror = event => log("Erro no canal de áudio", event?.message || "DataChannel");
    state.channel.onmessage = event => {
      try {
        const pcm16k = decodePCM(event.data);
        const playback = state.playbackResampler?.push(pcm16k) || new Float32Array(0);
        if (playback.length) state.playbackNode?.port.postMessage(playback, [playback.buffer]);
        state.receivedFrames++;
        if (state.receivedFrames % 10 === 0) render();
      } catch (error) {
        state.droppedFrames++;
        log("Frame de áudio recebido rejeitado", error.message);
      }
    };
    state.peer.onconnectionstatechange = () => {
      log("PeerConnection", state.peer?.connectionState || "closed");
      if (["failed", "closed"].includes(state.peer?.connectionState)) disconnectMedia({ notifyServer: false }).catch(() => {});
    };

    try {
      await state.peer.setLocalDescription(await state.peer.createOffer());
      await gatherComplete(state.peer);
      const body = await api(`/call/${encodeURIComponent(callId)}/webrtc`, {
        method: "POST",
        body: JSON.stringify({ offer: { type: "offer", sdp: state.peer.localDescription.sdp } }),
      });
      state.sessionId = body.sessionId;
      await state.peer.setRemoteDescription(body.answer);
      log("Sessão WebRTC criada", { callId, sessionId: state.sessionId });
      render();
    } catch (error) {
      await disconnectMedia({ notifyServer: false });
      throw error;
    }
  }

  async function disconnectMedia({ notifyServer = true } = {}) {
    const closingSession = state.sessionId;
    const closingCall = state.mediaCallId;
    state.sessionId = "";
    state.mediaCallId = "";

    state.microphoneStream?.getTracks().forEach(track => track.stop());
    state.microphoneStream = null;
    state.captureSource?.disconnect();
    state.captureNode?.disconnect();
    state.playbackNode?.disconnect();
    state.captureSource = null;
    state.captureNode = null;
    state.playbackNode = null;
    if (state.audioContext) await state.audioContext.close().catch(() => {});
    state.audioContext = null;
    state.channel?.close();
    state.peer?.close();
    state.channel = null;
    state.peer = null;
    state.capturePending = new Float32Array(0);

    if (notifyServer && closingSession && closingCall && ui.apiKey.value.trim()) {
      await api(`/call/${encodeURIComponent(closingCall)}/webrtc/${encodeURIComponent(closingSession)}`, {
        method: "DELETE",
      }).catch(() => {});
    }
    if (closingCall) log("Áudio desconectado", { callId: closingCall, sent: state.sentFrames, received: state.receivedFrames, dropped: state.droppedFrames });
    render();
  }

  async function runAction(action) {
    if (state.loading) return;
    setBusy(true);
    try { await action(); } catch (error) { log("Operação falhou", error.message); }
    finally { setBusy(false); render(); }
  }

  function startPolling() {
    if (state.pollTimer) return;
    state.pollTimer = window.setInterval(() => refreshStatus({ quiet: true }), POLL_INTERVAL_MS);
  }

  function stopPolling() {
    if (!state.pollTimer) return;
    clearInterval(state.pollTimer);
    state.pollTimer = null;
  }

  function togglePanel(force) {
    state.open = force ?? !state.open;
    ui.panel.hidden = !state.open;
    ui.launcher.classList.toggle("active", state.open);
    if (state.open) {
      startPolling();
      refreshStatus({ quiet: false });
      setTimeout(() => ui.apiKey.value ? ui.number.focus() : ui.apiKey.focus(), 50);
    } else {
      stopPolling();
    }
  }

  ui.launcher.addEventListener("click", () => togglePanel());
  ui.close.addEventListener("click", () => togglePanel(false));
  ui.save.addEventListener("click", () => runAction(async () => {
    saveConfig();
    log("Configuração salva", { baseUrl: normalizedBaseURL(), persistent: ui.remember.checked });
    await refreshStatus();
  }));
  ui.refresh.addEventListener("click", () => runAction(() => refreshStatus()));
  ui.start.addEventListener("click", () => runAction(startCall));
  ui.accept.addEventListener("click", () => runAction(acceptCall));
  ui.reject.addEventListener("click", () => runAction(rejectCall));
  ui.connect.addEventListener("click", () => runAction(() => connectMedia(selectedCall()?.id)));
  ui.mute.addEventListener("click", () => {
    state.muted = !state.muted;
    log(state.muted ? "Microfone silenciado" : "Microfone ativado");
    render();
  });
  ui.hangup.addEventListener("click", () => runAction(terminateCall));
  ui.number.addEventListener("keydown", event => {
    if (event.key === "Enter") {
      event.preventDefault();
      runAction(startCall);
    }
  });
  ui.apiKey.addEventListener("keydown", event => {
    if (event.key === "Enter") {
      event.preventDefault();
      runAction(async () => { saveConfig(); await refreshStatus(); });
    }
  });
  window.addEventListener("beforeunload", () => {
    state.microphoneStream?.getTracks().forEach(track => track.stop());
    state.peer?.close();
  });

  loadConfig();
  render();
  if (ui.apiKey.value.trim()) refreshStatus({ quiet: true });
})();
