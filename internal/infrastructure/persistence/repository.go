package persistence

import "context"

type UserRepository interface {
	IsUsernameTaken(ctx context.Context, hash string) (bool, error)
	GetAuthKey(ctx context.Context, usernameHash string) (string, error)
	CreateUser(ctx context.Context, username, usernameHash, authKey string) (int64, error)
	GetUsernameById(ctx context.Context, id int64) (string, error)
	GetUserIdByUsername(ctx context.Context, username string) (int64, error)
}

type FriendshipRepository interface {
	CreateFriendship(ctx context.Context, userID1, userID2 int64) error
	CheckFriendship(ctx context.Context, userID1, userID2 int64) (bool, error)
	GetFriendsList(ctx context.Context, userID int64) ([]string, error)
	RemoveFriendship(ctx context.Context, userID1, userID2 int64) error
}

type SessionRepository interface {
	CreateSession(ctx context.Context, usernameHash, token string) error
	GetUserIdBySession(ctx context.Context, token string) (int64, error)
	DeleteSession(ctx context.Context, token string) error
}

type DeviceKey struct {
	DeviceID  string
	PublicKey []byte
}

type DeviceRepository interface {
	RegisterDevice(ctx context.Context, userID int64, deviceID string, publicKey []byte, maxDevices int) error
	GetDeviceOwner(ctx context.Context, deviceID string) (int64, error)
	IsDeviceOwnedByUser(ctx context.Context, userID int64, deviceID string) (bool, error)
	IsActiveDeviceOwnedByUser(ctx context.Context, userID int64, deviceID string) (bool, error)
	GetDevicePublicKey(ctx context.Context, deviceID string) ([]byte, error)
	GetDevicesByUser(ctx context.Context, userID int64) ([]string, error)
	GetDevicesAndKeysByUser(ctx context.Context, userID int64) ([]DeviceKey, error)
	DeleteDevice(ctx context.Context, userID int64, deviceID string) error
}

type DeliveryQueueItem struct {
	ID              int64
	DeviceID        string
	MessageID       *string
	SenderDeviceID  *string
	ProtocolVersion *int
	Payload         []byte
	Sequence        int64
	RetryCount      int
	NextRetry       int64
	AckDeadline     int64
}

type DeliveryQueueRepository interface {
	GetLastSequence(ctx context.Context, deviceID string) (int64, error)
	Enqueue(ctx context.Context, deviceID string, payload []byte, ttlSeconds int, sequence int64) error
	EnqueueEnvelope(ctx context.Context, messageID string, senderDeviceID string, recipientDeviceID string, protocolVersion int, payload []byte, ttlSeconds int, sequence int64) error
	FanOutMessage(ctx context.Context, deviceIDs []string, payload []byte, ttlSeconds int) error
	GetQueue(ctx context.Context, deviceID string) ([]DeliveryQueueItem, error)
	UpdateState(ctx context.Context, messageID int64, retryCount int, nextRetry int64, ackDeadline int64) error
	ClaimItem(ctx context.Context, messageID int64, newNextRetry int64, currentNextRetry int64) (bool, error)
	GetDueItems(ctx context.Context, now int64) ([]DeliveryQueueItem, error)
	Acknowledge(ctx context.Context, deviceID string, messageID int64) error
	Cleanup(ctx context.Context) error
}

type IntegrityRepository interface {
	VerifyDatabase(ctx context.Context) bool
}

type SignedPrekeyRecord struct {
	DeviceID  string
	KeyID     uint32
	PublicKey []byte
	Signature []byte
}

type OneTimePrekeyRecord struct {
	DeviceID  string
	KeyID     uint32
	PublicKey []byte
}

type PrekeyBundle struct {
	IdentityKey        []byte
	SignedCurvePrekey  *SignedPrekeyRecord
	SignedPQPrekey     *SignedPrekeyRecord
	OneTimeCurvePrekey *OneTimePrekeyRecord
	OneTimePQPrekey    *OneTimePrekeyRecord
}

type PrekeyRepository interface {
	UploadPrekeys(
		ctx context.Context,
		deviceID string,
		signedCurve *SignedPrekeyRecord,
		oneTimeCurve []OneTimePrekeyRecord,
		signedPQ *SignedPrekeyRecord,
		oneTimePQ []OneTimePrekeyRecord,
	) error
	ClaimPrekeys(ctx context.Context, deviceID string) (*PrekeyBundle, error)
	ClaimPrekeysIdempotent(
		ctx context.Context,
		userID int64,
		deviceID string,
		idempotencyKey string,
		fingerprint string,
		buildResponse func(*PrekeyBundle) ([]byte, int, error),
	) ([]byte, int, error)
}
