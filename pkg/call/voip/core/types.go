// Package core contains the transport-independent WhatsApp VoIP domain types.
// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package core

type CallState string

const (
	CallStateInitiating      CallState = "initiating"
	CallStateRinging         CallState = "ringing"
	CallStateIncomingRinging CallState = "incoming_ringing"
	CallStateConnecting      CallState = "connecting"
	CallStateActive          CallState = "active"
	CallStateOnHold          CallState = "on_hold"
	CallStateEnded           CallState = "ended"
)

type CallDirection string

const (
	CallDirectionOutgoing CallDirection = "outgoing"
	CallDirectionIncoming CallDirection = "incoming"
)

type CallMediaType string

const (
	CallMediaTypeAudio CallMediaType = "audio"
	CallMediaTypeVideo CallMediaType = "video"
)

type EndCallReason string

const (
	EndCallReasonUserEnded    EndCallReason = "user_ended"
	EndCallReasonDeclined     EndCallReason = "declined"
	EndCallReasonTimeout      EndCallReason = "timeout"
	EndCallReasonBusy         EndCallReason = "busy"
	EndCallReasonCancelled    EndCallReason = "cancelled"
	EndCallReasonFailed       EndCallReason = "failed"
	EndCallReasonDoNotDisturb EndCallReason = "do_not_disturb"
	EndCallReasonUnknown      EndCallReason = "unknown"
)
