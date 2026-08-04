package managerconfig

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	config "github.com/evolution-foundation/evolution-go/pkg/config"
	managerauth "github.com/evolution-foundation/evolution-go/pkg/managerauth"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// InfrastructureSettings contains the values managed from Manager V2. It is
// intentionally separate from the global API key, which never leaves the .env.
type InfrastructureSettings struct {
	AMQPEnabled       bool   `json:"amqpEnabled"`
	AMQPURL           string `json:"amqpUrl"`
	AMQPGlobalEnabled bool   `json:"amqpGlobalEnabled"`
	WebhookURL        string `json:"webhookUrl"`
	ProxyEnabled      bool   `json:"proxyEnabled"`
	ProxyProtocol     string `json:"proxyProtocol"`
	ProxyHost         string `json:"proxyHost"`
	ProxyPort         string `json:"proxyPort"`
	ProxyUsername     string `json:"proxyUsername"`
	ProxyPassword     string `json:"proxyPassword"`
	MinioEnabled      bool   `json:"minioEnabled"`
	MinioEndpoint     string `json:"minioEndpoint"`
	MinioAccessKey    string `json:"minioAccessKey"`
	MinioSecretKey    string `json:"minioSecretKey"`
	MinioBucket       string `json:"minioBucket"`
	MinioUseSSL       bool   `json:"minioUseSsl"`
}

// StoredInfrastructureSettings keeps every configurable value encrypted at rest.
type StoredInfrastructureSettings struct {
	ID                   uint `gorm:"primaryKey"`
	AMQPEnabled          bool
	AMQPURLEncrypted     string `gorm:"type:text"`
	AMQPGlobalEnabled    bool
	WebhookURLEncrypted  string `gorm:"type:text"`
	ProxyEnabled         bool
	ProxyProtocol        string
	ProxyHostEncrypted   string `gorm:"type:text"`
	ProxyPortEncrypted   string `gorm:"type:text"`
	ProxyUserEncrypted   string `gorm:"type:text"`
	ProxyPassEncrypted   string `gorm:"type:text"`
	MinioEnabled         bool
	MinioEndEncrypted    string `gorm:"type:text"`
	MinioAccessEncrypted string `gorm:"type:text"`
	MinioSecretEncrypted string `gorm:"type:text"`
	MinioBucketEncrypted string `gorm:"type:text"`
	MinioUseSSL          bool
	CreatedAt            time.Time `gorm:"autoCreateTime"`
	UpdatedAt            time.Time `gorm:"autoUpdateTime"`
}

func (StoredInfrastructureSettings) TableName() string {
	return "manager_infrastructure_settings"
}

type Service struct {
	db     *gorm.DB
	config *config.Config
	key    []byte
}

func NewService(db *gorm.DB, appConfig *config.Config, secret string) *Service {
	hash := sha256.Sum256([]byte(secret))
	return &Service{db: db, config: appConfig, key: hash[:]}
}

// LoadAndApply overlays the .env defaults with settings saved in Manager V2.
// It is called before AMQP, MinIO and the other integrations are initialized.
func (s *Service) LoadAndApply(ctx context.Context) error {
	var stored StoredInfrastructureSettings
	err := s.db.WithContext(ctx).First(&stored).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	settings, err := s.decode(stored)
	if err != nil {
		return err
	}
	s.apply(settings)
	return nil
}

func (s *Service) Current() InfrastructureSettings {
	return InfrastructureSettings{
		AMQPEnabled:       s.config.AmqpUrl != "",
		AMQPURL:           s.config.AmqpUrl,
		AMQPGlobalEnabled: s.config.AmqpGlobalEnabled,
		WebhookURL:        s.config.WebhookUrl,
		ProxyEnabled:      s.config.ProxyHost != "",
		ProxyProtocol:     s.config.ProxyProtocol,
		ProxyHost:         s.config.ProxyHost,
		ProxyPort:         s.config.ProxyPort,
		ProxyUsername:     s.config.ProxyUsername,
		ProxyPassword:     s.config.ProxyPassword,
		MinioEnabled:      s.config.MinioEnabled,
		MinioEndpoint:     s.config.MinioEndpoint,
		MinioAccessKey:    s.config.MinioAccessKey,
		MinioSecretKey:    s.config.MinioSecretKey,
		MinioBucket:       s.config.MinioBucket,
		MinioUseSSL:       s.config.MinioUseSSL,
	}
}

func (s *Service) Save(ctx context.Context, settings InfrastructureSettings) error {
	if err := validate(settings); err != nil {
		return err
	}
	stored, err := s.encode(settings)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing StoredInfrastructureSettings
		if err := tx.First(&existing).Error; err == nil {
			stored.ID = existing.ID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Save(&stored).Error
	}); err != nil {
		return err
	}
	s.apply(settings)
	return nil
}

func (s *Service) apply(settings InfrastructureSettings) {
	if !settings.AMQPEnabled {
		settings.AMQPURL = ""
	}
	if !settings.ProxyEnabled {
		settings.ProxyProtocol = ""
		settings.ProxyHost = ""
		settings.ProxyPort = ""
		settings.ProxyUsername = ""
		settings.ProxyPassword = ""
	}
	if !settings.MinioEnabled {
		settings.MinioEndpoint = ""
		settings.MinioAccessKey = ""
		settings.MinioSecretKey = ""
		settings.MinioBucket = ""
	}
	s.config.AmqpUrl = settings.AMQPURL
	s.config.AmqpGlobalEnabled = settings.AMQPGlobalEnabled
	s.config.WebhookUrl = settings.WebhookURL
	s.config.ProxyProtocol = settings.ProxyProtocol
	s.config.ProxyHost = settings.ProxyHost
	s.config.ProxyPort = settings.ProxyPort
	s.config.ProxyUsername = settings.ProxyUsername
	s.config.ProxyPassword = settings.ProxyPassword
	s.config.MinioEnabled = settings.MinioEnabled
	s.config.MinioEndpoint = settings.MinioEndpoint
	s.config.MinioAccessKey = settings.MinioAccessKey
	s.config.MinioSecretKey = settings.MinioSecretKey
	s.config.MinioBucket = settings.MinioBucket
	s.config.MinioUseSSL = settings.MinioUseSSL
}

func (s *Service) encode(settings InfrastructureSettings) (StoredInfrastructureSettings, error) {
	amqpURL, err := s.encrypt(settings.AMQPURL)
	if err != nil {
		return StoredInfrastructureSettings{}, err
	}
	webhookURL, err := s.encrypt(settings.WebhookURL)
	if err != nil {
		return StoredInfrastructureSettings{}, err
	}
	proxyHost, err := s.encrypt(settings.ProxyHost)
	if err != nil {
		return StoredInfrastructureSettings{}, err
	}
	proxyPort, err := s.encrypt(settings.ProxyPort)
	if err != nil {
		return StoredInfrastructureSettings{}, err
	}
	proxyUser, err := s.encrypt(settings.ProxyUsername)
	if err != nil {
		return StoredInfrastructureSettings{}, err
	}
	proxyPass, err := s.encrypt(settings.ProxyPassword)
	if err != nil {
		return StoredInfrastructureSettings{}, err
	}
	minioEndpoint, err := s.encrypt(settings.MinioEndpoint)
	if err != nil {
		return StoredInfrastructureSettings{}, err
	}
	minioAccess, err := s.encrypt(settings.MinioAccessKey)
	if err != nil {
		return StoredInfrastructureSettings{}, err
	}
	minioSecret, err := s.encrypt(settings.MinioSecretKey)
	if err != nil {
		return StoredInfrastructureSettings{}, err
	}
	minioBucket, err := s.encrypt(settings.MinioBucket)
	if err != nil {
		return StoredInfrastructureSettings{}, err
	}
	return StoredInfrastructureSettings{
		AMQPEnabled: settings.AMQPEnabled, AMQPURLEncrypted: amqpURL, AMQPGlobalEnabled: settings.AMQPGlobalEnabled,
		WebhookURLEncrypted: webhookURL, ProxyEnabled: settings.ProxyEnabled, ProxyProtocol: settings.ProxyProtocol,
		ProxyHostEncrypted: proxyHost, ProxyPortEncrypted: proxyPort, ProxyUserEncrypted: proxyUser, ProxyPassEncrypted: proxyPass,
		MinioEnabled: settings.MinioEnabled, MinioEndEncrypted: minioEndpoint, MinioAccessEncrypted: minioAccess,
		MinioSecretEncrypted: minioSecret, MinioBucketEncrypted: minioBucket, MinioUseSSL: settings.MinioUseSSL,
	}, nil
}

func (s *Service) decode(stored StoredInfrastructureSettings) (InfrastructureSettings, error) {
	amqpURL, err := s.decrypt(stored.AMQPURLEncrypted)
	if err != nil {
		return InfrastructureSettings{}, err
	}
	webhookURL, err := s.decrypt(stored.WebhookURLEncrypted)
	if err != nil {
		return InfrastructureSettings{}, err
	}
	proxyHost, err := s.decrypt(stored.ProxyHostEncrypted)
	if err != nil {
		return InfrastructureSettings{}, err
	}
	proxyPort, err := s.decrypt(stored.ProxyPortEncrypted)
	if err != nil {
		return InfrastructureSettings{}, err
	}
	proxyUser, err := s.decrypt(stored.ProxyUserEncrypted)
	if err != nil {
		return InfrastructureSettings{}, err
	}
	proxyPass, err := s.decrypt(stored.ProxyPassEncrypted)
	if err != nil {
		return InfrastructureSettings{}, err
	}
	minioEndpoint, err := s.decrypt(stored.MinioEndEncrypted)
	if err != nil {
		return InfrastructureSettings{}, err
	}
	minioAccess, err := s.decrypt(stored.MinioAccessEncrypted)
	if err != nil {
		return InfrastructureSettings{}, err
	}
	minioSecret, err := s.decrypt(stored.MinioSecretEncrypted)
	if err != nil {
		return InfrastructureSettings{}, err
	}
	minioBucket, err := s.decrypt(stored.MinioBucketEncrypted)
	if err != nil {
		return InfrastructureSettings{}, err
	}
	return InfrastructureSettings{
		AMQPEnabled: stored.AMQPEnabled, AMQPURL: amqpURL, AMQPGlobalEnabled: stored.AMQPGlobalEnabled, WebhookURL: webhookURL,
		ProxyEnabled: stored.ProxyEnabled, ProxyProtocol: stored.ProxyProtocol, ProxyHost: proxyHost, ProxyPort: proxyPort,
		ProxyUsername: proxyUser, ProxyPassword: proxyPass, MinioEnabled: stored.MinioEnabled, MinioEndpoint: minioEndpoint,
		MinioAccessKey: minioAccess, MinioSecretKey: minioSecret, MinioBucket: minioBucket, MinioUseSSL: stored.MinioUseSSL,
	}, nil
}

func (s *Service) encrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(append(nonce, gcm.Seal(nil, nonce, []byte(value), nil)...)), nil
}

func (s *Service) decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	encoded, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("decode infrastructure setting: %w", err)
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(encoded) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted infrastructure setting")
	}
	plain, err := gcm.Open(nil, encoded[:gcm.NonceSize()], encoded[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("could not decrypt infrastructure setting")
	}
	return string(plain), nil
}

func validate(settings InfrastructureSettings) error {
	if settings.AMQPEnabled {
		parsed, err := url.Parse(settings.AMQPURL)
		if err != nil || (parsed.Scheme != "amqp" && parsed.Scheme != "amqps") || parsed.Host == "" {
			return errors.New("informe uma URL AMQP válida")
		}
	}
	if settings.WebhookURL != "" {
		parsed, err := url.ParseRequestURI(settings.WebhookURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return errors.New("informe uma URL de webhook válida")
		}
	}
	if settings.ProxyEnabled {
		switch settings.ProxyProtocol {
		case "http", "https", "socks5":
		default:
			return errors.New("protocolo de proxy inválido")
		}
		if settings.ProxyHost == "" || settings.ProxyPort == "" || settings.ProxyUsername == "" || settings.ProxyPassword == "" {
			return errors.New("informe host, porta, usuário e senha do proxy")
		}
	}
	if settings.MinioEnabled && (settings.MinioEndpoint == "" || settings.MinioAccessKey == "" || settings.MinioSecretKey == "" || settings.MinioBucket == "") {
		return errors.New("informe endpoint, chave de acesso, segredo e bucket do MinIO")
	}
	return nil
}

type Handler struct {
	service *Service
	auth    *managerauth.Service
}

func NewHandler(service *Service, auth *managerauth.Service) *Handler {
	return &Handler{service: service, auth: auth}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	group := engine.Group("/manager-v2/settings")
	group.Use(func(ctx *gin.Context) {
		if !h.auth.IsAuthorized(ctx.Request.Context(), ctx.Request) {
			ctx.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		ctx.Next()
	})
	group.GET("/infrastructure", func(ctx *gin.Context) { ctx.JSON(200, h.service.Current()) })
	group.PUT("/infrastructure", func(ctx *gin.Context) {
		var settings InfrastructureSettings
		if err := ctx.ShouldBindJSON(&settings); err != nil {
			ctx.JSON(400, gin.H{"error": "invalid infrastructure settings"})
			return
		}
		if err := h.service.Save(ctx.Request.Context(), settings); err != nil {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(200, gin.H{"message": "saved", "restartRequired": true})
	})
}
