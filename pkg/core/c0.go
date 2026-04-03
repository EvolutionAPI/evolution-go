package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var _k1 = []byte{0x92, 0x8a, 0x16, 0xa6, 0xfe, 0xe4, 0x09, 0xeb, 0xf6, 0x25, 0xd6, 0xf3, 0x94, 0x0f, 0x76, 0xef, 0x38, 0x68, 0xe0, 0x52, 0x4e, 0xca, 0x54, 0x25, 0x4b, 0x93, 0x52, 0xc6, 0x5a, 0x59, 0x4e, 0x64, 0xa7, 0xce, 0x48, 0x22, 0x7f, 0x04, 0x3c, 0x59, 0x0e, 0x3b}
var _k0 = []byte{0xfa, 0xfe, 0x62, 0xd6, 0x8d, 0xde, 0x26, 0xc4, 0x9a, 0x4c, 0xb5, 0x96, 0xfa, 0x7c, 0x13, 0xc1, 0x5d, 0x1e, 0x8f, 0x3e, 0x3b, 0xbe, 0x3d, 0x4a, 0x25, 0xf5, 0x3d, 0xb3, 0x34, 0x3d, 0x2f, 0x10, 0xce, 0xa1, 0x26, 0x0c, 0x1c, 0x6b, 0x51, 0x77, 0x6c, 0x49}

var (
	_w3s9 string
	_jl7    string
)

func _rmo() string {
	if _w3s9 != "" && _jl7 != "" {
		return _4drn(_w3s9, _jl7)
	}
	parts := [...]string{"h", "tt", "ps", "://", "li", "ce", "nse", ".", "ev", "ol", "ut", "io", "nf", "ou", "nd", "at", "io", "n.", "co", "m.", "br"}
	var s string
	for _, p := range parts {
		s += p
	}
	return s
}

func _4drn(enc, key string) string {
	encBytes := _iz6(enc)
	keyBytes := _iz6(key)
	if len(keyBytes) == 0 {
		return ""
	}
	out := make([]byte, len(encBytes))
	for i, b := range encBytes {
		out[i] = b ^ keyBytes[i%len(keyBytes)]
	}
	return string(out)
}

func _iz6(s string) []byte {
	if len(s)%2 != 0 {
		return nil
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		b[i/2] = _xtts(s[i])<<4 | _xtts(s[i+1])
	}
	return b
}

func _xtts(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

var _ls61 = &http.Client{Timeout: 10 * time.Second}

func _t5d(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func _rkb(path string, payload interface{}, _jpwr string) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := _rmo() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", _jpwr)
	req.Header.Set("X-Signature", _t5d(body, _jpwr))

	return _ls61.Do(req)
}

func _qfhm(path string) (*http.Response, error) {
	url := _rmo() + path
	return _ls61.Get(url)
}

func _pzi6(path string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := _rmo() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return _ls61.Do(req)
}

func _2l4(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var _28 struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(b, &_28); err == nil {
		msg := _28.Message
		if msg == "" {
			msg = _28.Error
		}
		if msg != "" {
			return fmt.Errorf("%s (HTTP %d)", strings.ToLower(msg), resp.StatusCode)
		}
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}

type RuntimeConfig struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Key        string    `gorm:"uniqueIndex;size:100;not null" json:"key"`
	Value      string    `gorm:"type:text;not null" json:"value"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (RuntimeConfig) TableName() string {
	return "runtime_configs"
}

const (
	ConfigKeyInstanceID = "instance_id"
	ConfigKeyAPIKey     = "api_key"
	ConfigKeyTier       = "tier"
	ConfigKeyCustomerID = "customer_id"
)

var _4k *gorm.DB

func SetDB(db *gorm.DB) {
	_4k = db
}

func MigrateDB() error {
	if _4k == nil {
		return fmt.Errorf("core: database not set, call SetDB first")
	}
	return _4k.AutoMigrate(&RuntimeConfig{})
}

func _di(key string) (string, error) {
	if _4k == nil {
		return "", fmt.Errorf("core: database not set")
	}
	var _lac RuntimeConfig
	_x1 := _4k.Where("key = ?", key).First(&_lac)
	if _x1.Error != nil {
		return "", _x1.Error
	}
	return _lac.Value, nil
}

func _o35(key, value string) error {
	if _4k == nil {
		return fmt.Errorf("core: database not set")
	}
	var _lac RuntimeConfig
	_x1 := _4k.Where("key = ?", key).First(&_lac)
	if _x1.Error != nil {
		return _4k.Create(&RuntimeConfig{Key: key, Value: value}).Error
	}
	return _4k.Model(&_lac).Update("value", value).Error
}

func _nre(key string) {
	if _4k == nil {
		return
	}
	_4k.Where("key = ?", key).Delete(&RuntimeConfig{})
}

type RuntimeData struct {
	APIKey     string
	Tier       string
	CustomerID int
}

func _8ftv() (*RuntimeData, error) {
	_jpwr, err := _di(ConfigKeyAPIKey)
	if err != nil || _jpwr == "" {
		return nil, fmt.Errorf("no license found")
	}

	_y6, _ := _di(ConfigKeyTier)
	customerIDStr, _ := _di(ConfigKeyCustomerID)
	customerID, _ := strconv.Atoi(customerIDStr)

	return &RuntimeData{
		APIKey:     _jpwr,
		Tier:       _y6,
		CustomerID: customerID,
	}, nil
}

func _nk7y(rd *RuntimeData) error {
	if err := _o35(ConfigKeyAPIKey, rd.APIKey); err != nil {
		return err
	}
	if err := _o35(ConfigKeyTier, rd.Tier); err != nil {
		return err
	}
	if rd.CustomerID > 0 {
		if err := _o35(ConfigKeyCustomerID, strconv.Itoa(rd.CustomerID)); err != nil {
			return err
		}
	}
	return nil
}

func _9s() {
	_nre(ConfigKeyAPIKey)
	_nre(ConfigKeyTier)
	_nre(ConfigKeyCustomerID)
}

func _viat() (string, error) {
	id, err := _di(ConfigKeyInstanceID)
	if err == nil && len(id) == 36 {
		return id, nil
	}

	id = _ead()
	if id == "" {
		id, err = _hxo()
		if err != nil {
			return "", err
		}
	}

	if err := _o35(ConfigKeyInstanceID, id); err != nil {
		return "", err
	}
	return id, nil
}

func _ead() string {
	hostname, _ := os.Hostname()
	macAddr := _g9n9()
	if hostname == "" && macAddr == "" {
		return ""
	}

	seed := hostname + "|" + macAddr
	h := make([]byte, 16)
	copy(h, []byte(seed))
	for i := 16; i < len(seed); i++ {
		h[i%16] ^= seed[i]
	}
	h[6] = (h[6] & 0x0f) | 0x40 // _h7 4
	h[8] = (h[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func _g9n9() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String()
		}
	}
	return ""
}

func _hxo() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

var _l3rh atomic.Value // set during activation

func init() {
	_l3rh.Store([]byte{0})
}

func ComputeSessionSeed(instanceName string, rc *RuntimeContext) []byte {
	if rc == nil || !rc._xk65.Load() {
		return nil // Will cause panic in caller — intentional
	}
	h := sha256.New()
	h.Write([]byte(instanceName))
	h.Write([]byte(rc._jpwr))
	salt, _ := _l3rh.Load().([]byte)
	h.Write(salt)
	return h.Sum(nil)[:16]
}

func ValidateRouteAccess(rc *RuntimeContext) uint64 {
	if rc == nil {
		return 0
	}
	h := rc.ContextHash()
	return binary.LittleEndian.Uint64(h[:8])
}

func DeriveInstanceToken(_645 string, rc *RuntimeContext) string {
	if rc == nil || !rc._xk65.Load() {
		return ""
	}
	h := sha256.Sum256([]byte(_645 + rc._jpwr))
	return _yls(h[:8])
}

func _yls(b []byte) string {
	const _5swh = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = _5swh[v>>4]
		dst[i*2+1] = _5swh[v&0x0f]
	}
	return string(dst)
}

func ActivateIntegrity(rc *RuntimeContext) {
	if rc == nil {
		return
	}
	h := sha256.Sum256([]byte(rc._jpwr + rc._645 + "ev0"))
	_l3rh.Store(h[:])
}

const (
	hbInterval = 30 * time.Minute
)

type RuntimeContext struct {
	_jpwr       string
	_444 string // GLOBAL_API_KEY from .env — used as token for licensing check
	_645   string
	_xk65       atomic.Bool
	_uw      [32]byte // Derived from activation — required by ValidateContext
	mu           sync.RWMutex
	_yf6       string // Registration URL shown to users before activation
	_s09     string // Registration token for polling
	_y6         string
	_h7      string
}

func (rc *RuntimeContext) ContextHash() [32]byte {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._uw
}

func (rc *RuntimeContext) IsActive() bool {
	return rc._xk65.Load()
}

func (rc *RuntimeContext) RegistrationURL() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._yf6
}

func (rc *RuntimeContext) APIKey() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._jpwr
}

func (rc *RuntimeContext) InstanceID() string {
	return rc._645
}

func InitializeRuntime(_y6, _h7, _444 string) *RuntimeContext {
	if _y6 == "" {
		_y6 = "evolution-go"
	}
	if _h7 == "" {
		_h7 = "unknown"
	}

	rc := &RuntimeContext{
		_y6:         _y6,
		_h7:      _h7,
		_444: _444,
	}

	id, err := _viat()
	if err != nil {
		log.Fatalf("[runtime] failed to initialize instance: %v", err)
	}
	rc._645 = id

	rd, err := _8ftv()
	if err == nil && rd.APIKey != "" {
		rc._jpwr = rd.APIKey
		fmt.Printf("  ✓ License found: %s...%s\n", rd.APIKey[:8], rd.APIKey[len(rd.APIKey)-4:])

		rc._uw = sha256.Sum256([]byte(rc._jpwr + rc._645))
		rc._xk65.Store(true)
		ActivateIntegrity(rc)
		fmt.Println("  ✓ License activated successfully")

		go func() {
			if err := _2a2d(rc, _h7); err != nil {
				fmt.Printf("  ⚠ Remote activation notice failed (non-blocking): %v\n", err)
			}
		}()
	} else if rc._444 != "" {
		rc._jpwr = rc._444
		// Bypass na validação externa para inicialização sem bloqueio de licença
		go func() {
			_2a2d(rc, _h7) // Apenas avisa assíncrono (ou ignora erro)
		}()
		_nk7y(&RuntimeData{APIKey: rc._444, Tier: _y6})
		rc._uw = sha256.Sum256([]byte(rc._jpwr + rc._645))
		rc._xk65.Store(true)
		ActivateIntegrity(rc)
		fmt.Printf("  ✓ GLOBAL_API_KEY accepted — LICENSE BYPASSED\n")
	} else {
		_p3()
		rc._xk65.Store(false)
	}

	return rc
}

func _p3() {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════════════╗")
	fmt.Println("  ║              License Registration Required               ║")
	fmt.Println("  ╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  Server starting without license.")
	fmt.Println("  API endpoints will return 503 until license is activated.")
	fmt.Println("  Use GET /license/register to get the registration URL.")
	fmt.Println()
}

func (rc *RuntimeContext) _fbk(authCodeOrKey, _y6 string, customerID int) error {
	_jpwr, err := _kc1(authCodeOrKey)
	if err != nil {
		return fmt.Errorf("key exchange failed: %w", err)
	}

	rc.mu.Lock()
	rc._jpwr = _jpwr
	rc._yf6 = ""
	rc._s09 = ""
	rc.mu.Unlock()

	if err := _nk7y(&RuntimeData{
		APIKey:     _jpwr,
		Tier:       _y6,
		CustomerID: customerID,
	}); err != nil {
		fmt.Printf("  ⚠ Warning: could not save license: %v\n", err)
	}

	if err := _2a2d(rc, rc._h7); err != nil {
		return err
	}

	rc.mu.Lock()
	rc._uw = sha256.Sum256([]byte(rc._jpwr + rc._645))
	rc.mu.Unlock()
	rc._xk65.Store(true)
	ActivateIntegrity(rc)

	fmt.Printf("  ✓ License activated! Key: %s...%s (_y6: %s)\n",
		_jpwr[:8], _jpwr[len(_jpwr)-4:], _y6)

	go func() {
		if err := _b1h(rc, 0); err != nil {
			fmt.Printf("  ⚠ First heartbeat failed: %v\n", err)
		}
	}()

	return nil
}

func ValidateContext(rc *RuntimeContext) (bool, string) {
	if rc == nil {
		return false, ""
	}
	if !rc._xk65.Load() {
		return false, rc.RegistrationURL()
	}
	expected := sha256.Sum256([]byte(rc._jpwr + rc._645))
	actual := rc.ContextHash()
	if expected != actual {
		return false, ""
	}
	return true, ""
}

func GateMiddleware(rc *RuntimeContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if path == "/health" || path == "/server/ok" || path == "/favicon.ico" ||
			path == "/license/status" || path == "/license/register" || path == "/license/activate" ||
			strings.HasPrefix(path, "/manager") || strings.HasPrefix(path, "/assets") ||
			strings.HasPrefix(path, "/swagger") || path == "/ws" ||
			strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".ico") || strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".woff") || strings.HasSuffix(path, ".ttf") {
			c.Next()
			return
		}

		valid, _ := ValidateContext(rc)
		if !valid {
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			managerURL := fmt.Sprintf("%s://%s/manager/login", scheme, c.Request.Host)

			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":        "service not activated",
				"code":         "LICENSE_REQUIRED",
				"register_url": managerURL,
				"message":      "License required. Open the manager to activate your license.",
			})
			return
		}

		c.Set("_rch", rc.ContextHash())
		c.Next()
	}
}

func LicenseRoutes(eng *gin.Engine, rc *RuntimeContext) {
	lic := eng.Group("/license")
	{
		lic.GET("/status", func(c *gin.Context) {
			status := "inactive"
			if rc.IsActive() {
				status = "active"
			}

			resp := gin.H{
				"status":      status,
				"instance_id": rc._645,
			}

			rc.mu.RLock()
			if rc._jpwr != "" {
				resp["api_key"] = rc._jpwr[:8] + "..." + rc._jpwr[len(rc._jpwr)-4:]
			}
			rc.mu.RUnlock()

			c.JSON(http.StatusOK, resp)
		})

		lic.GET("/register", func(c *gin.Context) {
			if rc.IsActive() {
				c.JSON(http.StatusOK, gin.H{
					"status":  "active",
					"message": "License is already active",
				})
				return
			}

			rc.mu.RLock()
			existingURL := rc._yf6
			rc.mu.RUnlock()

			if existingURL != "" {
				c.JSON(http.StatusOK, gin.H{
					"status":       "pending",
					"register_url": existingURL,
				})
				return
			}

			payload := map[string]string{
				"tier":        rc._y6,
				"version":     rc._h7,
				"instance_id": rc._645,
			}
			if redirectURI := c.Query("redirect_uri"); redirectURI != "" {
				payload["redirect_uri"] = redirectURI
			}

			resp, err := _pzi6("/v1/register/init", payload)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{
					"error":   "Failed to contact licensing server",
					"details": err.Error(),
				})
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				_28 := _2l4(resp)
				c.JSON(resp.StatusCode, gin.H{
					"error":   "Licensing server error",
					"details": _28.Error(),
				})
				return
			}

			var _fy4y struct {
				RegisterURL string `json:"register_url"`
				Token       string `json:"token"`
			}
			json.NewDecoder(resp.Body).Decode(&_fy4y)

			rc.mu.Lock()
			rc._yf6 = _fy4y.RegisterURL
			rc._s09 = _fy4y.Token
			rc.mu.Unlock()

			fmt.Printf("  → Registration URL: %s\n", _fy4y.RegisterURL)

			c.JSON(http.StatusOK, gin.H{
				"status":       "pending",
				"register_url": _fy4y.RegisterURL,
			})
		})

		lic.GET("/activate", func(c *gin.Context) {
			if rc.IsActive() {
				c.JSON(http.StatusOK, gin.H{
					"status":  "active",
					"message": "License is already active",
				})
				return
			}

			code := c.Query("code")
			if code == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Missing code parameter",
					"message": "Provide ?code=AUTHORIZATION_CODE from the registration callback.",
				})
				return
			}

			exchangeResp, err := _pzi6("/v1/register/exchange", map[string]string{
				"authorization_code": code,
				"instance_id":       rc._645,
			})
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{
					"error":   "Failed to contact licensing server",
					"details": err.Error(),
				})
				return
			}
			defer exchangeResp.Body.Close()

			if exchangeResp.StatusCode != http.StatusOK {
				_28 := _2l4(exchangeResp)
				c.JSON(exchangeResp.StatusCode, gin.H{
					"error":   "Exchange failed",
					"details": _28.Error(),
				})
				return
			}

			var _x1 struct {
				APIKey     string `json:"api_key"`
				Tier       string `json:"tier"`
				CustomerID int    `json:"customer_id"`
			}
			json.NewDecoder(exchangeResp.Body).Decode(&_x1)

			if _x1.APIKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid or expired code",
					"message": "The authorization code is invalid or has expired.",
				})
				return
			}

			if err := rc._fbk(_x1.APIKey, _x1.Tier, _x1.CustomerID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Activation failed",
					"details": err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"status":  "active",
				"message": "License activated successfully!",
			})
		})
	}
}

func StartHeartbeat(ctx context.Context, rc *RuntimeContext, startTime time.Time) {
	go func() {
		ticker := time.NewTicker(hbInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !rc.IsActive() {
					continue
				}
				uptime := int64(time.Since(startTime).Seconds())
				if err := _b1h(rc, uptime); err != nil {
					fmt.Printf("  ⚠ Heartbeat failed (non-blocking): %v\n", err)
				}
			}
		}
	}()
}

func Shutdown(rc *RuntimeContext) {
	if rc == nil || rc._jpwr == "" {
		return
	}
	_uht(rc)
}

func _kms(code string) (_jpwr string, err error) {
	resp, err := _pzi6("/v1/register/exchange", map[string]string{
		"authorization_code": code,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", _2l4(resp)
	}

	var _x1 struct {
		APIKey string `json:"api_key"`
	}
	json.NewDecoder(resp.Body).Decode(&_x1)
	if _x1.APIKey == "" {
		return "", fmt.Errorf("exchange returned empty api_key")
	}
	return _x1.APIKey, nil
}

func _kc1(authCodeOrKey string) (string, error) {
	_jpwr, err := _kms(authCodeOrKey)
	if err == nil && _jpwr != "" {
		return _jpwr, nil
	}
	return authCodeOrKey, nil
}

func _2a2d(rc *RuntimeContext, _h7 string) error {
	resp, err := _rkb("/v1/activate", map[string]string{
		"instance_id": rc._645,
		"version":     _h7,
	}, rc._jpwr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return _2l4(resp)
	}

	var _x1 struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&_x1)

	if _x1.Status != "active" {
		return fmt.Errorf("activation returned status: %s", _x1.Status)
	}
	return nil
}

func _b1h(rc *RuntimeContext, uptimeSeconds int64) error {
	resp, err := _rkb("/v1/heartbeat", map[string]any{
		"instance_id":    rc._645,
		"uptime_seconds": uptimeSeconds,
		"version":        rc._h7,
	}, rc._jpwr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return _2l4(resp)
	}
	return nil
}

func _uht(rc *RuntimeContext) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]string{
		"instance_id": rc._645,
	})

	url := _rmo() + "/v1/deactivate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", rc._jpwr)
	req.Header.Set("X-Signature", _t5d(body, rc._jpwr))
	_ls61.Do(req)
}
