package send_handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	send_service "github.com/EvolutionAPI/evolution-go/pkg/sendMessage/service"
	"github.com/gin-gonic/gin"
)

type SendHandler interface {
	SendText(ctx *gin.Context)
	SendLink(ctx *gin.Context)
	SendMedia(ctx *gin.Context)
	SendPoll(ctx *gin.Context)
	SendSticker(ctx *gin.Context)
	SendLocation(ctx *gin.Context)
	SendContact(ctx *gin.Context)
	SendButton(ctx *gin.Context)
	SendList(ctx *gin.Context)
	SendStatusText(ctx *gin.Context)
	SendStatusMedia(ctx *gin.Context)
}

type sendHandler struct {
	sendMessageService send_service.SendService
}

// Send a text message
// @Summary Send a text message
// @Description Send a text message
// @Tags Send Message
// @Accept json
// @Produce json
// @Param message body send_service.TextStruct true "Message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /send/text [post]
func (s *sendHandler) SendText(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *send_service.TextStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone number is required"})
		return
	}

	if data.Text == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "message body is required"})
		return
	}

	message, err := s.sendMessageService.SendText(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
}

// Send a link message
// @Summary Send a link message
// @Description Send a link message
// @Tags Send Message
// @Accept json
// @Produce json
// @Param message body send_service.LinkStruct true "Message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /send/link [post]
func (s *sendHandler) SendLink(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *send_service.LinkStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone number is required"})
		return
	}

	if data.Text == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "message body is required"})
		return
	}

	message, err := s.sendMessageService.SendLink(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
}

// Send a media message
// @Summary Send a media message
// @Description Send a media message
// @Tags Send Message
// @Accept json
// @Produce json
// @Param message body send_service.MediaStruct true "Message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /send/media [post]
func (s *sendHandler) SendMedia(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	contentType := ctx.ContentType()

	var data *send_service.MediaStruct

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Handle form-data
		number := ctx.PostForm("number")
		if number == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone number is required"})
			return
		}

		mediaType := ctx.PostForm("type")
		if mediaType == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "media type is required"})
			return
		}

		caption := ctx.PostForm("caption")
		filename := ctx.PostForm("filename")
		id := ctx.PostForm("id")
		delayStr := ctx.PostForm("delay")
		delay := int32(0)
		if delayStr != "" {
			delay64, err := strconv.ParseInt(delayStr, 10, 32)
			if err != nil {
				ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid delay"})
				return
			}
			delay = int32(delay64)
		}

		// Get file
		file, err := ctx.FormFile("file")
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
			return
		}

		// Open file
		fileData, err := file.Open()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "cannot open file"})
			return
		}
		defer fileData.Close()
		fileBytes, err := io.ReadAll(fileData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
			return
		}

		// Create MediaStruct
		data = &send_service.MediaStruct{
			Number:   number,
			Type:     mediaType,
			Caption:  caption,
			Filename: filename,
			Id:       id,
			Delay:    delay,
			// Other fields as necessary
		}

		// Pass fileBytes to the send service
		message, err := s.sendMessageService.SendMediaFile(data, fileBytes, instance)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})

	} else {

		err := ctx.ShouldBindBodyWithJSON(&data)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if data.Number == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone number is required"})
			return
		}

		if data.Url == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
			return
		}

		if data.Type == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "media type is required"})
			return
		}

		message, err := s.sendMessageService.SendMediaUrl(data, instance)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
	}
}

// Send a poll message
// @Summary Send a poll message
// @Description Send a poll message
// @Tags Send Message
// @Accept json
// @Produce json
// @Param message body send_service.PollStruct true "Message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /send/poll [post]
func (s *sendHandler) SendPoll(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *send_service.PollStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone number is required"})
		return
	}

	if data.Question == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "question is required"})
		return
	}

	if len(data.Options) < 2 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "minimum 2 options are required"})
		return
	}

	message, err := s.sendMessageService.SendPoll(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
}

// Send a sticker message
// @Summary Send a sticker message
// @Description Send a sticker message
// @Tags Send Message
// @Accept json
// @Produce json
// @Param message body send_service.StickerStruct true "Message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /send/sticker [post]
func (s *sendHandler) SendSticker(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *send_service.StickerStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone number is required"})
		return
	}

	if data.Sticker == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "sticker is required"})
		return
	}

	message, err := s.sendMessageService.SendSticker(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
}

// Send a location message
// @Summary Send a location message
// @Description Send a location message
// @Tags Send Message
// @Accept json
// @Produce json
// @Param message body send_service.LocationStruct true "Message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /send/location [post]
func (s *sendHandler) SendLocation(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *send_service.LocationStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone number is required"})
		return
	}

	if data.Latitude == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "latitude is required"})
		return
	}

	if data.Longitude == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "longitude is required"})
		return
	}

	if data.Address == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "address is required"})
		return
	}

	if data.Name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	message, err := s.sendMessageService.SendLocation(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
}

// Send a contact message
// @Summary Send a contact message
// @Description Send a contact message
// @Tags Send Message
// @Accept json
// @Produce json
// @Param message body send_service.ContactStruct true "Message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /send/contact [post]
func (s *sendHandler) SendContact(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *send_service.ContactStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone number is required"})
		return
	}

	if data.Vcard.Phone == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "contact phone number is required"})
		return
	}

	if data.Vcard.FullName == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "contact full name is required"})
		return
	}

	message, err := s.sendMessageService.SendContact(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
}

// Send a button message
// @Summary Send a button message
// @Description Send a button message
// @Tags Send Message
// @Accept json
// @Produce json
// @Param message body send_service.ContactStruct true "Message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /send/button [post]
func (s *sendHandler) SendButton(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *send_service.ButtonStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone number is required"})
		return
	}

	if data.Title == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	if data.Description == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "description is required"})
		return
	}

	if data.Footer == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "footer is required"})
		return
	}

	message, err := s.sendMessageService.SendButton(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
}

// Send a list message
// @Summary Send a list message
// @Description Send a list message
// @Tags Send Message
// @Accept json
// @Produce json
// @Param message body send_service.ContactStruct true "Message data"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Error on validation"
// @Failure 500 {object} gin.H "Internal server error"
// @Router /send/list [post]
func (s *sendHandler) SendList(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *send_service.ListStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Number == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "phone number is required"})
		return
	}

	if data.Title == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	if data.Description == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "description is required"})
		return
	}

	if data.FooterText == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "footer is required"})
		return
	}

	if data.ButtonText == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "button text is required"})
		return
	}

	message, err := s.sendMessageService.SendList(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
}

func (s *sendHandler) SendStatusText(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	var data *send_service.StatusTextStruct
	err := ctx.ShouldBindBodyWithJSON(&data)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Text == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}

	message, err := s.sendMessageService.SendStatusText(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
}

func (s *sendHandler) SendStatusMedia(ctx *gin.Context) {
	getInstance := ctx.MustGet("instance")

	instance, ok := getInstance.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return
	}

	contentType := ctx.ContentType()

	var data *send_service.StatusMediaStruct

	if strings.HasPrefix(contentType, "multipart/form-data") {
		mediaType := ctx.PostForm("type")
		if mediaType == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "media type is required"})
			return
		}

		if mediaType != "image" && mediaType != "video" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'image' or 'video'"})
			return
		}

		caption := ctx.PostForm("caption")
		id := ctx.PostForm("id")

		file, err := ctx.FormFile("file")
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
			return
		}

		fileData, err := file.Open()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "cannot open file"})
			return
		}
		defer fileData.Close()
		fileBytes, err := io.ReadAll(fileData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
			return
		}

		data = &send_service.StatusMediaStruct{
			Type:    mediaType,
			Caption: caption,
			Id:      id,
		}

		message, err := s.sendMessageService.SendStatusMediaFile(data, fileBytes, instance)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
	} else {
		err := ctx.ShouldBindBodyWithJSON(&data)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if data.Url == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
			return
		}

		if data.Type != "image" && data.Type != "video" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'image' or 'video'"})
			return
		}

		message, err := s.sendMessageService.SendStatusMediaUrl(data, instance)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": message})
	}
}

func NewSendHandler(
	sendMessageService send_service.SendService,
) SendHandler {
	return &sendHandler{
		sendMessageService: sendMessageService,
	}
}
