package community_handler

import (
	"net/http"

	_ "github.com/EvolutionAPI/evolution-go/pkg/common/response"
	community_service "github.com/EvolutionAPI/evolution-go/pkg/community/service"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
	_ "go.mau.fi/whatsmeow/types"
)

type CommunityHandler interface {
	CreateCommunity(ctx *gin.Context)
	CommunityAdd(ctx *gin.Context)
	CommunityRemove(ctx *gin.Context)
}

type communityHandler struct {
	communityService community_service.CommunityService
}

// Create community
// @Summary Create community
// @Description Create community
// @Tags Community
// @Accept json
// @Produce json
// @Param message body community_service.CreateCommunityStruct true "Community data"
// @Success 200 {object} response.Success{data=types.GroupInfo} "success"
// @Failure 400 {object} response.Error "Error on validation"
// @Failure 500 {object} response.Error "Internal server error"
// @Router /community/create [post]
func (c *communityHandler) CreateCommunity(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *community_service.CreateCommunityStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.CommunityName == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "community name is required"})
		return
	}

	community, err := c.communityService.CreateCommunity(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": community})
}

// Add participant to community
// @Summary Add participant to community
// @Description Add participant to community
// @Tags Community
// @Accept json
// @Produce json
// @Param message body community_service.AddParticipantStruct true "Participant data"
// @Success 200 {object} response.Success{data=community_service.CommunityParticipantResponse} "success"
// @Failure 400 {object} response.Error "Error on validation"
// @Failure 500 {object} response.Error "Internal server error"
// @Router /community/add [post]
func (c *communityHandler) CommunityAdd(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *community_service.AddParticipantStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.CommunityJID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "community jid is required"})
		return
	}

	if len(data.GroupJID) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "group jid is required"})
		return
	}

	resp, err := c.communityService.CommunityAdd(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": resp})
}

// Remove participant from community
// @Summary Remove participant from community
// @Description Remove participant from community
// @Tags Community
// @Accept json
// @Produce json
// @Param message body community_service.AddParticipantStruct true "Participant data"
// @Success 200 {object} response.Success{data=community_service.CommunityParticipantResponse} "success"
// @Failure 400 {object} response.Error "Error on validation"
// @Failure 500 {object} response.Error "Internal server error"
// @Router /community/remove [post]
func (c *communityHandler) CommunityRemove(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *community_service.AddParticipantStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.CommunityJID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "community jid is required"})
		return
	}

	if len(data.GroupJID) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "group jid is required"})
		return
	}

	resp, err := c.communityService.CommunityRemove(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": resp})
}

func NewCommunityHandler(
	communityService community_service.CommunityService,
) CommunityHandler {
	return &communityHandler{
		communityService: communityService,
	}
}
