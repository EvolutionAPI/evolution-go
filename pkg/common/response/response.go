package response

// Success is the envelope returned by every 2xx handler response. Handlers
// write it as gin.H{"message": "success", "data": ...}; this struct documents
// that shape for swaggo. Specialize `data` per route with composition, e.g.
//   @Success 200 {object} response.Success{data=send_service.MessageSendStruct}
type Success struct {
	Message string      `json:"message" example:"success"`
	Data    interface{} `json:"data,omitempty" swaggertype:"object"`
}

// Error is the envelope returned by every 4xx/5xx handler response.
type Error struct {
	Error string `json:"error" example:"error message"`
}
