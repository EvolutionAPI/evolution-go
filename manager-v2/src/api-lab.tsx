import { useMemo, useState } from "react";
import { API_OPERATIONS, type ApiOperation, type BodyMode } from "./api-catalog";
import type { ApiAuthMode, ApiExecutionResult, EvolutionApi, EvolutionConnection } from "./api";
import { GuidedRequestEditor, supportsGuidedRequest, validateRequestDraft } from "./guided-request";
import { RequestPresetPanel, type RequestPresetDraft } from "./request-presets";

function replaceInstanceId(path: string, connection: EvolutionConnection): string {
  return path.replaceAll(":instanceId", connection.instanceId || "INSTANCE_ID");
}

function stringifySample(sample: unknown): string {
  return sample === undefined ? "" : JSON.stringify(sample, null, 2);
}

function appendFormValue(form: FormData, key: string, value: unknown): void {
  if (value === undefined || value === null) return;
  if (typeof value === "object") form.set(key, JSON.stringify(value));
  else form.set(key, String(value));
}

function responseText(result: ApiExecutionResult | null): string {
  if (!result) return "Execute uma operação para visualizar a resposta.";
  if (typeof result.data === "string") return result.data;
  return JSON.stringify(result.data, null, 2) ?? String(result.data);
}

function buildCurl(
  connection: EvolutionConnection,
  operationValue: Pick<ApiOperation, "auth" | "fileField">,
  method: string,
  bodyMode: BodyMode,
  path: string,
  body: string,
): string {
  const keyLabel = operationValue.auth === "admin" ? "SUA_CHAVE_GLOBAL" : operationValue.auth === "none" ? "" : "SUA_CHAVE_DA_INSTANCIA";
  const parts = [`curl -X ${method} '${connection.baseUrl}${path}'`];
  if (keyLabel) parts.push(`-H 'apikey: ${keyLabel}'`);
  if (bodyMode === "json" && body.trim()) {
    parts.push("-H 'Content-Type: application/json'");
    parts.push(`--data '${body.replaceAll("'", "'\\''")}'`);
  }
  if (bodyMode === "multipart") {
    try {
      const parsed = JSON.parse(body || "{}") as Record<string, unknown>;
      Object.entries(parsed).forEach(([key, value]) => parts.push(`-F '${key}=${typeof value === "object" ? JSON.stringify(value) : String(value)}'`));
    } catch {
      // Keep cURL visible while the JSON editor is incomplete.
    }
    parts.push(`-F '${operationValue.fileField || "file"}=@/caminho/arquivo'`);
  }
  return parts.join(" \\\n  ");
}

export function ApiLab({ api, connection }: { api: EvolutionApi | null; connection: EvolutionConnection }) {
  const initial = API_OPERATIONS.find((item) => item.id === "send-text") ?? API_OPERATIONS[0];
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("Todos");
  const [selectedId, setSelectedId] = useState(initial.id);
  const [method, setMethod] = useState(initial.method);
  const [path, setPath] = useState(() => replaceInstanceId(initial.path, connection));
  const [bodyMode, setBodyMode] = useState<BodyMode>(initial.bodyMode);
  const [body, setBody] = useState(() => stringifySample(initial.sample));
  const [auth, setAuth] = useState<ApiAuthMode>(initial.auth);
  const [file, setFile] = useState<File | null>(null);
  const [fileField, setFileField] = useState(initial.fileField || "file");
  const [editorMode, setEditorMode] = useState<"guided" | "json">(supportsGuidedRequest(initial.id) ? "guided" : "json");
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<ApiExecutionResult | null>(null);
  const [history, setHistory] = useState<Array<{ id: number; title: string; status: number; duration: number }>>([]);

  const selected = API_OPERATIONS.find((item) => item.id === selectedId) ?? initial;
  const guidedAvailable = supportsGuidedRequest(selected.id);
  const validation = useMemo(() => validateRequestDraft(selected.id, bodyMode, body, file), [body, bodyMode, file, selected.id]);
  const categories = useMemo(() => ["Todos", ...Array.from(new Set(API_OPERATIONS.map((item) => item.category)))], []);
  const visible = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase("pt-BR");
    return API_OPERATIONS.filter((item) => {
      if (category !== "Todos" && item.category !== category) return false;
      if (!normalized) return true;
      return `${item.title} ${item.path} ${item.description}`.toLocaleLowerCase("pt-BR").includes(normalized);
    });
  }, [category, query]);

  const choose = (item: ApiOperation) => {
    setSelectedId(item.id);
    setMethod(item.method);
    setPath(replaceInstanceId(item.path, connection));
    setBodyMode(item.bodyMode);
    setBody(stringifySample(item.sample));
    setAuth(item.auth);
    setFile(null);
    setFileField(item.fileField || "file");
    setEditorMode(supportsGuidedRequest(item.id) ? "guided" : "json");
    setError("");
    setResult(null);
  };

  const applyPreset = (preset: RequestPresetDraft) => {
    const operation = API_OPERATIONS.find((item) => item.id === preset.operationId);
    if (!operation) {
      setError(`A operação ${preset.operationId} não existe mais no catálogo.`);
      return;
    }
    setSelectedId(operation.id);
    setMethod(preset.method);
    setPath(preset.path);
    setBodyMode(preset.bodyMode);
    setBody(preset.body);
    setAuth(preset.auth);
    setFile(null);
    setFileField(preset.fileField || operation.fileField || "file");
    setEditorMode(supportsGuidedRequest(operation.id) ? "guided" : "json");
    setError("");
    setResult(null);
  };

  const resetPayload = () => {
    setBody(stringifySample(selected.sample));
    setFile(null);
    setFileField(selected.fileField || "file");
    setError("");
  };

  const formatPayload = () => {
    try {
      setBody(JSON.stringify(JSON.parse(body || "{}") as unknown, null, 2));
      setError("");
    } catch (cause) {
      setError(cause instanceof Error ? `JSON inválido: ${cause.message}` : "JSON inválido");
    }
  };

  const run = async () => {
    if (!api || running) return;
    if (validation.errors.length) {
      setError(validation.errors.join(" "));
      return;
    }
    setRunning(true);
    setError("");
    try {
      let requestBody: BodyInit | null | undefined;
      if (bodyMode === "json" && body.trim()) {
        requestBody = JSON.stringify(JSON.parse(body) as unknown);
      } else if (bodyMode === "multipart") {
        const form = new FormData();
        const values = JSON.parse(body || "{}") as Record<string, unknown>;
        Object.entries(values).forEach(([key, value]) => appendFormValue(form, key, value));
        if (file) form.set(fileField || "file", file, file.name);
        requestBody = form;
      }
      const response = await api.execute({ method, path, auth, body: requestBody });
      setResult(response);
      setHistory((current) => [{ id: Date.now(), title: selected.title, status: response.status, duration: response.durationMs }, ...current].slice(0, 20));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Falha ao executar a requisição");
    } finally {
      setRunning(false);
    }
  };

  const curl = buildCurl(connection, { auth, fileField }, method, bodyMode, path, body);
  const renderedResponse = responseText(result);
  const presetDraft: RequestPresetDraft = {
    operationId: selected.id,
    operationTitle: selected.title,
    method,
    path,
    auth,
    bodyMode,
    body,
    fileField,
  };

  return (
    <div className="api-lab-layout">
      <aside className="card api-catalog">
        <div className="section-heading"><div><span className="eyebrow">Catálogo completo</span><h2>{API_OPERATIONS.length} operações</h2></div></div>
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar rota ou função" />
        <div className="api-category-strip">
          {categories.map((item) => <button type="button" className={category === item ? "active" : ""} key={item} onClick={() => setCategory(item)}>{item}</button>)}
        </div>
        <div className="api-operation-list">
          {visible.map((item) => (
            <button type="button" className={item.id === selected.id ? "selected" : ""} key={item.id} onClick={() => choose(item)}>
              <span className={`http-method method-${item.method.toLowerCase()}`}>{item.method}</span>
              <span><strong>{item.title}</strong><small>{item.path}</small></span>
            </button>
          ))}
        </div>
      </aside>

      <section className="api-console">
        <section className="card api-request-card">
          <div className="section-heading">
            <div><span className="eyebrow">{selected.category}</span><h2>{selected.title}</h2></div>
            <span className={`http-method method-${method.toLowerCase()}`}>{method}</span>
          </div>
          <p className="api-description">{selected.description}</p>
          <div className="api-route-row editable">
            <select value={method} onChange={(event) => setMethod(event.target.value)} aria-label="Método HTTP">
              {(["GET", "POST", "PUT", "PATCH", "DELETE"] as const).map((item) => <option key={item}>{item}</option>)}
            </select>
            <input value={path} onChange={(event) => setPath(event.target.value)} aria-label="Caminho da API" />
            <select value={auth} onChange={(event) => setAuth(event.target.value as ApiAuthMode)} aria-label="Tipo de autenticação">
              <option value="instance">Chave da instância</option>
              <option value="admin">Chave global</option>
              <option value="none">Sem autenticação</option>
            </select>
            <select value={bodyMode} onChange={(event) => setBodyMode(event.target.value as BodyMode)} aria-label="Formato do corpo">
              <option value="none">Sem corpo</option>
              <option value="json">JSON</option>
              <option value="multipart">Multipart</option>
            </select>
          </div>

          {bodyMode !== "none" && (
            <>
              <div className="request-mode-row">
                {guidedAvailable && <button type="button" className={editorMode === "guided" ? "active" : ""} onClick={() => setEditorMode("guided")}>Formulário guiado</button>}
                <button type="button" className={editorMode === "json" ? "active" : ""} onClick={() => setEditorMode("json")}>JSON avançado</button>
                <span>{guidedAvailable ? "Os dois modos usam o mesmo payload." : "Esta operação usa o editor JSON livre."}</span>
              </div>

              {guidedAvailable && editorMode === "guided" ? (
                <GuidedRequestEditor operationId={selected.id} body={body} onChange={setBody} />
              ) : (
                <label className="api-editor-label">
                  <span>{bodyMode === "multipart" ? "Campos multipart em JSON" : "Corpo JSON"}</span>
                  <div className="api-inline-actions">
                    <button type="button" onClick={formatPayload}>Formatar JSON</button>
                    <button type="button" onClick={resetPayload}>Restaurar exemplo</button>
                  </div>
                  <textarea value={body} onChange={(event) => setBody(event.target.value)} spellCheck={false} rows={16} />
                </label>
              )}

              <div className="request-validation">
                {validation.errors.map((item) => <div className="validation-error" key={item}>Erro: {item}</div>)}
                {validation.warnings.map((item) => <div className="validation-warning" key={item}>Atenção: {item}</div>)}
              </div>
            </>
          )}
          {bodyMode === "multipart" && (
            <label className="api-file-field">
              <span>Arquivo</span>
              <div className="multipart-file-row">
                <input value={fileField} onChange={(event) => setFileField(event.target.value)} placeholder="Nome do campo: file" />
                <input type="file" onChange={(event) => setFile(event.target.files?.[0] ?? null)} />
              </div>
              <small>{file ? `${file.name} · ${Math.ceil(file.size / 1024)} KB` : "Selecione um arquivo quando a rota exigir upload."}</small>
            </label>
          )}

          {error && <div className="alert error">{error}</div>}
          <div className="api-actions">
            <button type="button" className="button secondary" onClick={resetPayload}>Restaurar exemplo</button>
            <button type="button" className="button secondary" onClick={() => void navigator.clipboard.writeText(curl)}>Copiar cURL</button>
            <button type="button" className="button primary" disabled={!api || running || validation.errors.length > 0} onClick={() => void run()}>{running ? "Executando…" : "Executar teste"}</button>
          </div>
        </section>

        <section className="card api-response-card">
          <div className="section-heading">
            <div><span className="eyebrow">Resposta</span><h2>{result ? `${result.status} ${result.statusText}` : "Aguardando execução"}</h2></div>
            <div className="api-inline-actions">
              {result && <button type="button" onClick={() => void navigator.clipboard.writeText(renderedResponse)}>Copiar resposta</button>}
              {result && <span className={`response-status ${result.ok ? "ok" : "failed"}`}>{result.durationMs} ms</span>}
            </div>
          </div>
          {result && <div className="api-response-meta"><span>{result.url}</span><span>{result.ok ? "Sucesso" : "Erro HTTP"}</span></div>}
          <pre>{renderedResponse}</pre>
        </section>

        <section className="card api-curl-card">
          <div className="section-heading"><div><span className="eyebrow">Reprodução</span><h2>cURL equivalente</h2></div></div>
          <pre>{curl}</pre>
        </section>
      </section>

      <aside className="card api-history">
        <RequestPresetPanel current={presetDraft} onApply={applyPreset} />
        <div className="section-heading"><div><span className="eyebrow">Sessão</span><h2>Últimos testes</h2></div></div>
        {history.length === 0 ? <p>Nenhuma requisição executada.</p> : history.map((item) => (
          <div className="api-history-item" key={item.id}>
            <strong>{item.title}</strong>
            <span className={item.status >= 200 && item.status < 300 ? "ok" : "failed"}>{item.status}</span>
            <small>{item.duration} ms</small>
          </div>
        ))}
      </aside>
    </div>
  );
}
