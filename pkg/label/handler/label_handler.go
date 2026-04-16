package label_handler

import (
	"net/http"

	_ "github.com/EvolutionAPI/evolution-go/pkg/common/response"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	_ "github.com/EvolutionAPI/evolution-go/pkg/label/model"
	label_service "github.com/EvolutionAPI/evolution-go/pkg/label/service"
	"github.com/gin-gonic/gin"
)

type LabelHandler interface {
	ChatLabel(ctx *gin.Context)
	MessageLabel(ctx *gin.Context)
	EditLabel(ctx *gin.Context)
	ChatUnlabel(ctx *gin.Context)
	MessageUnlabel(ctx *gin.Context)
	GetLabels(ctx *gin.Context)
}

type labelHandler struct {
	labelService label_service.LabelService
}

// Add label to chat
// @Summary Add label to chat
// @Description Add label to chat
// @Tags Label
// @Accept json
// @Produce json
// @Param message body label_service.ChatLabelStruct true "Label data"
// @Success 200 {object} response.Success "success"
// @Failure 400 {object} response.Error "Error on validation"
// @Failure 500 {object} response.Error "Internal server error"
// @Router /label/chat [post]
func (l *labelHandler) ChatLabel(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *label_service.ChatLabelStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.JID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "jid is required"})
		return
	}

	if data.LabelID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "label id is required"})
		return
	}

	err = l.labelService.ChatLabel(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Add label to message
// @Summary Add label to message
// @Description Add label to message
// @Tags Label
// @Accept json
// @Produce json
// @Param message body label_service.MessageLabelStruct true "Label data"
// @Success 200 {object} response.Success "success"
// @Failure 400 {object} response.Error "Error on validation"
// @Failure 500 {object} response.Error "Internal server error"
// @Router /label/message [post]
func (l *labelHandler) MessageLabel(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *label_service.MessageLabelStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.JID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "jid is required"})
		return
	}

	if data.LabelID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "label id is required"})
		return
	}

	if data.MessageID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "message id is required"})
		return
	}

	err = l.labelService.MessageLabel(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Edit label
// @Summary Edit label
// @Description Edit label
// @Tags Label
// @Accept json
// @Produce json
// @Param message body label_service.EditLabelStruct true "Label data"
// @Success 200 {object} response.Success "success"
// @Failure 400 {object} response.Error "Error on validation"
// @Failure 500 {object} response.Error "Internal server error"
// @Router /label/edit [post]
func (l *labelHandler) EditLabel(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *label_service.EditLabelStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.LabelID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "label id is required"})
		return
	}

	if data.Name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	err = l.labelService.EditLabel(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Remove label from chat
// @Summary Remove label from chat
// @Description Remove label from chat
// @Tags Label
// @Accept json
// @Produce json
// @Param message body label_service.ChatLabelStruct true "Label data"
// @Success 200 {object} response.Success "success"
// @Failure 400 {object} response.Error "Error on validation"
// @Failure 500 {object} response.Error "Internal server error"
// @Router /unlabel/chat [post]
func (l *labelHandler) ChatUnlabel(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *label_service.ChatLabelStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.JID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "jid is required"})
		return
	}

	if data.LabelID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "label id is required"})
		return
	}

	err = l.labelService.ChatUnlabel(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Remove label from message
// @Summary Remove label from message
// @Description Remove label from message
// @Tags Label
// @Accept json
// @Produce json
// @Param message body label_service.MessageLabelStruct true "Label data"
// @Success 200 {object} response.Success "success"
// @Failure 400 {object} response.Error "Error on validation"
// @Failure 500 {object} response.Error "Internal server error"
// @Router /unlabel/message [post]
func (l *labelHandler) MessageUnlabel(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *label_service.MessageLabelStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.JID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "jid is required"})
		return
	}

	if data.LabelID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "label id is required"})
		return
	}

	if data.MessageID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "message id is required"})
		return
	}

	err = l.labelService.MessageUnlabel(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Get all labels
// @Summary Get all labels
// @Description Get all labels
// @Tags Label
// @Accept json
// @Produce json
// @Success 200 {array} label_model.Label "success"
// @Failure 500 {object} response.Error "Internal server error"
// @Router /label [get]
func (l *labelHandler) GetLabels(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	labels, err := l.labelService.GetLabels(instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, labels)
}

func NewLabelHandler(
	labelService label_service.LabelService,
) LabelHandler {
	return &labelHandler{
		labelService: labelService,
	}
}
