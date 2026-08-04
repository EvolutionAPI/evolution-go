package managerauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	CookieName = "evolution_manager_v2_session"
	issuer     = "evolution-go-manager-v2"
	audience   = "evolution-go-manager-v2"
	tokenTTL   = 12 * time.Hour
)

var (
	ErrSetupComplete      = errors.New("administrator account already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// Administrator is the local account allowed to access the Manager V2.
// Its password is stored only as a bcrypt hash.
type Administrator struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey"`
	Name         string    `json:"name"`
	Email        string    `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (a *Administrator) BeforeCreate(_ *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}

type PublicAdministrator struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type Service struct {
	db         *gorm.DB
	signingKey []byte
}

func NewService(db *gorm.DB, secret string) *Service {
	return &Service{db: db, signingKey: []byte(secret)}
}

func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&Administrator{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *Service) Bootstrap(ctx context.Context, credentials Credentials) (*Administrator, error) {
	name, email, password, err := normalizeCredentials(credentials, true)
	if err != nil {
		return nil, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash administrator password: %w", err)
	}

	admin := &Administrator{Name: name, Email: email, PasswordHash: string(passwordHash)}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&Administrator{}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrSetupComplete
		}
		return tx.Create(admin).Error
	})
	if err != nil {
		return nil, err
	}
	return admin, nil
}

func (s *Service) Authenticate(ctx context.Context, credentials Credentials) (*Administrator, error) {
	_, email, password, err := normalizeCredentials(credentials, false)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	var admin Administrator
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return &admin, nil
}

func (s *Service) CreateToken(admin *Administrator) (string, time.Time, error) {
	expiresAt := time.Now().Add(tokenTTL)
	claims := Claims{
		Role: "administrator",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   admin.ID,
			Audience:  jwt.ClaimStrings{audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.signingKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign manager token: %w", err)
	}
	return signed, expiresAt, nil
}

func (s *Service) AdministratorFromRequest(ctx context.Context, request *http.Request) (*Administrator, error) {
	cookie, err := request.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		return nil, ErrInvalidCredentials
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidCredentials
		}
		return s.signingKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(issuer), jwt.WithAudience(audience), jwt.WithLeeway(30*time.Second))
	if err != nil || !token.Valid || claims.Role != "administrator" || claims.Subject == "" {
		return nil, ErrInvalidCredentials
	}

	var admin Administrator
	if err := s.db.WithContext(ctx).First(&admin, "id = ?", claims.Subject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	return &admin, nil
}

func (s *Service) IsAuthorized(ctx context.Context, request *http.Request) bool {
	_, err := s.AdministratorFromRequest(ctx, request)
	return err == nil
}

func Public(admin *Administrator) PublicAdministrator {
	return PublicAdministrator{ID: admin.ID, Name: admin.Name, Email: admin.Email}
}

func normalizeCredentials(credentials Credentials, requireName bool) (string, string, string, error) {
	name := strings.TrimSpace(credentials.Name)
	email := strings.ToLower(strings.TrimSpace(credentials.Email))
	password := credentials.Password

	if requireName && len(name) < 2 {
		return "", "", "", errors.New("name must have at least 2 characters")
	}
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(parsedEmail.Address, email) {
		return "", "", "", errors.New("a valid email is required")
	}
	if len(password) < 12 || len(password) > 72 {
		return "", "", "", errors.New("password must have between 12 and 72 characters")
	}
	return name, email, password, nil
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(engine *gin.Engine) {
	group := engine.Group("/manager-v2/auth")
	group.GET("/status", h.Status)
	group.POST("/setup", h.Setup)
	group.POST("/login", h.Login)
	group.POST("/logout", h.Logout)
}

func (h *Handler) Status(ctx *gin.Context) {
	setupRequired, err := h.service.SetupRequired(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not check manager setup"})
		return
	}

	response := gin.H{"setupRequired": setupRequired, "authenticated": false}
	if admin, err := h.service.AdministratorFromRequest(ctx.Request.Context(), ctx.Request); err == nil {
		response["authenticated"] = true
		response["user"] = Public(admin)
	}
	ctx.JSON(http.StatusOK, response)
}

func (h *Handler) Setup(ctx *gin.Context) {
	var credentials Credentials
	if err := ctx.ShouldBindJSON(&credentials); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid registration data"})
		return
	}

	admin, err := h.service.Bootstrap(ctx.Request.Context(), credentials)
	if err != nil {
		switch {
		case errors.Is(err, ErrSetupComplete):
			ctx.JSON(http.StatusConflict, gin.H{"error": "administrator account already configured"})
		default:
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	h.startSession(ctx, admin)
}

func (h *Handler) Login(ctx *gin.Context) {
	var credentials Credentials
	if err := ctx.ShouldBindJSON(&credentials); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid login data"})
		return
	}

	admin, err := h.service.Authenticate(ctx.Request.Context(), credentials)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "email or password is incorrect"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not sign in"})
		return
	}
	h.startSession(ctx, admin)
}

func (h *Handler) Logout(ctx *gin.Context) {
	h.clearSession(ctx)
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) startSession(ctx *gin.Context, admin *Administrator) {
	token, expiresAt, err := h.service.CreateToken(admin)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "could not create session"})
		return
	}
	ctx.SetSameSite(http.SameSiteStrictMode)
	ctx.SetCookie(CookieName, token, int(time.Until(expiresAt).Seconds()), "/", "", requestIsSecure(ctx.Request), true)
	ctx.JSON(http.StatusOK, gin.H{"user": Public(admin), "expiresAt": expiresAt.UTC()})
}

func (h *Handler) clearSession(ctx *gin.Context) {
	ctx.SetSameSite(http.SameSiteStrictMode)
	ctx.SetCookie(CookieName, "", -1, "/", "", requestIsSecure(ctx.Request), true)
}

func requestIsSecure(request *http.Request) bool {
	return request.TLS != nil || strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https")
}
