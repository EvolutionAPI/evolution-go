import { useEffect, useMemo, useState } from "react";
import { displayPhone, type EvolutionApi, type EvolutionCall, type EvolutionContact } from "./api";
import { useIncomingCallAlerts } from "./call-alerts";
import type { CallDeskState } from "./calls";

type CallAction = "accept" | "reject" | null;

type CallerIdentity = {
	callId: string;
	name: string;
	avatarUrl: string | null;
};

function canonicalPeer(value: string): string {
	return value.trim().replace(/:\d+(?=@)/, "").toLowerCase();
}

function contactName(contact: EvolutionContact | undefined): string {
	if (!contact) return "";
	return [contact.BusinessName, contact.PushName, contact.FullName, contact.FirstName]
		.map((value) => value?.trim())
		.find(Boolean) || "";
}

function initials(value: string): string {
	const letters = value.trim().split(/\s+/).map((part) => part[0]).join("").slice(0, 2);
	return letters.toUpperCase() || "☎";
}

function formatElapsed(value?: string): string {
	if (!value) return "00:00";
	const startedAt = new Date(value).getTime();
	if (Number.isNaN(startedAt)) return "00:00";
	const seconds = Math.max(0, Math.floor((Date.now() - startedAt) / 1000));
	return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

function outcomeLabel(call: EvolutionCall): string {
	if (call.state === "failed") return call.error ? `Falhou: ${call.error}` : "Falhou ao concluir a chamada";
	if (call.endReason === "answered_elsewhere") return "Atendida em outro dispositivo";
	if (call.endReason === "rejected_elsewhere") return "Recusada por outro dispositivo";
	if (call.endReason === "rejected") return "Chamada recusada";
	if (call.endReason === "caller_cancelled") return "Chamada encerrada antes do atendimento";
	return "Chamada encerrada";
}

export function IncomingCallOverlay({
	api,
	desk,
	onOpenCalls,
	onAccept,
}: {
	api: EvolutionApi | null;
	desk: CallDeskState;
	onOpenCalls: () => void;
	onAccept: (call: EvolutionCall) => Promise<void>;
}) {
	const [action, setAction] = useState<CallAction>(null);
	const [identity, setIdentity] = useState<CallerIdentity>({ callId: "", name: "", avatarUrl: null });
	const [avatarFailed, setAvatarFailed] = useState(false);
	const [elapsed, setElapsed] = useState("00:00");
	const incomingCalls = useMemo(() => desk.snapshot.calls
		.filter((call) => call.direction === "incoming" && call.state === "ringing")
		.reverse(), [desk.snapshot.calls]);
	const incoming = incomingCalls[0] ?? null;
	const callerIdentity = incoming && identity.callId === incoming.id ? identity : { callId: "", name: "", avatarUrl: null };
	const callerName = callerIdentity.name || (incoming ? displayPhone(incoming.peer) : "");
	const alerts = useIncomingCallAlerts(incoming, callerName);

	useEffect(() => setAction(null), [incoming?.id]);
	useEffect(() => setAvatarFailed(false), [incoming?.id, identity.avatarUrl]);

	useEffect(() => {
		if (!incoming) {
			setElapsed("00:00");
			return;
		}
		const update = () => setElapsed(formatElapsed(incoming.createdAt));
		update();
		const timer = window.setInterval(update, 1000);
		return () => window.clearInterval(timer);
	}, [incoming?.createdAt, incoming?.id]);

	useEffect(() => {
		let active = true;
		if (!incoming || !api) {
			setIdentity({ callId: "", name: "", avatarUrl: null });
			return () => { active = false; };
		}
		setIdentity({ callId: incoming.id, name: "", avatarUrl: null });
		void api.contacts().catch(() => [] as EvolutionContact[]).then((contacts) => {
			if (!active) return;
			const peer = canonicalPeer(incoming.peer);
			const contact = contacts.find((entry) => canonicalPeer(entry.Jid) === peer);
			setIdentity((current) => current.callId === incoming.id ? { ...current, name: contactName(contact) } : current);
		});
		void api.avatar(incoming.peer).catch(() => null).then((avatarUrl) => {
			if (!active) return;
			setIdentity((current) => current.callId === incoming.id ? { ...current, avatarUrl } : current);
		});
		return () => { active = false; };
	}, [api, incoming?.id, incoming?.peer]);

	if (!incoming && !desk.recentOutcome) return null;

	if (!incoming && desk.recentOutcome) {
		const outcome = desk.recentOutcome;
		return (
			<aside className="incoming-call-overlay call-outcome-overlay" role="status" aria-live="polite">
				<div className="incoming-call-icon outcome" aria-hidden="true">☎</div>
				<div className="incoming-call-content">
					<span className="eyebrow">Atualização da chamada</span>
					<h2>{outcomeLabel(outcome)}</h2>
					<p>{displayPhone(outcome.peer)}{outcome.answeredBy ? ` · ${outcome.answeredBy}` : ""}</p>
				</div>
				<div className="incoming-call-actions">
					<button type="button" className="incoming-call-open" onClick={onOpenCalls}>Ver chamadas</button>
					<button type="button" className="incoming-call-dismiss" onClick={desk.dismissOutcome}>Fechar</button>
				</div>
			</aside>
		);
	}

	const accept = async () => {
		if (action || !incoming) return;
		setAction("accept");
		try {
			await onAccept(incoming);
		} catch {
			// The desk retains the error and keeps the action card available.
		} finally {
			setAction(null);
		}
	};

	const reject = async () => {
		if (action || !incoming) return;
		setAction("reject");
		try {
			await desk.reject(incoming);
		} catch {
			// The desk retains the error and keeps the action card available.
		} finally {
			setAction(null);
		}
	};

	const openCurrent = () => {
		desk.setSelectedCallId(incoming.id);
		onOpenCalls();
	};

	return (
		<aside className="incoming-call-overlay" role="alertdialog" aria-live="assertive" aria-labelledby="incoming-call-title">
			<div className="incoming-call-avatar" aria-hidden="true">
				{callerIdentity.avatarUrl && !avatarFailed
					? <img src={callerIdentity.avatarUrl} alt="" onError={() => setAvatarFailed(true)} />
					: <span>{initials(callerName)}</span>}
			</div>
			<div className="incoming-call-content">
				<span className="eyebrow">Ligação recebida · {elapsed}</span>
				<h2 id="incoming-call-title">{callerName}</h2>
				<p>{displayPhone(incoming.peer)} · {desk.snapshot.instanceId ? `Instância ${desk.snapshot.instanceId}` : "Instância selecionada"}</p>
				{incomingCalls.length > 1 && <p className="incoming-call-queue-note">+{incomingCalls.length - 1} ligação(ões) aguardando na fila</p>}
				{desk.error && <div className="incoming-call-error" role="alert">{desk.error}</div>}
			</div>
			<div className="incoming-call-actions">
				<button type="button" className="incoming-call-open" onClick={openCurrent}>Abrir painel</button>
				<button type="button" className="incoming-call-alert-toggle" onClick={alerts.toggleMuted}>{alerts.muted ? "Ativar toque" : "Silenciar toque"}</button>
				{alerts.notificationAccess === "default" && <button type="button" className="incoming-call-alert-toggle" onClick={() => void alerts.requestNotifications()}>Ativar alertas</button>}
				<button type="button" className="incoming-call-reject" onClick={() => void reject()} disabled={action !== null}>{action === "reject" ? "Recusando…" : "Recusar"}</button>
				<button type="button" className="incoming-call-accept" onClick={() => void accept()} disabled={action !== null}>{action === "accept" ? "Atendendo…" : "Atender"}</button>
			</div>
		</aside>
	);
}
