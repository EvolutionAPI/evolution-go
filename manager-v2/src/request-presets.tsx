import { useMemo, useRef, useState } from "react";
import type { ApiAuthMode } from "./api";
import type { BodyMode } from "./api-catalog";
import "./request-presets.css";

export type RequestPresetDraft = {
  operationId: string;
  operationTitle: string;
  method: string;
  path: string;
  auth: ApiAuthMode;
  bodyMode: BodyMode;
  body: string;
  fileField: string;
};

type RequestPreset = RequestPresetDraft & {
  id: string;
  name: string;
  updatedAt: string;
};

type RequestPresetPanelProps = {
  current: RequestPresetDraft;
  onApply: (preset: RequestPresetDraft) => void;
  showHeading?: boolean;
};

const STORAGE_KEY = "evolution-go.manager-v2.request-presets.v1";

function isAuthMode(value: unknown): value is ApiAuthMode {
  return value === "instance" || value === "admin" || value === "none";
}

function isBodyMode(value: unknown): value is BodyMode {
  return value === "none" || value === "json" || value === "multipart";
}

function normalizePreset(value: unknown): RequestPreset | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const item = value as Record<string, unknown>;
  if (
    typeof item.id !== "string" ||
    typeof item.name !== "string" ||
    typeof item.operationId !== "string" ||
    typeof item.operationTitle !== "string" ||
    typeof item.method !== "string" ||
    typeof item.path !== "string" ||
    !isAuthMode(item.auth) ||
    !isBodyMode(item.bodyMode) ||
    typeof item.body !== "string" ||
    typeof item.fileField !== "string" ||
    typeof item.updatedAt !== "string"
  ) return null;

  return {
    id: item.id,
    name: item.name,
    operationId: item.operationId,
    operationTitle: item.operationTitle,
    method: item.method,
    path: item.path,
    auth: item.auth,
    bodyMode: item.bodyMode,
    body: item.body,
    fileField: item.fileField,
    updatedAt: item.updatedAt,
  };
}

function loadPresets(): RequestPreset[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.map(normalizePreset).filter((item): item is RequestPreset => item !== null);
  } catch {
    return [];
  }
}

function persistPresets(presets: RequestPreset[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(presets));
}

function downloadPresets(presets: RequestPreset[]): void {
  const payload = JSON.stringify({ version: 1, exportedAt: new Date().toISOString(), presets }, null, 2);
  const url = URL.createObjectURL(new Blob([payload], { type: "application/json" }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `evolution-go-presets-${new Date().toISOString().slice(0, 10)}.json`;
  anchor.click();
  URL.revokeObjectURL(url);
}

function importedPresets(value: unknown): RequestPreset[] {
  const root = value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
  const source = Array.isArray(value) ? value : root.presets;
  if (!Array.isArray(source)) throw new Error("O arquivo não contém uma lista de presets.");
  const normalized = source.map(normalizePreset).filter((item): item is RequestPreset => item !== null);
  if (!normalized.length && source.length) throw new Error("Nenhum preset válido foi encontrado.");
  return normalized;
}

export function RequestPresetPanel({ current, onApply, showHeading = true }: RequestPresetPanelProps) {
  const [presets, setPresets] = useState<RequestPreset[]>(loadPresets);
  const [name, setName] = useState("");
  const [message, setMessage] = useState("");
  const fileInput = useRef<HTMLInputElement | null>(null);

  const operationPresets = useMemo(
    () => presets.filter((item) => item.operationId === current.operationId),
    [current.operationId, presets],
  );

  const commit = (next: RequestPreset[]) => {
    setPresets(next);
    persistPresets(next);
  };

  const save = () => {
    const trimmed = name.trim() || `${current.operationTitle} ${operationPresets.length + 1}`;
    const now = new Date().toISOString();
    const existing = presets.find((item) => item.operationId === current.operationId && item.name.toLocaleLowerCase("pt-BR") === trimmed.toLocaleLowerCase("pt-BR"));
    const preset: RequestPreset = {
      ...current,
      id: existing?.id ?? `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      name: trimmed,
      updatedAt: now,
    };
    const next = [preset, ...presets.filter((item) => item.id !== preset.id)].slice(0, 100);
    commit(next);
    setName("");
    setMessage(existing ? "Preset atualizado." : "Preset salvo.");
  };

  const remove = (id: string) => {
    commit(presets.filter((item) => item.id !== id));
    setMessage("Preset removido.");
  };

  const importFile = async (file: File | null) => {
    if (!file) return;
    try {
      const parsed = JSON.parse(await file.text()) as unknown;
      const incoming = importedPresets(parsed);
      const merged = [...incoming, ...presets].reduce<RequestPreset[]>((items, item) => {
        if (items.some((existing) => existing.id === item.id)) return items;
        items.push(item);
        return items;
      }, []).slice(0, 100);
      commit(merged);
      setMessage(`${incoming.length} preset(s) importado(s).`);
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "Falha ao importar presets.");
    } finally {
      if (fileInput.current) fileInput.current.value = "";
    }
  };

  return (
    <section className="request-presets">
      {showHeading && (
        <div className="request-presets-heading">
          <div>
            <span className="eyebrow">Coleção local</span>
            <h3>Presets de teste</h3>
          </div>
          <span>{presets.length}/100</span>
        </div>
      )}
      <p>Salva rota e payload sem armazenar chaves ou arquivos.</p>
      <div className="request-preset-save">
        <input value={name} onChange={(event) => setName(event.target.value)} placeholder="Nome do preset" />
        <button type="button" onClick={save}>Salvar atual</button>
      </div>
      <div className="request-preset-tools">
        <button type="button" disabled={!presets.length} onClick={() => downloadPresets(presets)}>Exportar</button>
        <button type="button" onClick={() => fileInput.current?.click()}>Importar</button>
        <input ref={fileInput} type="file" accept="application/json,.json" hidden onChange={(event) => void importFile(event.target.files?.[0] ?? null)} />
      </div>
      {message && <small className="request-preset-message">{message}</small>}
      <div className="request-preset-list">
        {operationPresets.length === 0 ? <small>Nenhum preset salvo para esta operação.</small> : operationPresets.map((item) => (
          <div className="request-preset-item" key={item.id}>
            <button type="button" className="request-preset-apply" onClick={() => onApply(item)}>
              <strong>{item.name}</strong>
              <small>{item.method} · {item.bodyMode} · {new Date(item.updatedAt).toLocaleDateString("pt-BR")}</small>
            </button>
            <button type="button" className="request-preset-delete" aria-label={`Excluir ${item.name}`} onClick={() => remove(item.id)}>×</button>
          </div>
        ))}
      </div>
    </section>
  );
}
