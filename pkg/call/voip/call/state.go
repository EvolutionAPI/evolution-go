// Portions are derived from JotaDev66/WaCalls under the MIT license in ../LICENSE-WACALLS.
package call

import (
	"fmt"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/call/voip/core"
)

type StateData struct {
	State        core.CallState
	ConnectedAt  *time.Time
	AcceptedAt   *time.Time
	EndedAt      *time.Time
	AudioMuted   bool
	VideoOff     bool
	Silenced     bool
	EndReason    core.EndCallReason
	DurationSecs int
}

type Info struct {
	CallID      string
	PeerJID     string
	CallCreator string
	Direction   core.CallDirection
	MediaType   core.CallMediaType
	StateData   StateData
	CreatedAt   time.Time
}

func NewOutgoing(callID, peerJID, creator string, mediaType core.CallMediaType) *Info {
	return &Info{
		CallID:      callID,
		PeerJID:     peerJID,
		CallCreator: creator,
		Direction:   core.CallDirectionOutgoing,
		MediaType:   mediaType,
		CreatedAt:   time.Now().UTC(),
		StateData: StateData{
			State:      core.CallStateInitiating,
			VideoOff:   mediaType != core.CallMediaTypeVideo,
			AudioMuted: false,
		},
	}
}

func NewIncoming(callID, peerJID, creator string, mediaType core.CallMediaType) *Info {
	return &Info{
		CallID:      callID,
		PeerJID:     peerJID,
		CallCreator: creator,
		Direction:   core.CallDirectionIncoming,
		MediaType:   mediaType,
		CreatedAt:   time.Now().UTC(),
		StateData: StateData{
			State:      core.CallStateIncomingRinging,
			VideoOff:   mediaType != core.CallMediaTypeVideo,
			AudioMuted: false,
		},
	}
}

func (c *Info) IsInitiator() bool { return c != nil && c.Direction == core.CallDirectionOutgoing }
func (c *Info) IsActive() bool    { return c != nil && c.StateData.State == core.CallStateActive }
func (c *Info) IsEnded() bool     { return c != nil && c.StateData.State == core.CallStateEnded }
func (c *Info) CanAccept() bool {
	return c != nil && c.StateData.State == core.CallStateIncomingRinging
}

func (c *Info) Clone() *Info {
	if c == nil {
		return nil
	}
	clone := *c
	clone.StateData.ConnectedAt = cloneTime(c.StateData.ConnectedAt)
	clone.StateData.AcceptedAt = cloneTime(c.StateData.AcceptedAt)
	clone.StateData.EndedAt = cloneTime(c.StateData.EndedAt)
	return &clone
}

type TransitionType string

const (
	TransitionOfferSent         TransitionType = "offer_sent"
	TransitionRemoteAccepted    TransitionType = "remote_accepted"
	TransitionLocalAccepted     TransitionType = "local_accepted"
	TransitionRemoteRejected    TransitionType = "remote_rejected"
	TransitionLocalRejected     TransitionType = "local_rejected"
	TransitionMediaConnected    TransitionType = "media_connected"
	TransitionTerminated        TransitionType = "terminated"
	TransitionHold              TransitionType = "hold"
	TransitionResume            TransitionType = "resume"
	TransitionAudioMuteChanged  TransitionType = "audio_mute_changed"
	TransitionVideoStateChanged TransitionType = "video_state_changed"
)

type Transition struct {
	Type   TransitionType
	Reason core.EndCallReason
	Muted  bool
	Off    bool
}

type InvalidTransition struct {
	CurrentState core.CallState
	Attempted    TransitionType
}

func (e *InvalidTransition) Error() string {
	return fmt.Sprintf("invalid transition %q in state %q", e.Attempted, e.CurrentState)
}

func (c *Info) Apply(transition Transition) error {
	if c == nil {
		return fmt.Errorf("nil call state")
	}
	state := &c.StateData
	now := time.Now().UTC()

	switch transition.Type {
	case TransitionOfferSent:
		if state.State != core.CallStateInitiating {
			return invalid(state.State, transition.Type)
		}
		state.State = core.CallStateRinging
	case TransitionRemoteAccepted:
		if state.State != core.CallStateRinging {
			return invalid(state.State, transition.Type)
		}
		state.State = core.CallStateConnecting
		state.AcceptedAt = &now
	case TransitionLocalAccepted:
		if state.State != core.CallStateIncomingRinging {
			return invalid(state.State, transition.Type)
		}
		state.State = core.CallStateConnecting
		state.AcceptedAt = &now
	case TransitionRemoteRejected:
		if state.State != core.CallStateRinging {
			return invalid(state.State, transition.Type)
		}
		endState(state, now, transition.Reason)
	case TransitionLocalRejected:
		if state.State != core.CallStateIncomingRinging {
			return invalid(state.State, transition.Type)
		}
		endState(state, now, transition.Reason)
	case TransitionMediaConnected:
		if state.State != core.CallStateConnecting {
			return invalid(state.State, transition.Type)
		}
		state.State = core.CallStateActive
		state.ConnectedAt = &now
		state.VideoOff = c.MediaType != core.CallMediaTypeVideo
	case TransitionTerminated:
		if state.State == core.CallStateEnded {
			return invalid(state.State, transition.Type)
		}
		if (state.State == core.CallStateActive || state.State == core.CallStateOnHold) && state.ConnectedAt != nil {
			state.DurationSecs = int(now.Sub(*state.ConnectedAt).Seconds())
		}
		endState(state, now, transition.Reason)
	case TransitionHold:
		if state.State != core.CallStateActive {
			return invalid(state.State, transition.Type)
		}
		state.State = core.CallStateOnHold
	case TransitionResume:
		if state.State != core.CallStateOnHold {
			return invalid(state.State, transition.Type)
		}
		state.State = core.CallStateActive
	case TransitionAudioMuteChanged:
		if state.State != core.CallStateActive {
			return invalid(state.State, transition.Type)
		}
		state.AudioMuted = transition.Muted
	case TransitionVideoStateChanged:
		if state.State != core.CallStateActive {
			return invalid(state.State, transition.Type)
		}
		state.VideoOff = transition.Off
	default:
		return invalid(state.State, transition.Type)
	}
	return nil
}

func invalid(state core.CallState, transition TransitionType) error {
	return &InvalidTransition{CurrentState: state, Attempted: transition}
}

func endState(state *StateData, now time.Time, reason core.EndCallReason) {
	state.State = core.CallStateEnded
	state.EndedAt = &now
	state.EndReason = reason
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
