import type { EvolutionApi } from "./api";

const DATA_CHANNEL_LABEL = "evolution-call-pcm";
const DATA_CHANNEL_PROTOCOL = "evcall.pcm.v1";
const PCM_RATE = 16000;
const PCM_FRAME_SAMPLES = 960;
const HEADER_BYTES = 16;
const MAX_BUFFERED_AMOUNT = 256 * 1024;

export interface MediaStats {
  sent: number;
  received: number;
  dropped: number;
}

export type MediaStatus = "idle" | "connecting" | "connected" | "failed";

interface BridgeCallbacks {
  onStatus: (status: MediaStatus) => void;
  onStats: (stats: MediaStats) => void;
  onLog: (message: string, details?: unknown) => void;
}

class StreamingLinearResampler {
  private readonly step: number;
  private position = 0;
  private carry = new Float32Array(0);

  constructor(inputRate: number, outputRate: number) {
    this.step = inputRate / outputRate;
  }

  push(input: Float32Array): Float32Array {
    if (!input.length) return new Float32Array(0);
    const data = new Float32Array(this.carry.length + input.length);
    data.set(this.carry);
    data.set(input, this.carry.length);
    const output: number[] = [];
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

function encodePCM(samples: Float32Array): ArrayBuffer {
  const buffer = new ArrayBuffer(HEADER_BYTES + samples.length * 4);
  const bytes = new Uint8Array(buffer);
  bytes.set([0x45, 0x56, 0x50, 0x43]);
  const view = new DataView(buffer);
  view.setUint8(4, 1);
  view.setUint8(5, 1);
  view.setUint16(6, 0, true);
  view.setUint32(8, PCM_RATE, true);
  view.setUint32(12, samples.length, true);
  samples.forEach((value, index) => {
    const sample = Number.isFinite(value) ? Math.max(-1, Math.min(1, value)) : 0;
    view.setFloat32(HEADER_BYTES + index * 4, sample, true);
  });
  return buffer;
}

function decodePCM(buffer: ArrayBuffer): Float32Array {
  if (buffer.byteLength < HEADER_BYTES) throw new Error("Frame PCM truncado");
  const bytes = new Uint8Array(buffer, 0, 4);
  if (bytes[0] !== 0x45 || bytes[1] !== 0x56 || bytes[2] !== 0x50 || bytes[3] !== 0x43) {
    throw new Error("Cabeçalho PCM inválido");
  }
  const view = new DataView(buffer);
  if (view.getUint8(4) !== 1 || view.getUint8(5) !== 1 || view.getUint16(6, true) !== 0) {
    throw new Error("Versão PCM incompatível");
  }
  if (view.getUint32(8, true) !== PCM_RATE) throw new Error("Sample rate PCM incompatível");
  const count = view.getUint32(12, true);
  if (!count || count > PCM_FRAME_SAMPLES * 4 || buffer.byteLength !== HEADER_BYTES + count * 4) {
    throw new Error("Tamanho PCM inválido");
  }
  const output = new Float32Array(count);
  for (let index = 0; index < count; index++) {
    output[index] = view.getFloat32(HEADER_BYTES + index * 4, true);
  }
  return output;
}

async function installWorklet(context: AudioContext): Promise<void> {
  const source = `
    class EvolutionManagerV2PCM extends AudioWorkletProcessor {
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
    registerProcessor('evolution-manager-v2-pcm', EvolutionManagerV2PCM);
  `;
  const url = URL.createObjectURL(new Blob([source], { type: "text/javascript" }));
  try {
    await context.audioWorklet.addModule(url);
  } finally {
    URL.revokeObjectURL(url);
  }
}

async function gatherICE(connection: RTCPeerConnection): Promise<void> {
  if (connection.iceGatheringState === "complete") return;
  await new Promise<void>((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      connection.removeEventListener("icegatheringstatechange", listener);
      reject(new Error("Timeout ao coletar candidatos ICE"));
    }, 15000);
    const listener = () => {
      if (connection.iceGatheringState === "complete") {
        window.clearTimeout(timeout);
        connection.removeEventListener("icegatheringstatechange", listener);
        resolve();
      }
    };
    connection.addEventListener("icegatheringstatechange", listener);
  });
}

export class EvolutionPcmBridge {
  private peer: RTCPeerConnection | null = null;
  private channel: RTCDataChannel | null = null;
  private sessionId = "";
  private callId = "";
  private audioContext: AudioContext | null = null;
  private microphone: MediaStream | null = null;
  private captureSource: MediaStreamAudioSourceNode | null = null;
  private captureNode: AudioWorkletNode | null = null;
  private playbackNode: AudioWorkletNode | null = null;
  private captureResampler: StreamingLinearResampler | null = null;
  private playbackResampler: StreamingLinearResampler | null = null;
  private pending = new Float32Array(0);
  private muted = false;
  private stats: MediaStats = { sent: 0, received: 0, dropped: 0 };

  constructor(
    private readonly api: EvolutionApi,
    private readonly callbacks: BridgeCallbacks,
  ) {}

  get activeCallId(): string {
    return this.callId;
  }

  get isMuted(): boolean {
    return this.muted;
  }

  setMuted(value: boolean): void {
    this.muted = value;
    this.callbacks.onLog(value ? "Microfone silenciado" : "Microfone ativado");
  }

  private publishStats(): void {
    this.callbacks.onStats({ ...this.stats });
  }

  private appendCapture(samples: Float32Array): void {
    const joined = new Float32Array(this.pending.length + samples.length);
    joined.set(this.pending);
    joined.set(samples, this.pending.length);
    let offset = 0;
    while (joined.length - offset >= PCM_FRAME_SAMPLES) {
      const frame = joined.slice(offset, offset + PCM_FRAME_SAMPLES);
      offset += PCM_FRAME_SAMPLES;
      if (this.muted) continue;
      if (this.channel?.readyState === "open" && this.channel.bufferedAmount <= MAX_BUFFERED_AMOUNT) {
        this.channel.send(encodePCM(frame));
        this.stats.sent++;
      } else {
        this.stats.dropped++;
      }
    }
    this.pending = joined.slice(offset);
    this.publishStats();
  }

  private async startAudio(): Promise<void> {
    if (!window.AudioContext || !window.AudioWorkletNode) {
      throw new Error("Este navegador não suporta AudioWorklet");
    }
    this.audioContext = new AudioContext({ latencyHint: "interactive" });
    await installWorklet(this.audioContext);
    await this.audioContext.resume();
    this.captureResampler = new StreamingLinearResampler(this.audioContext.sampleRate, PCM_RATE);
    this.playbackResampler = new StreamingLinearResampler(PCM_RATE, this.audioContext.sampleRate);

    this.playbackNode = new AudioWorkletNode(this.audioContext, "evolution-manager-v2-pcm", {
      numberOfInputs: 0,
      numberOfOutputs: 1,
      outputChannelCount: [1],
      processorOptions: { mode: "playback" },
    });
    this.playbackNode.connect(this.audioContext.destination);

    this.microphone = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true,
      },
      video: false,
    });
    this.captureSource = this.audioContext.createMediaStreamSource(this.microphone);
    this.captureNode = new AudioWorkletNode(this.audioContext, "evolution-manager-v2-pcm", {
      numberOfInputs: 1,
      numberOfOutputs: 0,
      processorOptions: { mode: "capture" },
    });
    this.captureNode.port.onmessage = (event: MessageEvent<Float32Array>) => {
      const resampled = this.captureResampler?.push(event.data) ?? new Float32Array(0);
      this.appendCapture(resampled);
    };
    this.captureSource.connect(this.captureNode);
    this.callbacks.onLog("Microfone e reprodução iniciados", { sampleRate: this.audioContext.sampleRate });
  }

  async connect(callId: string): Promise<void> {
    if (!window.isSecureContext && location.hostname !== "localhost") {
      throw new Error("O microfone exige HTTPS");
    }
    if (this.peer) await this.disconnect();
    this.callbacks.onStatus("connecting");
    this.callId = callId;
    this.stats = { sent: 0, received: 0, dropped: 0 };
    this.pending = new Float32Array(0);
    this.publishStats();

    const peer = new RTCPeerConnection({ iceServers: [] });
    const channel = peer.createDataChannel(DATA_CHANNEL_LABEL, {
      ordered: true,
      protocol: DATA_CHANNEL_PROTOCOL,
    });
    channel.binaryType = "arraybuffer";
    channel.bufferedAmountLowThreshold = MAX_BUFFERED_AMOUNT / 2;
    this.peer = peer;
    this.channel = channel;

    channel.onopen = () => {
      void this.startAudio()
        .then(() => this.callbacks.onStatus("connected"))
        .catch(async (cause) => {
          this.callbacks.onLog("Falha ao iniciar áudio", cause instanceof Error ? cause.message : cause);
          this.callbacks.onStatus("failed");
          await this.disconnect();
        });
    };
    channel.onmessage = (event: MessageEvent<ArrayBuffer>) => {
      try {
        const pcm = decodePCM(event.data);
        const playback = this.playbackResampler?.push(pcm) ?? new Float32Array(0);
        if (playback.length) this.playbackNode?.port.postMessage(playback, [playback.buffer]);
        this.stats.received++;
      } catch (cause) {
        this.stats.dropped++;
        this.callbacks.onLog("Frame de áudio rejeitado", cause instanceof Error ? cause.message : cause);
      }
      this.publishStats();
    };
    channel.onerror = () => this.callbacks.onLog("Erro no DataChannel de áudio");
    channel.onclose = () => {
      if (this.callId === callId) void this.disconnect(false);
    };
    peer.onconnectionstatechange = () => {
      this.callbacks.onLog("PeerConnection", peer.connectionState);
      if (["failed", "closed"].includes(peer.connectionState)) void this.disconnect(false);
    };

    try {
      await peer.setLocalDescription(await peer.createOffer());
      await gatherICE(peer);
      if (!peer.localDescription) throw new Error("Oferta WebRTC não foi criada");
      const response = await this.api.createWebRTC(callId, {
        type: "offer",
        sdp: peer.localDescription.sdp,
      });
      this.sessionId = response.sessionId;
      await peer.setRemoteDescription(response.answer);
      this.callbacks.onLog("Sessão WebRTC criada", { callId, sessionId: response.sessionId });
    } catch (cause) {
      await this.disconnect(false);
      this.callbacks.onStatus("failed");
      throw cause;
    }
  }

  async disconnect(notifyServer = true): Promise<void> {
    const callId = this.callId;
    const sessionId = this.sessionId;
    this.callId = "";
    this.sessionId = "";
    this.microphone?.getTracks().forEach((track) => track.stop());
    this.microphone = null;
    this.captureSource?.disconnect();
    this.captureNode?.disconnect();
    this.playbackNode?.disconnect();
    this.captureSource = null;
    this.captureNode = null;
    this.playbackNode = null;
    if (this.audioContext) await this.audioContext.close().catch(() => undefined);
    this.audioContext = null;
    this.channel?.close();
    this.peer?.close();
    this.channel = null;
    this.peer = null;
    this.captureResampler = null;
    this.playbackResampler = null;
    this.pending = new Float32Array(0);
    this.callbacks.onStatus("idle");
    if (notifyServer && callId && sessionId) {
      await this.api.closeWebRTC(callId, sessionId).catch(() => undefined);
    }
    if (callId) this.callbacks.onLog("Áudio desconectado", { callId, ...this.stats });
  }
}
