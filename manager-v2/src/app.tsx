import { useEffect, useMemo, useState } from "react";
import { ApiLab } from "./api-lab";
import { clearConnection, EvolutionApi, loadConnection, saveConnection, type EvolutionConnection } from "./api";
import { loadManagerSession, logoutManager, ManagerAuthScreen, type ManagerSession, type ManagerUser } from "./auth";
import { CallWorkspace } from "./call-workspace";
import { InstanceSettingsWorkspace, InstanceWorkspace } from "./instance";
import { InfrastructureSettingsPanel } from "./infrastructure-settings";

type View = "instances" | "api" | "calls" | "settings" | "instance-settings";
type ManagerRoute = { view: Exclude<View, "instance-settings"> } | { view: "instance-settings"; instanceId: string };

const NAV_ITEMS: Array<{ id: Exclude<View, "instance-settings">; marker: string; label: string }> = [
  { id: "instances", marker: "I", label: "Instâncias" },
  { id: "api", marker: "A", label: "API Lab" },
  { id: "calls", marker: "C", label: "Chamadas" },
  { id: "settings", marker: "S", label: "Configurações" },
];

type AuthState = ManagerSession & { checking: boolean };

function connectionHost(value: string): string {
  try {
    return new URL(value).host;
  } catch {
    return value || "URL não configurada";
  }
}

function readRoute(): ManagerRoute {
  const path = window.location.pathname.replace(/\/+$/, "") || "/";
  const match = path.match(/^\/manager-v2\/instances\/([^/]+)\/settings$/);
  if (match) return { view: "instance-settings", instanceId: decodeURIComponent(match[1]) };
  return { view: "instances" };
}

function routePath(route: ManagerRoute): string {
  if (route.view === "instance-settings") return `/manager-v2/instances/${encodeURIComponent(route.instanceId)}/settings`;
  return "/manager-v2/";
}

export function App() {
  const [route, setRoute] = useState<ManagerRoute>(readRoute);
  const [connection, setConnection] = useState(loadConnection);
  const [auth, setAuth] = useState<AuthState>({ checking: true, authenticated: false, setupRequired: false });
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const view = route.view;
  const hasInstanceKey = Boolean(connection.apiKey.trim());
  const api = useMemo(() => new EvolutionApi(connection), [connection]);

  useEffect(() => {
    void loadManagerSession()
      .then((session) => setAuth({ ...session, checking: false }))
      .catch(() => setAuth({ checking: false, authenticated: false, setupRequired: false }));
  }, []);

  useEffect(() => {
    const updateRoute = () => setRoute(readRoute());
    window.addEventListener("popstate", updateRoute);
    return () => window.removeEventListener("popstate", updateRoute);
  }, []);

  const navigate = (nextRoute: ManagerRoute) => {
    const nextPath = routePath(nextRoute);
    if (window.location.pathname !== nextPath) window.history.pushState({}, "", nextPath);
    setRoute(nextRoute);
  };

  const updateConnection = (next: EvolutionConnection) => {
    saveConnection(next);
    setConnection(next);
  };

  const authenticated = (user: ManagerUser) => {
    setAuth({ checking: false, authenticated: true, setupRequired: false, user });
  };

  const signOut = async () => {
    if (isLoggingOut) return;
    setIsLoggingOut(true);
    try {
      await logoutManager();
    } finally {
      clearConnection();
      setConnection(loadConnection());
      setAuth({ checking: false, authenticated: false, setupRequired: false });
      setIsLoggingOut(false);
    }
  };

  if (auth.checking) {
    return <main className="auth-page"><div className="auth-loading">Verificando acesso…</div></main>;
  }

  if (!auth.authenticated || !auth.user) {
    return <ManagerAuthScreen setupRequired={auth.setupRequired} onAuthenticated={authenticated} />;
  }

  const currentLabel = view === "instance-settings"
    ? "Configurações da instância"
    : NAV_ITEMS.find((item) => item.id === view)?.label;
  const activeNavigation = view === "instance-settings" ? "instances" : view;

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">E</span>
          <div><strong>Evolution GO</strong><small>Manager</small></div>
        </div>
        <nav aria-label="Navegação principal">
          {NAV_ITEMS.map((item) => (
            <button key={item.id} className={activeNavigation === item.id ? "active" : ""} onClick={() => navigate({ view: item.id })}>
              <span className="nav-marker" aria-hidden="true">{item.marker}</span>{item.label}
            </button>
          ))}
        </nav>
        <div className="sidebar-foot">
          <span className="status-dot online" />
          <div>
            <strong>{auth.user.name}</strong>
            <small>{connectionHost(connection.baseUrl)}</small>
          </div>
        </div>
      </aside>

      <main>
        <header className="topbar">
          <div><span className="breadcrumb">Manager /</span><strong>{currentLabel}</strong></div>
          <div className="top-actions">
            <span className={`connection-pill ${hasInstanceKey ? "connected" : "disconnected"}`}>{hasInstanceKey ? "Instância configurada" : "Sessão administrativa"}</span>
            {view !== "settings" && <button type="button" className="settings-shortcut" onClick={() => navigate({ view: "settings" })}>Configurações</button>}
            <button type="button" className="logout-button" onClick={() => void signOut()} disabled={isLoggingOut}>{isLoggingOut ? "Saindo…" : "Sair"}</button>
          </div>
        </header>

        <div className="content">
          {view === "instances" && <InstanceWorkspace api={api} connection={connection} onSave={updateConnection} onOpenSettings={() => navigate({ view: "settings" })} onOpenInstanceSettings={(instanceId) => navigate({ view: "instance-settings", instanceId })} canManage />}
          {view === "instance-settings" && <InstanceSettingsWorkspace api={api} connection={connection} onSave={updateConnection} instanceId={route.instanceId} onBack={() => navigate({ view: "instances" })} />}
          {view === "api" && <ApiLab api={api} connection={connection} />}
          {view === "calls" && <CallWorkspace api={api} connection={connection} onSelectInstance={updateConnection} />}
          {view === "settings" && (
            <div className="settings-workspace">
              <section className="card settings-hero">
                <div className="settings-hero-copy">
                  <span className="eyebrow">Manager</span>
                  <h1>Configurações</h1>
                  <p>Configure somente os serviços compartilhados do servidor. Cada instância possui configurações e token próprios.</p>
                </div>
                <dl className="settings-hero-summary">
                  <div><dt>Administrador</dt><dd title={auth.user.name}>{auth.user.name}</dd></div>
                  <div><dt>Conta</dt><dd title={auth.user.email}>{auth.user.email}</dd></div>
                  <div><dt>Servidor API</dt><dd title={connectionHost(connection.baseUrl)}>{connectionHost(connection.baseUrl)}</dd></div>
                  <div><dt>Modo</dt><dd>Multi-instâncias</dd></div>
                </dl>
              </section>
              <section className="settings-section">
                <div className="settings-section-heading"><div><h2>Infraestrutura do servidor</h2><p>Integrações globais, rede e armazenamento. Configure cada instância separadamente pela tela de Instâncias.</p></div></div>
                <InfrastructureSettingsPanel api={api} />
              </section>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
