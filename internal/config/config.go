package config

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	projectRoot     string
	projectRootOnce sync.Once
)

type AppConfig struct {
	Environment       string
	Debug             bool
	AllowedOrigins    []string
	DatabaseDir       string
	MasterKeyDir      string
	BindAddress       string
	MaxDevicesPerUser int
	TrustedProxies    []string

	IntegritySeed     string
	DatabaseName      string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MasterKeyFilename string
	CertFilename      string
	KeyFilename       string

	// Limits
	HttpMaxBodyBytes    int64
	WsMaxMessageBytes   int64
	MaxEnvelopeBytes    int
	MaxMailboxBytes     int
	MaxMailboxMessages  int
	MaxGlobalWebSockets int
	DeliveryBatchSize   int

	DbMaxOpenConns    int
	DbMaxIdleConns    int
	DbConnMaxLifetime time.Duration

	SqliteBusyTimeoutMs int
	SqliteCacheSize     int
	SqliteMmapSizeBytes int64

	RequestContextTimeout time.Duration
	WsPingPeriod          time.Duration
	WsPongWait            time.Duration

	// Rate Limiting
	MaxConnPerIP       int
	MaxReqPerSecIP     int
	MaxMsgPerSec       int
	MaxDiscoveryPerSec int
	MaxAuthPerSecIP    int
}

func GetStrictEnvInt(key string, defaultValue int, min int, max int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("Critical: Invalid config value for %s: %s (must be an integer)", key, value)
	}
	if parsed < min {
		log.Fatalf("Critical: %s=%d is below safe minimum of %d", key, parsed, min)
	}
	if parsed > max {
		log.Fatalf("Critical: %s=%d is above safe maximum of %d", key, parsed, max)
	}
	return parsed
}

func GetStrictEnvInt64(key string, defaultValue int64, min int64, max int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		log.Fatalf("Critical: Invalid config value for %s: %s (must be an integer)", key, value)
	}
	if parsed < min {
		log.Fatalf("Critical: %s=%d is below safe minimum of %d", key, parsed, min)
	}
	if parsed > max {
		log.Fatalf("Critical: %s=%d is above safe maximum of %d", key, parsed, max)
	}
	return parsed
}

func LoadConfig() AppConfig {
	env := strings.ToLower(GetEnvOrDefault("NEONECT_ENV", "beta"))
	switch env {
	case "development", "beta", "production":
		// valid environments
	default:
		log.Fatalf("Critical: Unsupported NEONECT_ENV: %s. Supported values are: development, beta, production", env)
	}

	debug := strings.EqualFold(GetEnvOrDefault("NEONECT_DEBUG", ""), "true")
	if env == "beta" || env == "production" {
		debug = false
	}

	origins := GetEnvOrDefault("NEONECT_ALLOWED_ORIGINS", "")
	var allowed []string
	if origins != "" {
		for _, origin := range strings.Split(origins, ",") {
			allowed = append(allowed, strings.TrimSpace(origin))
		}
	}

	storageRoot := GetStoragePath()
	proxies := GetEnvOrDefault("NEONECT_TRUSTED_PROXIES", "")
	var trustedProxies []string
	if proxies != "" {
		for _, p := range strings.Split(proxies, ",") {
			trustedProxies = append(trustedProxies, strings.TrimSpace(p))
		}
	}

	maxDevices := GetEnvInt("NEONECT_MAX_DEVICES", 10)

	dbDir := GetEnvOrDefault("NEONECT_DB_DIR", filepath.Join(storageRoot, "database"))
	keyDir := GetEnvOrDefault("NEONECT_KEY_DIR", filepath.Join(storageRoot, "keys"))

	dbNameDefault := DbBetaName
	if env == "production" {
		dbNameDefault = "neonect_v1_production"
	}
	if env == "development" {
		dbNameDefault = "neonect_v1_development"
	}

	httpBody := GetStrictEnvInt64("NEONECT_HTTP_MAX_BODY_BYTES", 4194304, 1, 1073741824)
	wsBody := GetStrictEnvInt64("NEONECT_WS_MAX_MESSAGE_BYTES", 4194304, 1, 1073741824)
	envelope := GetStrictEnvInt("NEONECT_MAX_ENVELOPE_BYTES", 1048576, 1, 1073741824)
	if int64(envelope) > httpBody && int64(envelope) > wsBody {
		log.Fatalf("Critical: NEONECT_MAX_ENVELOPE_BYTES (%d) must not exceed both HTTP and WS ingress limits", envelope)
	}

	dbOpen := GetStrictEnvInt("NEONECT_DB_MAX_OPEN_CONNS", 10, 1, 1000)
	dbIdle := GetStrictEnvInt("NEONECT_DB_MAX_IDLE_CONNS", 5, 0, 1000)
	if dbIdle > dbOpen {
		log.Fatalf("Critical: NEONECT_DB_MAX_IDLE_CONNS (%d) cannot be greater than NEONECT_DB_MAX_OPEN_CONNS (%d)", dbIdle, dbOpen)
	}
	dbLifetime := GetStrictEnvInt("NEONECT_DB_CONN_MAX_LIFETIME_SECONDS", 3600, 0, 86400)

	pingPeriod := GetStrictEnvInt("NEONECT_WS_PING_PERIOD_SECONDS", 30, 1, 3600)
	pongWait := GetStrictEnvInt("NEONECT_WS_PONG_WAIT_SECONDS", 45, 2, 3600)
	if pongWait <= pingPeriod {
		log.Fatalf("Critical: NEONECT_WS_PONG_WAIT_SECONDS (%d) must be strictly greater than NEONECT_WS_PING_PERIOD_SECONDS (%d)", pongWait, pingPeriod)
	}

	return AppConfig{
		Environment:       env,
		Debug:             debug,
		AllowedOrigins:    allowed,
		DatabaseDir:       dbDir,
		MasterKeyDir:      keyDir,
		BindAddress:       GetEnvOrDefault("NEONECT_BIND_ADDR", "0.0.0.0:0"),
		MaxDevicesPerUser: maxDevices,
		TrustedProxies:    trustedProxies,

		IntegritySeed:     GetEnvOrDefault("NEONECT_INTEGRITY_SEED", IntegrityCheckSeed),
		DatabaseName:      GetEnvOrDefault("NEONECT_DB_NAME", dbNameDefault),
		ReadTimeout:       time.Duration(GetEnvInt("NEONECT_READ_TIMEOUT", 15)) * time.Second,
		WriteTimeout:      time.Duration(GetEnvInt("NEONECT_WRITE_TIMEOUT", 15)) * time.Second,
		IdleTimeout:       time.Duration(GetEnvInt("NEONECT_IDLE_TIMEOUT", 60)) * time.Second,
		MasterKeyFilename: GetEnvOrDefault("NEONECT_MASTER_KEY_FILE", "master.key"),
		CertFilename:      GetEnvOrDefault("NEONECT_CERT_FILE", "cert.pem"),
		KeyFilename:       GetEnvOrDefault("NEONECT_KEY_FILE", "key.pem"),

		HttpMaxBodyBytes:    httpBody,
		WsMaxMessageBytes:   wsBody,
		MaxEnvelopeBytes:    envelope,
		MaxMailboxBytes:     GetStrictEnvInt("NEONECT_MAX_MAILBOX_BYTES", 52428800, 1, 1073741824),
		MaxMailboxMessages:  GetStrictEnvInt("NEONECT_MAX_MAILBOX_MESSAGES", 1000, 1, 1000000),
		MaxGlobalWebSockets: GetStrictEnvInt("NEONECT_MAX_GLOBAL_WEBSOCKETS", 10000, 1, 1000000),
		DeliveryBatchSize:   GetStrictEnvInt("NEONECT_DELIVERY_BATCH_SIZE", 100, 1, 10000),

		DbMaxOpenConns:    dbOpen,
		DbMaxIdleConns:    dbIdle,
		DbConnMaxLifetime: time.Duration(dbLifetime) * time.Second,

		SqliteBusyTimeoutMs: GetStrictEnvInt("NEONECT_SQLITE_BUSY_TIMEOUT_MS", 10000, 1, 300000),
		SqliteCacheSize:     GetStrictEnvInt("NEONECT_SQLITE_CACHE_SIZE", -32000, -2000000, 2000000),
		SqliteMmapSizeBytes: GetStrictEnvInt64("NEONECT_SQLITE_MMAP_SIZE_BYTES", 268435456, 0, 1099511627776),

		RequestContextTimeout: time.Duration(GetStrictEnvInt("NEONECT_REQUEST_CONTEXT_TIMEOUT_SECONDS", 30, 1, 3600)) * time.Second,
		WsPingPeriod:          time.Duration(pingPeriod) * time.Second,
		WsPongWait:            time.Duration(pongWait) * time.Second,

		MaxConnPerIP:       GetStrictEnvInt("NEONECT_MAX_CONN_PER_IP", RateLimitMaxConnPerIP, 1, 1000),
		MaxReqPerSecIP:     GetStrictEnvInt("NEONECT_MAX_REQ_PER_SEC_IP", RateLimitMaxReqPerSecIP, 1, 10000),
		MaxMsgPerSec:       GetStrictEnvInt("NEONECT_MAX_MSG_PER_SEC", RateLimitMaxMsgPerSec, 1, 10000),
		MaxDiscoveryPerSec: GetStrictEnvInt("NEONECT_MAX_DISCOVERY_PER_SEC", RateLimitMaxDiscoveryPerSec, 1, 100000),
		MaxAuthPerSecIP:    GetStrictEnvInt("NEONECT_MAX_AUTH_PER_SEC_IP", 5, 1, 1000),
	}
}

func GetEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func GetEnvOrDefault(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func GetProjectRoot() string {
	projectRootOnce.Do(func() {
		dir, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get working directory: %v", err)
		}

		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				projectRoot = dir
				return
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				if os.Getenv("NEONECT_ENV") == "production" || os.Getenv("NEONECT_ENV") == "beta" {
					cwd, _ := os.Getwd()
					projectRoot = cwd
					return
				}
				log.Fatalf("Critical: Could not find project root (go.mod). Storage cannot be resolved safely.")
			}
			dir = parent
		}
	})
	return projectRoot
}

func GetStoragePath() string {
	return GetEnvOrDefault("NEONECT_STORAGE_ROOT", filepath.Join(GetProjectRoot(), "storage"))
}

func ValidateStoragePath(target, root string) error {
	resolvedRoot, err := CanonicalizePath(root)
	if err != nil {
		return err
	}

	resolvedTarget, err := CanonicalizePath(target)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil {
		return err
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return os.ErrPermission
	}

	return nil
}

func CanonicalizePath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		if !strings.HasSuffix(cwd, string(filepath.Separator)) {
			cwd += string(filepath.Separator)
		}
		path = cwd + path
	}

	vol := filepath.VolumeName(path)
	pathNoVol := path[len(vol):]

	components := strings.Split(pathNoVol, string(os.PathSeparator))

	current := vol
	if current == "" {
		if strings.HasPrefix(path, "/") {
			current = "/"
		}
	} else if strings.HasPrefix(pathNoVol, "\\") || strings.HasPrefix(pathNoVol, "/") {
		current += string(os.PathSeparator)
	}

	for _, comp := range components {
		if comp == "" || comp == "." {
			continue
		}
		if comp == ".." {
			current = filepath.Dir(current)
			continue
		}

		next := filepath.Join(current, comp)
		eval, err := filepath.EvalSymlinks(next)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				current = next
			} else {
				return "", err
			}
		} else {
			current = eval
		}
	}

	return filepath.Clean(current), nil
}
