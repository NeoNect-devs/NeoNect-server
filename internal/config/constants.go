package config

import "time"

const (
	DriverSqlite       = "sqlite"
	MaxOpenConns       = 10
	PingPeriod         = 30 * time.Second
	PongWait           = 45 * time.Second
	MaxIdleConns       = 5
	ConnMaxLifetime    = time.Hour
	IntegrityCheckSeed = "integrity_pulse"
)

const (
	BlockTypeAuth    = "AUTH"
	BlockTypeSession = "USER_INFO_SESSION"
	BlockTypeDevice  = "DEVICE"
)

const (
	DefaultMessageTTL          = 24 * 3600
	DefaultSchedulerInterval   = 1 * time.Second
	DefaultCleanupInterval     = 1 * time.Hour
	DeliveryLockDuration       = 120
	AcknowledgmentTimeout      = 60
	InitialBackoffDuration     = 30
	MaximumBackoffDuration     = 3600
	MaximumRetryAttempts       = 20
	TerminalRetryBackoff       = 3600
	DatabaseInitializationWait = 30 * time.Second
	MaintenanceTimeout         = 30 * time.Second
)

const (
	SessionCookieName = "neonect_sid"
	SessionDuration   = 24 * time.Hour
	CookiePath        = "/"
	TokenEntropy      = 32
)

const (
	StatusSuccess   = "success"
	StatusOk        = "ok"
	ContentTypeJSON = "application/json"
)

const (
	MasterKeySize = 64
	DbBetaName    = "neonect_v1_beta"
	DirPerm       = 0700
	FilePerm      = 0600
	SqliteExt     = ".sqlite"
)

const (
	RateLimitMaxConnPerIP       = 10
	RateLimitMaxMsgPerSec       = 100
	RateLimitMaxReqPerSecIP     = 50
	RateLimitMaxDiscoveryPerSec = 10
)

const (
	ReadTimeout           = 15 * time.Second
	WriteTimeout          = 15 * time.Second
	IdleTimeout           = 60 * time.Second
	ShutdownTimeout       = 10 * time.Second
	GlobalMaxBodySize     = 4 << 20
	RequestContextTimeout = 30 * time.Second
)

const (
	SqliteBusyTimeout = 10000
	SqliteCacheSize   = -32000
	SqliteMmapSize    = 268435456
)

const (
	MinUsernameLength = 3
	MinPasswordLength = 8
)
