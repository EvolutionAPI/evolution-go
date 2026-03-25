package group_handler

import (
	"net/http"

	group_service "github.com/EvolutionAPI/evolution-go/pkg/group/service"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
)

type GroupHandler interface {
	ListGroups(ctx *gin.Context)
	GetGroupInfo(ctx *gin.Context)
	GetGroupInviteLink(ctx *gin.Context)
	GetGroupInfoFromInviteLink(ctx *gin.Context)
	SetGroupPhoto(ctx *gin.Context)
	SetGroupName(ctx *gin.Context)
	SetGroupDescription(ctx *gin.Context)
	CreateGroup(ctx *gin.Context)
	UpdateParticipant(ctx *gin.Context)
	GetMyGroups(ctx *gin.Context)
	JoinGroupLink(ctx *gin.Context)
	LeaveGroup(ctx *gin.Context)
}

type groupHandler struct {
	groupService group_service.GroupService
}

// List groups
// @Summary List groups
// @Description List groups
// @Tags Group
// @Accept json
// @Produce json
// @Success 200 {object} gin.H "success"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/list [get]
func (g *groupHandler) ListGroups(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	resp, err := g.groupService.ListGroups(instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": resp})
}

// Get group info
// @Summary Get group info
// @Description Get group info
// @Tags Group
// @Accept json
// @Produce json
// @Param message body group_service.GetGroupInfoStruct true "Group data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/info [post]
func (g *groupHandler) GetGroupInfo(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *group_service.GetGroupInfoStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.GroupJID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "groupJID is required"})
		return
	}

	resp, err := g.groupService.GetGroupInfo(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": resp})
}

// Get group invite link
// @Summary Get group invite link
// @Description Get group invite link
// @Tags Group
// @Accept json
// @Produce json
// @Param message body group_service.GetGroupInviteLinkStruct true "Group data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/invitelink [post]
func (g *groupHandler) GetGroupInviteLink(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *group_service.GetGroupInviteLinkStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.GroupJID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "groupJID is required"})
		return
	}

	resp, err := g.groupService.GetGroupInviteLink(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": resp})
}

// Get group info from invite link
// @Summary Get group info from invite link
// @Description Get group info from invite link
// @Tags Group
// @Accept json
// @Produce json
// @Param message body group_service.GetGroupInfoFromInviteLinkStruct true "Group Invite Data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/invite-info [post]
func (g *groupHandler) GetGroupInfoFromInviteLink(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *group_service.GetGroupInfoFromInviteLinkStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Code == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	resp, err := g.groupService.GetGroupInfoFromInviteLink(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": resp})
}

// Set group photo
// @Summary Set group photo
// @Description Set group photo
// @Tags Group
// @Accept json
// @Produce json
// @Param message body group_service.SetGroupPhotoStruct true "Group data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/photo [post]
func (g *groupHandler) SetGroupPhoto(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *group_service.SetGroupPhotoStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.GroupJID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "groupJID is required"})
		return
	}

	if data.Image == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "image is required"})
		return
	}

	resp, err := g.groupService.SetGroupPhoto(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": resp})
}

// Set group name
// @Summary Set group name
// @Description Set group name
// @Tags Group
// @Accept json
// @Produce json
// @Param message body group_service.SetGroupNameStruct true "Group data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/name [post]
func (g *groupHandler) SetGroupName(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *group_service.SetGroupNameStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.GroupJID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "groupJID is required"})
		return
	}

	if data.Name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	err = g.groupService.SetGroupName(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Set group description
// @Summary Set group description
// @Description Set group description
// @Tags Group
// @Accept json
// @Produce json
// @Param message body group_service.SetGroupDescriptionStruct true "Group data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/description [post]
func (g *groupHandler) SetGroupDescription(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *group_service.SetGroupDescriptionStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.GroupJID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "groupJID is required"})
		return
	}

	// Description can be empty to clear the group description
	// No validation needed for Description field

	err = g.groupService.SetGroupDescription(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Create group
// @Summary Create group
// @Description Create group
// @Tags Group
// @Accept json
// @Produce json
// @Param message body group_service.CreateGroupStruct true "Group data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/create [post]
func (g *groupHandler) CreateGroup(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *group_service.CreateGroupStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.GroupName == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "groupName is required"})
		return
	}

	if len(data.Participants) < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "participants are required"})
		return
	}

	group, err := g.groupService.CreateGroup(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": group})
}

// Update participant
// @Summary Update participant
// @Description Update participant
// @Tags Group
// @Accept json
// @Produce json
// @Param message body group_service.AddParticipantStruct true "Group data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/participant [post]
func (g *groupHandler) UpdateParticipant(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *group_service.AddParticipantStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.GroupJID.String() == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "groupJid is required"})
		return
	}

	if data.Action == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "action is required"})
		return
	}

	if len(data.Participants) < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "participants are required"})
		return
	}

	err = g.groupService.UpdateParticipant(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Get my groups
// @Summary Get my groups
// @Description Get my groups
// @Tags Group
// @Accept json
// @Produce json
// @Success 200 {object} gin.H "success"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/myall [get]
func (g *groupHandler) GetMyGroups(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	groups, err := g.groupService.GetMyGroups(instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": groups})
}

// Join group link
// @Summary Join group link
// @Description Join group link
// @Tags Group
// @Accept json
// @Produce json
// @Param message body group_service.JoinGroupStruct true "Group data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/join [post]
func (g *groupHandler) JoinGroupLink(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *group_service.JoinGroupStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Code == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	err = g.groupService.JoinGroupLink(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Leave group
// @Summary Leave group
// @Description Leave group
// @Tags Group
// @Accept json
// @Produce json
// @Param message body group_service.LeaveGroupStruct true "Group data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /group/leave [post]
func (g *groupHandler) LeaveGroup(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *group_service.LeaveGroupStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.GroupJID.String() == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "groupJid is required"})
		return
	}

	err = g.groupService.LeaveGroup(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

func NewGroupHandler(
	groupService group_service.GroupService,
) GroupHandler {
	return &groupHandler{
		groupService: groupService,
	}
}
