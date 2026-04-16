package community_service

type CommunityParticipantResponse struct {
	Success []string `json:"success"`
	Failed  []string `json:"failed"`
}