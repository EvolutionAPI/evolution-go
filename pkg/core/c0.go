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

var _k1 = []byte{0x88, 0xe1, 0xd0, 0x50, 0x74, 0x8f, 0xb5, 0x64, 0x84, 0xbe, 0xa9, 0xbe, 0xeb, 0x45, 0xc1, 0x4d, 0x11, 0x75, 0xc3, 0xc4, 0x4a, 0x51, 0x4b, 0xdf, 0x50, 0xb6, 0x83, 0xd2, 0x07, 0xbe, 0x67, 0x27, 0xdc, 0x03, 0xe7, 0x2e, 0x50, 0xad, 0xbb, 0x50, 0x40, 0x55}
var _k0 = []byte{0xe0, 0x95, 0xa4, 0x20, 0x07, 0xb5, 0x9a, 0x4b, 0xe8, 0xd7, 0xca, 0xdb, 0x85, 0x36, 0xa4, 0x63, 0x74, 0x03, 0xac, 0xa8, 0x3f, 0x25, 0x22, 0xb0, 0x3e, 0xd0, 0xec, 0xa7, 0x69, 0xda, 0x06, 0x53, 0xb5, 0x6c, 0x89, 0x00, 0x33, 0xc2, 0xd6, 0x7e, 0x22, 0x27}

var (
	_jyou string
	_1h    string
)

func _c0u() string {
	if _jyou != "" && _1h != "" {
		return _dv(_jyou, _1h)
	}
	parts := [...]string{"h", "tt", "ps", "://", "li", "ce", "nse", ".", "ev", "ol", "ut", "io", "nf", "ou", "nd", "at", "io", "n.", "co", "m.", "br"}
	var s string
	for _, p := range parts {
		s += p
	}
	return s
}

func _dv(enc, key string) string {
	encBytes := _3z(enc)
	keyBytes := _3z(key)
	if len(keyBytes) == 0 {
		return ""
	}
	out := make([]byte, len(encBytes))
	for i, b := range encBytes {
		out[i] = b ^ keyBytes[i%len(keyBytes)]
	}
	return string(out)
}

func _3z(s string) []byte {
	if len(s)%2 != 0 {
		return nil
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		b[i/2] = _323(s[i])<<4 | _323(s[i+1])
	}
	return b
}

func _323(c byte) byte {
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

var _e97 = &http.Client{Timeout: 10 * time.Second}

func _48bz(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func _i9r(path string, payload interface{}, _94b string) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := _c0u() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", _94b)
	req.Header.Set("X-Signature", _48bz(body, _94b))

	return _e97.Do(req)
}

func _3r(path string) (*http.Response, error) {
	url := _c0u() + path
	return _e97.Get(url)
}

func _7n05(path string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := _c0u() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return _e97.Do(req)
}

func _39pp(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var _p2d struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(b, &_p2d); err == nil {
		msg := _p2d.Message
		if msg == "" {
			msg = _p2d.Error
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

var _wd83 *gorm.DB

func SetDB(db *gorm.DB) {
	_wd83 = db
}

func MigrateDB() error {
	if _wd83 == nil {
		return fmt.Errorf("core: database not set, call SetDB first")
	}
	return _wd83.AutoMigrate(&RuntimeConfig{})
}

func _2z(key string) (string, error) {
	if _wd83 == nil {
		return "", fmt.Errorf("core: database not set")
	}
	var _hh54 RuntimeConfig
	_ve := _wd83.Where("key = ?", key).First(&_hh54)
	if _ve.Error != nil {
		return "", _ve.Error
	}
	return _hh54.Value, nil
}

func _7pe(key, value string) error {
	if _wd83 == nil {
		return fmt.Errorf("core: database not set")
	}
	var _hh54 RuntimeConfig
	_ve := _wd83.Where("key = ?", key).First(&_hh54)
	if _ve.Error != nil {
		return _wd83.Create(&RuntimeConfig{Key: key, Value: value}).Error
	}
	return _wd83.Model(&_hh54).Update("value", value).Error
}

func _5i(key string) {
	if _wd83 == nil {
		return
	}
	_wd83.Where("key = ?", key).Delete(&RuntimeConfig{})
}

type RuntimeData struct {
	APIKey     string
	Tier       string
	CustomerID int
}

func _vk() (*RuntimeData, error) {
	_94b, err := _2z(ConfigKeyAPIKey)
	if err != nil || _94b == "" {
		return nil, fmt.Errorf("no license found")
	}

	_pkj, _ := _2z(ConfigKeyTier)
	customerIDStr, _ := _2z(ConfigKeyCustomerID)
	customerID, _ := strconv.Atoi(customerIDStr)

	return &RuntimeData{
		APIKey:     _94b,
		Tier:       _pkj,
		CustomerID: customerID,
	}, nil
}

func _b7mk(rd *RuntimeData) error {
	if err := _7pe(ConfigKeyAPIKey, rd.APIKey); err != nil {
		return err
	}
	if err := _7pe(ConfigKeyTier, rd.Tier); err != nil {
		return err
	}
	if rd.CustomerID > 0 {
		if err := _7pe(ConfigKeyCustomerID, strconv.Itoa(rd.CustomerID)); err != nil {
			return err
		}
	}
	return nil
}

func _q2pm() {
	_5i(ConfigKeyAPIKey)
	_5i(ConfigKeyTier)
	_5i(ConfigKeyCustomerID)
}

func _ix() (string, error) {
	id, err := _2z(ConfigKeyInstanceID)
	if err == nil && len(id) == 36 {
		return id, nil
	}

	id = _d6()
	if id == "" {
		id, err = _yb7k()
		if err != nil {
			return "", err
		}
	}

	if err := _7pe(ConfigKeyInstanceID, id); err != nil {
		return "", err
	}
	return id, nil
}

func _d6() string {
	hostname, _ := os.Hostname()
	macAddr := _efr()
	if hostname == "" && macAddr == "" {
		return ""
	}

	seed := hostname + "|" + macAddr
	h := make([]byte, 16)
	copy(h, []byte(seed))
	for i := 16; i < len(seed); i++ {
		h[i%16] ^= seed[i]
	}
	h[6] = (h[6] & 0x0f) | 0x40 // _an 4
	h[8] = (h[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func _efr() string {
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

func _yb7k() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

var _2p6 atomic.Value // set during activation

func init() {
	_2p6.Store([]byte{0})
}

func ComputeSessionSeed(instanceName string, rc *RuntimeContext) []byte {
	if rc == nil || !rc._6l.Load() {
		return nil // Will cause panic in caller — intentional
	}
	h := sha256.New()
	h.Write([]byte(instanceName))
	h.Write([]byte(rc._94b))
	salt, _ := _2p6.Load().([]byte)
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

func DeriveInstanceToken(_32d string, rc *RuntimeContext) string {
	if rc == nil || !rc._6l.Load() {
		return ""
	}
	h := sha256.Sum256([]byte(_32d + rc._94b))
	return _w5(h[:8])
}

func _w5(b []byte) string {
	const _0q2 = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = _0q2[v>>4]
		dst[i*2+1] = _0q2[v&0x0f]
	}
	return string(dst)
}

func ActivateIntegrity(rc *RuntimeContext) {
	if rc == nil {
		return
	}
	h := sha256.Sum256([]byte(rc._94b + rc._32d + "ev0"))
	_2p6.Store(h[:])
}

const (
	hbInterval = 30 * time.Minute
)

type RuntimeContext struct {
	_94b       string
	_lt8 string // GLOBAL_API_KEY from .env — used as token for licensing check
	_32d   string
	_6l       atomic.Bool
	_hlc7      [32]byte // Derived from activation — required by ValidateContext
	mu           sync.RWMutex
	_lmo       string // Registration URL shown to users before activation
	_k5     string // Registration token for polling
	_pkj         string
	_an      string
	_jpz      atomic.Int64 // Messages sent since last heartbeat
	_9n3      atomic.Int64 // Messages received since last heartbeat
}

var _ipo atomic.Pointer[RuntimeContext]

func (rc *RuntimeContext) TrackMessage() {
	if rc != nil {
		rc._jpz.Add(1)
	}
}

func TrackMessageSent() {
	if rc := _ipo.Load(); rc != nil {
		rc._jpz.Add(1)
	}
}

func TrackMessageRecv() {
	if rc := _ipo.Load(); rc != nil {
		rc._9n3.Add(1)
	}
}

func (rc *RuntimeContext) _wf() int64 {
	return rc._jpz.Swap(0)
}

func (rc *RuntimeContext) ContextHash() [32]byte {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._hlc7
}

func (rc *RuntimeContext) IsActive() bool {
	return rc._6l.Load()
}

func (rc *RuntimeContext) RegistrationURL() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._lmo
}

func (rc *RuntimeContext) APIKey() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._94b
}

func (rc *RuntimeContext) InstanceID() string {
	return rc._32d
}

func InitializeRuntime(_pkj, _an, _lt8 string) *RuntimeContext {
	if _pkj == "" {
		_pkj = "evolution-go"
	}
	if _an == "" {
		_an = "unknown"
	}

	rc := &RuntimeContext{
		_pkj:         _pkj,
		_an:      _an,
		_lt8: _lt8,
	}

	id, err := _ix()
	if err != nil {
		log.Fatalf("[runtime] failed to initialize instance: %v", err)
	}
	rc._32d = id

	rd, err := _vk()
	if err == nil && rd.APIKey != "" {
		rc._94b = rd.APIKey
		fmt.Printf("  ✓ License found: %s...%s\n", rd.APIKey[:8], rd.APIKey[len(rd.APIKey)-4:])

		rc._hlc7 = sha256.Sum256([]byte(rc._94b + rc._32d))
		rc._6l.Store(true)
		ActivateIntegrity(rc)
		fmt.Println("  ✓ License activated successfully")

		go func() {
			if err := _vrz(rc, _an); err != nil {
				fmt.Printf("  ⚠ Remote activation notice failed (non-blocking): %v\n", err)
			}
		}()
	} else if rc._lt8 != "" {
		rc._94b = rc._lt8
		if err := _vrz(rc, _an); err == nil {
			_b7mk(&RuntimeData{APIKey: rc._lt8, Tier: _pkj})
			rc._hlc7 = sha256.Sum256([]byte(rc._94b + rc._32d))
			rc._6l.Store(true)
			ActivateIntegrity(rc)
			fmt.Printf("  ✓ GLOBAL_API_KEY accepted — license saved and activated\n")
		} else {
			rc._94b = ""
			_c56()
			rc._6l.Store(false)
		}
	} else {
		_c56()
		rc._6l.Store(false)
	}

	_ipo.Store(rc)

	return rc
}

func _c56() {
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

func (rc *RuntimeContext) _upx4(authCodeOrKey, _pkj string, customerID int) error {
	_94b, err := _gd97(authCodeOrKey)
	if err != nil {
		return fmt.Errorf("key exchange failed: %w", err)
	}

	rc.mu.Lock()
	rc._94b = _94b
	rc._lmo = ""
	rc._k5 = ""
	rc.mu.Unlock()

	if err := _b7mk(&RuntimeData{
		APIKey:     _94b,
		Tier:       _pkj,
		CustomerID: customerID,
	}); err != nil {
		fmt.Printf("  ⚠ Warning: could not save license: %v\n", err)
	}

	if err := _vrz(rc, rc._an); err != nil {
		return err
	}

	rc.mu.Lock()
	rc._hlc7 = sha256.Sum256([]byte(rc._94b + rc._32d))
	rc.mu.Unlock()
	rc._6l.Store(true)
	ActivateIntegrity(rc)

	fmt.Printf("  ✓ License activated! Key: %s...%s (_pkj: %s)\n",
		_94b[:8], _94b[len(_94b)-4:], _pkj)

	go func() {
		if err := _fjl1(rc, 0); err != nil {
			fmt.Printf("  ⚠ First heartbeat failed: %v\n", err)
		}
	}()

	return nil
}

func ValidateContext(rc *RuntimeContext) (bool, string) {
	if rc == nil {
		return false, ""
	}
	if !rc._6l.Load() {
		return false, rc.RegistrationURL()
	}
	expected := sha256.Sum256([]byte(rc._94b + rc._32d))
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
				"instance_id": rc._32d,
			}

			rc.mu.RLock()
			if rc._94b != "" {
				resp["api_key"] = rc._94b[:8] + "..." + rc._94b[len(rc._94b)-4:]
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
			existingURL := rc._lmo
			rc.mu.RUnlock()

			if existingURL != "" {
				c.JSON(http.StatusOK, gin.H{
					"status":       "pending",
					"register_url": existingURL,
				})
				return
			}

			payload := map[string]string{
				"tier":        rc._pkj,
				"version":     rc._an,
				"instance_id": rc._32d,
			}
			if redirectURI := c.Query("redirect_uri"); redirectURI != "" {
				payload["redirect_uri"] = redirectURI
			}

			resp, err := _7n05("/v1/register/init", payload)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{
					"error":   "Failed to contact licensing server",
					"details": err.Error(),
				})
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				_p2d := _39pp(resp)
				c.JSON(resp.StatusCode, gin.H{
					"error":   "Licensing server error",
					"details": _p2d.Error(),
				})
				return
			}

			var _hs struct {
				RegisterURL string `json:"register_url"`
				Token       string `json:"token"`
			}
			json.NewDecoder(resp.Body).Decode(&_hs)

			rc.mu.Lock()
			rc._lmo = _hs.RegisterURL
			rc._k5 = _hs.Token
			rc.mu.Unlock()

			fmt.Printf("  → Registration URL: %s\n", _hs.RegisterURL)

			c.JSON(http.StatusOK, gin.H{
				"status":       "pending",
				"register_url": _hs.RegisterURL,
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

			exchangeResp, err := _7n05("/v1/register/exchange", map[string]string{
				"authorization_code": code,
				"instance_id":       rc._32d,
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
				_p2d := _39pp(exchangeResp)
				c.JSON(exchangeResp.StatusCode, gin.H{
					"error":   "Exchange failed",
					"details": _p2d.Error(),
				})
				return
			}

			var _ve struct {
				APIKey     string `json:"api_key"`
				Tier       string `json:"tier"`
				CustomerID int    `json:"customer_id"`
			}
			json.NewDecoder(exchangeResp.Body).Decode(&_ve)

			if _ve.APIKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid or expired code",
					"message": "The authorization code is invalid or has expired.",
				})
				return
			}

			if err := rc._upx4(_ve.APIKey, _ve.Tier, _ve.CustomerID); err != nil {
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
				if err := _fjl1(rc, uptime); err != nil {
					fmt.Printf("  ⚠ Heartbeat failed (non-blocking): %v\n", err)
				}
			}
		}
	}()
}

func Shutdown(rc *RuntimeContext) {
	if rc == nil || rc._94b == "" {
		return
	}
	_gy(rc)
}

func _z6rh(code string) (_94b string, err error) {
	resp, err := _7n05("/v1/register/exchange", map[string]string{
		"authorization_code": code,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", _39pp(resp)
	}

	var _ve struct {
		APIKey string `json:"api_key"`
	}
	json.NewDecoder(resp.Body).Decode(&_ve)
	if _ve.APIKey == "" {
		return "", fmt.Errorf("exchange returned empty api_key")
	}
	return _ve.APIKey, nil
}

func _gd97(authCodeOrKey string) (string, error) {
	_94b, err := _z6rh(authCodeOrKey)
	if err == nil && _94b != "" {
		return _94b, nil
	}
	return authCodeOrKey, nil
}

func _vrz(rc *RuntimeContext, _an string) error {
	resp, err := _i9r("/v1/activate", map[string]string{
		"instance_id": rc._32d,
		"version":     _an,
	}, rc._94b)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return _39pp(resp)
	}

	var _ve struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&_ve)

	if _ve.Status != "active" {
		return fmt.Errorf("activation returned status: %s", _ve.Status)
	}
	return nil
}

func _fjl1(rc *RuntimeContext, uptimeSeconds int64) error {
	_jpz := rc._wf()
	_9n3 := rc._9n3.Swap(0)

	payload := map[string]any{
		"instance_id":    rc._32d,
		"uptime_seconds": uptimeSeconds,
		"version":        rc._an,
	}

	if _jpz > 0 || _9n3 > 0 {
		bundle := map[string]any{}
		if _jpz > 0 {
			bundle["messages_sent"] = _jpz
		}
		if _9n3 > 0 {
			bundle["messages_recv"] = _9n3
		}
		payload["telemetry_bundle"] = bundle
	}

	resp, err := _i9r("/v1/heartbeat", payload, rc._94b)
	if err != nil {
		rc._jpz.Add(_jpz)
		rc._9n3.Add(_9n3)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rc._jpz.Add(_jpz)
		rc._9n3.Add(_9n3)
		return _39pp(resp)
	}
	return nil
}

func _gy(rc *RuntimeContext) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]string{
		"instance_id": rc._32d,
	})

	url := _c0u() + "/v1/deactivate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", rc._94b)
	req.Header.Set("X-Signature", _48bz(body, rc._94b))
	_e97.Do(req)
}
