import { useState } from "react";

export interface ManagerUser {
  id: string;
  name: string;
  email: string;
}

export interface ManagerSession {
  authenticated: boolean;
  setupRequired: boolean;
  user?: ManagerUser;
}

type AuthResponse = { user: ManagerUser };

async function request<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(`/manager-v2/auth/${path}`, {
    method: body === undefined ? "GET" : "POST",
    credentials: "include",
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null) as { error?: string } | null;
    throw new Error(payload?.error || "Não foi possível concluir a solicitação.");
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export function loadManagerSession(): Promise<ManagerSession> {
  return request<ManagerSession>("status");
}

export async function logoutManager(): Promise<void> {
  await request<void>("logout", {});
}

export function ManagerAuthScreen({
  setupRequired,
  onAuthenticated,
}: {
  setupRequired: boolean;
  onAuthenticated: (user: ManagerUser) => void;
}) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitting) return;
    if (setupRequired && password !== confirmation) {
      setError("As senhas não coincidem.");
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      const response = await request<AuthResponse>(setupRequired ? "setup" : "login", setupRequired
        ? { name, email, password }
        : { email, password });
      onAuthenticated(response.user);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível entrar.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="auth-page">
      <section className="auth-panel" aria-labelledby="auth-title">
        <div className="auth-brand"><span className="brand-mark">E</span><span>Evolution GO</span></div>
        <span className="eyebrow">Manager V2</span>
        <h1 id="auth-title">{setupRequired ? "Crie a conta do administrador" : "Acesse o Manager"}</h1>
        <p>{setupRequired ? "Este é o primeiro acesso. Crie as credenciais que protegerão o gerenciamento das instâncias." : "Entre com as credenciais do administrador para gerenciar suas instâncias."}</p>
        <form onSubmit={(event) => void submit(event)}>
          {setupRequired && (
            <label>
              <span>Seu nome</span>
              <input autoFocus autoComplete="name" value={name} onChange={(event) => setName(event.target.value)} placeholder="Ex.: Ana Silva" required minLength={2} />
            </label>
          )}
          <label>
            <span>E-mail</span>
            <input autoFocus={!setupRequired} type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="voce@empresa.com" required />
          </label>
          <label>
            <span>Senha</span>
            <input type="password" autoComplete={setupRequired ? "new-password" : "current-password"} value={password} onChange={(event) => setPassword(event.target.value)} placeholder={setupRequired ? "Mínimo de 12 caracteres" : "Sua senha"} required minLength={12} maxLength={72} />
          </label>
          {setupRequired && (
            <label>
              <span>Confirme a senha</span>
              <input type="password" autoComplete="new-password" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} placeholder="Repita a senha" required minLength={12} maxLength={72} />
            </label>
          )}
          {error && <div className="auth-error" role="alert">{error}</div>}
          <button type="submit" className="button primary auth-submit" disabled={submitting}>{submitting ? "Aguarde…" : setupRequired ? "Criar conta e entrar" : "Entrar"}</button>
        </form>
      </section>
    </main>
  );
}
