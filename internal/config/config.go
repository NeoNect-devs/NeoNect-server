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

		// In non-development environments, if no go.mod is found, fallback to the executable directory or /opt/neonect.
		// However, it's safer to just return the working directory and let explicit config paths take precedence.
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				projectRoot = dir
				return
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				if os.Getenv("NEONECT_ENV") == "production" || os.Getenv("NEONECT_ENV") == "beta" {
					// Don't crash, just use current working directory or /opt/neonect
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

// ValidateStoragePath checks if target is safely contained within root.
// It explicitly resolves symlinks for all existing components and prevents
// directory traversal, sibling prefix bypass, and escaping via symlinks.
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

// CanonicalizePath securely evaluates a path, resolving all existing symlinks.
// It stops resolving when it encounters the first non-existent component,
// and returns the accumulated canonical path joined with the remaining non-existent components.
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
