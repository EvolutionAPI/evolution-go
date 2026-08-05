import { useCallback, useEffect, useRef, useState } from "react";
import type { EvolutionCall } from "./api";

const MUTE_STORAGE_KEY = "evolution.managerV2.call-alerts-muted.v1";

export type NotificationAccess = NotificationPermission | "unsupported";

function storedMutePreference(): boolean {
	try {
		return window.localStorage.getItem(MUTE_STORAGE_KEY) === "true";
	} catch {
		return false;
	}
}

function currentNotificationAccess(): NotificationAccess {
	return typeof Notification === "undefined" ? "unsupported" : Notification.permission;
}

export function useIncomingCallAlerts(call: EvolutionCall | null, callerName: string) {
	const [muted, setMuted] = useState(storedMutePreference);
	const [notificationAccess, setNotificationAccess] = useState<NotificationAccess>(currentNotificationAccess);
	const notifiedCallId = useRef("");

	const toggleMuted = useCallback(() => {
		setMuted((current) => {
			const next = !current;
			try {
				window.localStorage.setItem(MUTE_STORAGE_KEY, String(next));
			} catch {
				// A blocked storage policy must not prevent a call from being handled.
			}
			return next;
		});
	}, []);

	const requestNotifications = useCallback(async () => {
		if (typeof Notification === "undefined") {
			setNotificationAccess("unsupported");
			return "unsupported" as const;
		}
		const permission = await Notification.requestPermission();
		setNotificationAccess(permission);
		return permission;
	}, []);

	useEffect(() => {
		if (!call) return;
		const originalTitle = document.title;
		let blink = false;
		const titleTimer = window.setInterval(() => {
			blink = !blink;
			document.title = blink ? "☎ Ligação recebida" : originalTitle;
		}, 1000);

		if (typeof Notification !== "undefined" && Notification.permission === "granted" && notifiedCallId.current !== call.id) {
			try {
				notifiedCallId.current = call.id;
				const notification = new Notification("Ligação recebida", {
					body: callerName ? `${callerName} está ligando.` : "Uma pessoa está ligando.",
					tag: `evolution-call-${call.id}`,
				});
				notification.onclick = () => {
					window.focus();
					notification.close();
				};
			} catch {
				// A browser may revoke notification access between the permission
				// check and construction; visual and audio alerts still continue.
			}
		}

		let audioContext: AudioContext | null = null;
		let toneTimer: number | undefined;
		let disposed = false;
		const stopTone = () => {
			if (toneTimer !== undefined) window.clearInterval(toneTimer);
			toneTimer = undefined;
			if (audioContext && audioContext.state !== "closed") void audioContext.close();
			audioContext = null;
		};
		const startTone = async () => {
			if (muted || disposed || audioContext) return;
			try {
				const browserWindow = globalThis as typeof globalThis & {
					AudioContext?: typeof AudioContext;
					webkitAudioContext?: typeof AudioContext;
				};
				const AudioContextConstructor = browserWindow.AudioContext || browserWindow.webkitAudioContext;
				if (!AudioContextConstructor) return;
				const context = new AudioContextConstructor();
				audioContext = context;
				await context.resume();
				if (disposed || muted || audioContext !== context || context.state !== "running") {
					stopTone();
					return;
				}
				const playTone = () => {
					if (context.state !== "running") return;
					const oscillator = context.createOscillator();
					const gain = context.createGain();
					oscillator.type = "sine";
					oscillator.frequency.setValueAtTime(740, context.currentTime);
					gain.gain.setValueAtTime(0.0001, context.currentTime);
					gain.gain.exponentialRampToValueAtTime(0.12, context.currentTime + 0.025);
					gain.gain.exponentialRampToValueAtTime(0.0001, context.currentTime + 0.34);
					oscillator.connect(gain).connect(context.destination);
					oscillator.start();
					oscillator.stop(context.currentTime + 0.36);
				};
				playTone();
				toneTimer = window.setInterval(playTone, 1250);
			} catch {
				// Browsers may defer audio until a gesture. The listeners below retry
				// on the next interaction without interrupting the call controls.
				stopTone();
			}
		};
		const unlockTone = () => { void startTone(); };
		if (!muted) {
			void startTone();
			window.addEventListener("pointerdown", unlockTone, { once: true });
			window.addEventListener("keydown", unlockTone, { once: true });
		}

		return () => {
			disposed = true;
			window.clearInterval(titleTimer);
			if (document.title === "☎ Ligação recebida" || document.title === originalTitle) document.title = originalTitle;
			window.removeEventListener("pointerdown", unlockTone);
			window.removeEventListener("keydown", unlockTone);
			stopTone();
		};
	}, [call?.id, callerName, muted, notificationAccess]);

	return { muted, toggleMuted, notificationAccess, requestNotifications };
}
