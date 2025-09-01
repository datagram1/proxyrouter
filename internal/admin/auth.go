package admin

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// AuthManager handles authentication for the admin interface
type AuthManager struct {
	db           *sql.DB
	sessionStore *SessionStore
	config       *Config
}

// Config holds authentication configuration
type Config struct {
	SessionSecret string
	PasswordHash  string
	MaxAttempts   int
	WindowSeconds int
}

// SessionStore manages user sessions
type SessionStore struct {
	sessions map[string]*Session
}

// Session represents a user session
type Session struct {
	Username   string
	CreatedAt  time.Time
	LastAccess time.Time
	ForceChange bool
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(db *sql.DB, config *Config) *AuthManager {
	return &AuthManager{
		db:           db,
		sessionStore: &SessionStore{sessions: make(map[string]*Session)},
		config:       config,
	}
}

// AuthenticateUser authenticates a user with username and password
func (am *AuthManager) AuthenticateUser(ctx context.Context, username, password string) (bool, bool, error) {
	var passwordHash string
	var forceChange bool

	query := `SELECT password_hash, force_change FROM admin_users WHERE username = ?`
	err := am.db.QueryRowContext(ctx, query, username).Scan(&passwordHash, &forceChange)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, false, nil
		}
		return false, false, fmt.Errorf("failed to query user: %w", err)
	}

	// Verify password
	valid, err := am.verifyPassword(password, passwordHash)
	if err != nil {
		return false, false, fmt.Errorf("failed to verify password: %w", err)
	}

	return valid, forceChange, nil
}

// verifyPassword verifies a password against its hash
func (am *AuthManager) verifyPassword(password, hash string) (bool, error) {
	if strings.HasPrefix(hash, "$argon2id$") {
		return am.verifyArgon2ID(password, hash)
	} else if strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") {
		return am.verifyBcrypt(password, hash)
	}
	return false, fmt.Errorf("unsupported hash format")
}

// verifyArgon2ID verifies an Argon2id hash
func (am *AuthManager) verifyArgon2ID(password, hash string) (bool, error) {
	// Parse the hash
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid argon2id hash format")
	}

	var time uint32
	var memory uint32
	var parallelism uint8

	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &parallelism)
	if err != nil {
		return false, fmt.Errorf("failed to parse argon2id parameters: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("failed to decode key: %w", err)
	}

	// Generate hash with same parameters
	computedKey := argon2.IDKey([]byte(password), salt, time, memory, parallelism, uint32(len(key)))

	// Compare keys
	return subtle.ConstantTimeCompare(computedKey, key) == 1, nil
}

// verifyBcrypt verifies a bcrypt hash
func (am *AuthManager) verifyBcrypt(password, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil, nil
}

// hashPassword hashes a password using the configured algorithm
func (am *AuthManager) hashPassword(password string) (string, error) {
	switch am.config.PasswordHash {
	case "argon2id":
		return am.hashArgon2ID(password)
	case "bcrypt":
		return am.hashBcrypt(password)
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", am.config.PasswordHash)
	}
}

// hashArgon2ID hashes a password using Argon2id
func (am *AuthManager) hashArgon2ID(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Argon2id parameters: time=3, memory=64MB, parallelism=2
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)

	// Format: $argon2id$v=19$m=65536,t=3,p=2$salt$key
	hash := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		64*1024, 3, 2,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key))

	return hash, nil
}

// hashBcrypt hashes a password using bcrypt
func (am *AuthManager) hashBcrypt(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// CreateSession creates a new session for a user
func (am *AuthManager) CreateSession(username string, forceChange bool) (string, error) {
	sessionID := am.generateSessionID()
	
	session := &Session{
		Username:    username,
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
		ForceChange: forceChange,
	}
	
	am.sessionStore.sessions[sessionID] = session
	return sessionID, nil
}

// GetSession retrieves a session by ID
func (am *AuthManager) GetSession(sessionID string) (*Session, bool) {
	session, exists := am.sessionStore.sessions[sessionID]
	if !exists {
		return nil, false
	}
	
	// Update last access time
	session.LastAccess = time.Now()
	return session, true
}

// DeleteSession removes a session
func (am *AuthManager) DeleteSession(sessionID string) {
	delete(am.sessionStore.sessions, sessionID)
}

// generateSessionID generates a random session ID
func (am *AuthManager) generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// ChangePassword changes a user's password
func (am *AuthManager) ChangePassword(ctx context.Context, username, currentPassword, newPassword string) error {
	// Verify current password
	valid, _, err := am.AuthenticateUser(ctx, username, currentPassword)
	if err != nil {
		return fmt.Errorf("failed to verify current password: %w", err)
	}
	if !valid {
		return fmt.Errorf("current password is incorrect")
	}

	// Hash new password
	hash, err := am.hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password in database
	query := `UPDATE admin_users SET password_hash = ?, force_change = 0, updated_at = CURRENT_TIMESTAMP WHERE username = ?`
	_, err = am.db.ExecContext(ctx, query, hash, username)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// CreateUser creates a new admin user
func (am *AuthManager) CreateUser(ctx context.Context, username, password string) error {
	// Hash password
	hash, err := am.hashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert user
	query := `INSERT INTO admin_users (username, password_hash, force_change) VALUES (?, ?, 0)`
	_, err = am.db.ExecContext(ctx, query, username, hash)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// LogAudit logs an audit event
func (am *AuthManager) LogAudit(ctx context.Context, username, action, detail, ip string) error {
	query := `INSERT INTO admin_audit (username, action, detail, ip) VALUES (?, ?, ?, ?)`
	_, err := am.db.ExecContext(ctx, query, username, action, detail, ip)
	if err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}
	return nil
}

// GetUserByUsername retrieves a user by username
func (am *AuthManager) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	query := `SELECT id, username, force_change, created_at, updated_at FROM admin_users WHERE username = ?`
	err := am.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.ForceChange, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// User represents an admin user
type User struct {
	ID          int       `json:"id"`
	Username    string    `json:"username"`
	ForceChange bool      `json:"force_change"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
