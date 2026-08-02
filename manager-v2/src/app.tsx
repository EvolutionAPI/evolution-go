import { useMemo, useState } from "react";
import { ApiLab } from "./api-lab";
import { loadConnection, saveConnection, type EvolutionConnection } from "./api";
import { CallWorkspace } from "./call-workspace";
import { ConnectionEditor, InstanceWorkspace } from "./instance";

type View = "instance" | "api" | "calls" | "settings";

const NAV_ITEMS: Array<{ id: View; icon: string; label: string }> = [
  { id: "instance", icon: "◫", label: "Instância" },
  { id: "api", icon: "⌘", label: "API Lab" },
  { id: "calls", icon: "☎", label: "Chamadas" },
  { id: "settings", icon: "⚙", label: "Configuração" },
];

function connectionHost(value: string): string {
  try {
    return new URL(value).host;
  } catch {
    return value || "URL não configurada";
  }
}

export function App() {
  const [view, setView] = useState<View>("instance");
  const [connection, setConnection] = useState(loadConnection);
  const api = useMemo(
    () => connection.apiKey || connection.adminApiKey ? new (requireApi())(connection) : null,
    [connection],
  );

  const updateConnection = (next: EvolutionConnection) => {
    saveConnection(next);
    setConnection(next);
  };

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">E</span>
          <div><strong>Evolution GO</strong><small>API Test Manager</small></div>
        </div>
        <nav>
          {NAV_ITEMS.map((item) => (
            <button key={item.id} className={view === item.id ? "active" : ""} onClick={() => setView(item.id)}>
              <span>{item.icon}</span>{item.label}
            </button>
          ))}
        </nav>
        <div className="sidebar-foot">
          <span className={`status-dot ${connection.apiKey ? "online" : "offline"}`} />
          <div>
            <strong>{connection.instanceId || (connection.apiKey ? "Instância configurada" : "Sem instância")}</strong>
            <small>{connectionHost(connection.baseUrl)}</small>
          </div>
        </div>
      </aside>

      <main>
        <header className="topbar">
          <div><span className="breadcrumb">Manager V2 /</span><strong>{NAV_ITEMS.find((item) => item.id === view)?.label}</strong></div>
          <div className="top-actions">
            <span className={`connection-pill ${connection.apiKey ? "connected" : "disconnected"}`}>
              {connection.apiKey ? "Chave da instância salva" : "Configuração necessária"}
            </span>
            <button className="profile-button" title="API Test Manager">API</button>
          </div>
        </header>

        <div className="content">
          {view === "instance" && <InstanceWorkspace api={api} connection={connection} onSave={updateConnection} />}
          {view === "api" && <ApiLab api={api} connection={connection} />}
          {view === "calls" && <CallWorkspace api={api} />}
          {view === "settings" && <ConnectionEditor value={connection} onSave={updateConnection} />}

          {!connection.apiKey && !["instance", "settings"].includes(view) && (
            <div className="setup-overlay">
              <ConnectionEditor value={connection} onSave={updateConnection} compact />
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

function requireApi() {
  // Kept behind a function so the memo only constructs the client when a key exists.
  // Static import avoids code splitting and keeps TypeScript inference intact.
  return EvolutionApiConstructor;
}

import { EvolutionApi as EvolutionApiConstructor } from "./api";
