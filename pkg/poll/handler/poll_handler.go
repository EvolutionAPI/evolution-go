package poll_handler

import (
	"encoding/json"
	"net/http"

	_ "github.com/EvolutionAPI/evolution-go/pkg/common/response"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	poll_service "github.com/EvolutionAPI/evolution-go/pkg/poll/service"
	"github.com/gin-gonic/gin"
)

type PollHandler struct {
	pollService   poll_service.PollService
	loggerWrapper *logger_wrapper.LoggerManager
}

// NewPollHandler cria handler usando PollService existente (evita dupla inicialização)
func NewPollHandler(pollService poll_service.PollService, loggerWrapper *logger_wrapper.LoggerManager) *PollHandler {
	return &PollHandler{
		pollService:   pollService,
		loggerWrapper: loggerWrapper,
	}
}

// GetPollResults retorna os resultados de uma enquete
// @Summary Get poll results
// @Description Retorna todos os votos de uma enquete específica
// @Tags Polls
// @Accept json
// @Produce json
// @Param pollMessageId path string true "ID da mensagem da enquete"
// @Success 200 {object} github_com_EvolutionAPI_evolution-go_pkg_poll_model.PollResults
// @Failure 400 {object} response.Error
// @Failure 404 {object} response.Error
// @Failure 500 {object} response.Error
// @Router /polls/{pollMessageId}/results [get]
func (h *PollHandler) GetPollResults(c *gin.Context) {
	pollMessageID := c.Param("pollMessageId")

	// Pegar instance do contexto de autenticação
	instanceInterface, exists := c.Get("instance")
	if !exists {
		h.loggerWrapper.GetLogger("poll-handler").LogWarn("[POLL] Instance not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		return
	}

	// Converter para struct Instance
	type Instance struct {
		Id string `json:"id"`
	}
	instanceBytes, _ := json.Marshal(instanceInterface)
	var instance Instance
	if err := json.Unmarshal(instanceBytes, &instance); err != nil {
		h.loggerWrapper.GetLogger("poll-handler").LogError("[POLL] Failed to parse instance: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get instance information",
		})
		return
	}

	instanceID := instance.Id

	// Validações de segurança
	if pollMessageID == "" {
		h.loggerWrapper.GetLogger("poll-handler").LogWarn("[POLL] Missing pollMessageId")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "pollMessageId is required",
		})
		return
	}

	h.loggerWrapper.GetLogger("poll-handler").LogInfo("[POLL] Fetching results for poll %s (instance: %s)", pollMessageID, instanceID)

	// Buscar resultados do banco
	results, err := h.pollService.GetPollResults(c.Request.Context(), pollMessageID, instanceID)
	if err != nil {
		h.loggerWrapper.GetLogger("poll-handler").LogError("[POLL] Error fetching results: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch poll results",
		})
		return
	}

	if results.TotalVotes == 0 {
		h.loggerWrapper.GetLogger("poll-handler").LogInfo("[POLL] No votes found for poll %s", pollMessageID)
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "No votes found for this poll",
			"message": "This poll has no votes yet, or the pollMessageId is incorrect",
		})
		return
	}

	h.loggerWrapper.GetLogger("poll-handler").LogInfo("[POLL] Returning %d votes for poll %s", results.TotalVotes, pollMessageID)
	c.JSON(http.StatusOK, results)
}
