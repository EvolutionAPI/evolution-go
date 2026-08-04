import { useMemo, useState } from "react";
import { ApiLab } from "./api-lab";
import { EvolutionApi, loadConnection, saveConnection, type EvolutionConnection } from "./api";
import { CallWorkspace } from "./call-workspace";
import { ConnectionEditor, InstanceWorkspace } from "./instance";

type View = "instances" | "api" | "calls" | "settings";

const NAV_ITEMS: Array<{ id: View; marker: string; label: string }> = [
  { id: "instances", marker: "I", label: "Instâncias" },
  { id: "api", marker: "A", label: "API Lab" },
  { id: "calls", marker: "C", label: "Chamadas" },
  { id: "settings", marker: "S", label: "Configurações" },
];

function connectionHost(value: string): string {
  try {
    return new URL(value).host;
  } catch {
    return value || "URL não configurada";
  }
}

export function App() {
  const [view, setView] = useState<View>("instances");
  const [connection, setConnection] = useState(loadConnection);
  const hasAdminKey = Boolean(connection.adminApiKey.trim());
  const hasInstanceKey = Boolean(connection.apiKey.trim());
  const api = useMemo(() => hasAdminKey || hasInstanceKey ? new EvolutionApi(connection) : null, [connection, hasAdminKey, hasInstanceKey]);

  const updateConnection = (next: EvolutionConnection) => {
    saveConnection(next);
    setConnection(next);
  };

  const connectionLabel = hasAdminKey
    ? "Acesso administrativo"
    : hasInstanceKey
      ? "Acesso à instância"
      : "Configuração necessária";

  const currentLabel = NAV_ITEMS.find((item) => item.id === view)?.label;

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">E</span>
          <div><strong>Evolution GO</strong><small>Manager</small></div>
        </div>
        <nav aria-label="Navegação principal">
          {NAV_ITEMS.map((item) => (
            <button key={item.id} className={view === item.id ? "active" : ""} onClick={() => setView(item.id)}>
              <span className="nav-marker" aria-hidden="true">{item.marker}</span>{item.label}
            </button>
          ))}
        </nav>
        <div className="sidebar-foot">
          <span className={`status-dot ${hasAdminKey || hasInstanceKey ? "online" : "offline"}`} />
          <div>
            <strong>{connection.instanceId || (hasAdminKey ? "Administrador" : "Sem acesso")}</strong>
            <small>{connectionHost(connection.baseUrl)}</small>
          </div>
        </div>
      </aside>

      <main>
        <header className="topbar">
          <div><span className="breadcrumb">Manager /</span><strong>{currentLabel}</strong></div>
          <div className="top-actions">
            <span className={`connection-pill ${hasAdminKey || hasInstanceKey ? "connected" : "disconnected"}`}>{connectionLabel}</span>
            {view !== "settings" && <button type="button" className="settings-shortcut" onClick={() => setView("settings")}>Configurações</button>}
          </div>
        </header>

        <div className="content">
          {view === "instances" && <InstanceWorkspace api={api} connection={connection} onSave={updateConnection} onOpenSettings={() => setView("settings")} />}
          {view === "api" && <ApiLab api={api} connection={connection} />}
          {view === "calls" && <CallWorkspace api={api} />}
          {view === "settings" && (
            <div className="settings-workspace">
              <section className="settings-intro">
                <span className="eyebrow">Manager</span>
                <h1>Configurações</h1>
                <p>Defina o acesso da API para gerenciar suas instâncias com segurança.</p>
              </section>
              <ConnectionEditor value={connection} onSave={updateConnection} />
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
