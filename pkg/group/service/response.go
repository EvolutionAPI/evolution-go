package group_service

import "go.mau.fi/whatsmeow/types"

type CreateGroupResponse struct {
	Jid    types.JID   `json:"jid"`
	Name   string      `json:"name"`
	Owner  types.JID   `json:"owner"`
	Added  []types.JID `json:"added"`
	Failed []types.JID `json:"failed"`
}