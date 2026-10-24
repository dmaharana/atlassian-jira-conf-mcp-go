package vault

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrSiteNotFound = errors.New("approved site not found")
	ErrNoTokens     = errors.New("no oauth tokens found for user")
)

type User struct {
	ID        string    `json:"id"`
	MCPAPIKey string    `json:"mcp_api_key"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type TokenData struct {
	UserID       string    `json:"user_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	TokenType    string    `json:"token_type"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ApprovedSite struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CloudID   string    `json:"cloud_id"`
	SiteName  string    `json:"site_name"`
	SiteURL   string    `json:"site_url"`
	IsDefault bool      `json:"is_default"`
	Scopes    string    `json:"scopes"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DB struct {
	db        *sql.DB
	encryptor *Encryptor
}

func OpenDB(dbPath string, encryptor *Encryptor) (*DB, error) {
	if dbPath == "" {
		dbPath = filepath.Join("data", "mcp_vault.db")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	// Controlled concurrency to prevent SQLite contention while supporting concurrent readers
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	vaultDB := &DB{
		db:        db,
		encryptor: encryptor,
	}

	if err := vaultDB.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return vaultDB, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		mcp_api_key TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(mcp_api_key);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

	CREATE TABLE IF NOT EXISTS tokens (
		user_id TEXT PRIMARY KEY,
		access_token_enc BLOB NOT NULL,
		refresh_token_enc BLOB NOT NULL,
		expiry TIMESTAMP NOT NULL,
		token_type TEXT NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS approved_sites (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		cloud_id TEXT NOT NULL,
		site_name TEXT NOT NULL,
		site_url TEXT NOT NULL,
		is_default INTEGER NOT NULL DEFAULT 0,
		scopes TEXT NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		UNIQUE(user_id, cloud_id),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_approved_sites_user ON approved_sites(user_id);
	`
	_, err := d.db.Exec(schema)
	return err
}

// UpsertUser finds an existing user by email or creates a new user, returning the User object.
func (d *DB) UpsertUser(email, name string) (*User, error) {
	existing, err := d.GetUserByEmail(email)
	if err == nil {
		if name != "" && existing.Name != name {
			_, _ = d.db.Exec("UPDATE users SET name = ? WHERE id = ?", name, existing.ID)
			existing.Name = name
		}
		return existing, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	apiKey := fmt.Sprintf("mcp_%s%s", uuid.New().String()[:8], uuid.New().String()[:16])
	user := &User{
		ID:        uuid.New().String(),
		MCPAPIKey: apiKey,
		Email:     email,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}

	query := `INSERT INTO users (id, mcp_api_key, email, name, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err = d.db.Exec(query, user.ID, user.MCPAPIKey, user.Email, user.Name, user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return user, nil
}

func (d *DB) GetUserByAPIKey(apiKey string) (*User, error) {
	row := d.db.QueryRow("SELECT id, mcp_api_key, email, name, created_at FROM users WHERE mcp_api_key = ?", apiKey)
	var u User
	err := row.Scan(&u.ID, &u.MCPAPIKey, &u.Email, &u.Name, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

func (d *DB) GetUserByEmail(email string) (*User, error) {
	row := d.db.QueryRow("SELECT id, mcp_api_key, email, name, created_at FROM users WHERE email = ?", email)
	var u User
	err := row.Scan(&u.ID, &u.MCPAPIKey, &u.Email, &u.Name, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

func (d *DB) GetUserByID(id string) (*User, error) {
	row := d.db.QueryRow("SELECT id, mcp_api_key, email, name, created_at FROM users WHERE id = ?", id)
	var u User
	err := row.Scan(&u.ID, &u.MCPAPIKey, &u.Email, &u.Name, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

// SaveTokens encrypts and stores access and refresh tokens for a user.
func (d *DB) SaveTokens(td *TokenData) error {
	accessEnc, err := d.encryptor.Encrypt([]byte(td.AccessToken))
	if err != nil {
		return fmt.Errorf("encrypt access token: %w", err)
	}

	refreshEnc, err := d.encryptor.Encrypt([]byte(td.RefreshToken))
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}

	now := time.Now().UTC()
	query := `
	INSERT INTO tokens (user_id, access_token_enc, refresh_token_enc, expiry, token_type, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id) DO UPDATE SET
		access_token_enc = excluded.access_token_enc,
		refresh_token_enc = excluded.refresh_token_enc,
		expiry = excluded.expiry,
		token_type = excluded.token_type,
		updated_at = excluded.updated_at
	`
	_, err = d.db.Exec(query, td.UserID, accessEnc, refreshEnc, td.Expiry.UTC(), td.TokenType, now)
	if err != nil {
		return fmt.Errorf("upsert tokens: %w", err)
	}
	td.UpdatedAt = now
	return nil
}

// GetTokens retrieves and decrypts stored tokens for a user.
func (d *DB) GetTokens(userID string) (*TokenData, error) {
	row := d.db.QueryRow("SELECT access_token_enc, refresh_token_enc, expiry, token_type, updated_at FROM tokens WHERE user_id = ?", userID)
	var accessEnc, refreshEnc []byte
	var td TokenData
	td.UserID = userID

	err := row.Scan(&accessEnc, &refreshEnc, &td.Expiry, &td.TokenType, &td.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoTokens
	}
	if err != nil {
		return nil, fmt.Errorf("scan tokens: %w", err)
	}

	accessPlain, err := d.encryptor.Decrypt(accessEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt access token: %w", err)
	}
	refreshPlain, err := d.encryptor.Decrypt(refreshEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh token: %w", err)
	}

	td.AccessToken = string(accessPlain)
	td.RefreshToken = string(refreshPlain)
	return &td, nil
}

// SaveApprovedSite stores or updates an approved Atlassian Cloud site for a user.
func (d *DB) SaveApprovedSite(site *ApprovedSite) error {
	if site.ID == "" {
		site.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	site.UpdatedAt = now

	// If this site is default, clear previous default flag for this user
	if site.IsDefault {
		_, _ = d.db.Exec("UPDATE approved_sites SET is_default = 0 WHERE user_id = ?", site.UserID)
	}

	query := `
	INSERT INTO approved_sites (id, user_id, cloud_id, site_name, site_url, is_default, scopes, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(user_id, cloud_id) DO UPDATE SET
		site_name = excluded.site_name,
		site_url = excluded.site_url,
		is_default = CASE WHEN excluded.is_default = 1 THEN 1 ELSE approved_sites.is_default END,
		scopes = excluded.scopes,
		updated_at = excluded.updated_at
	`
	isDefInt := 0
	if site.IsDefault {
		isDefInt = 1
	}

	_, err := d.db.Exec(query, site.ID, site.UserID, site.CloudID, site.SiteName, site.SiteURL, isDefInt, site.Scopes, now)
	if err != nil {
		return fmt.Errorf("upsert approved site: %w", err)
	}
	return nil
}

// GetDefaultSite retrieves the user's default approved site, or the first site if none is explicitly default.
func (d *DB) GetDefaultSite(userID string) (*ApprovedSite, error) {
	query := `
	SELECT id, user_id, cloud_id, site_name, site_url, is_default, scopes, updated_at
	FROM approved_sites
	WHERE user_id = ?
	ORDER BY is_default DESC, updated_at DESC
	LIMIT 1
	`
	row := d.db.QueryRow(query, userID)
	var s ApprovedSite
	var isDefInt int
	err := row.Scan(&s.ID, &s.UserID, &s.CloudID, &s.SiteName, &s.SiteURL, &isDefInt, &s.Scopes, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSiteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan site: %w", err)
	}
	s.IsDefault = (isDefInt == 1)
	return &s, nil
}

// GetSiteByCloudID retrieves an approved site by its Atlassian Cloud ID for a user.
func (d *DB) GetSiteByCloudID(userID, cloudID string) (*ApprovedSite, error) {
	query := `
	SELECT id, user_id, cloud_id, site_name, site_url, is_default, scopes, updated_at
	FROM approved_sites
	WHERE user_id = ? AND cloud_id = ?
	LIMIT 1
	`
	row := d.db.QueryRow(query, userID, cloudID)
	var s ApprovedSite
	var isDefInt int
	err := row.Scan(&s.ID, &s.UserID, &s.CloudID, &s.SiteName, &s.SiteURL, &isDefInt, &s.Scopes, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSiteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan site: %w", err)
	}
	s.IsDefault = (isDefInt == 1)
	return &s, nil
}

// ListSites returns all approved sites for a user.
func (d *DB) ListSites(userID string) ([]ApprovedSite, error) {
	query := `
	SELECT id, user_id, cloud_id, site_name, site_url, is_default, scopes, updated_at
	FROM approved_sites
	WHERE user_id = ?
	ORDER BY is_default DESC, site_name ASC
	`
	rows, err := d.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("query sites: %w", err)
	}
	defer rows.Close()

	var sites []ApprovedSite
	for rows.Next() {
		var s ApprovedSite
		var isDefInt int
		if err := rows.Scan(&s.ID, &s.UserID, &s.CloudID, &s.SiteName, &s.SiteURL, &isDefInt, &s.Scopes, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan site row: %w", err)
		}
		s.IsDefault = (isDefInt == 1)
		sites = append(sites, s)
	}
	return sites, rows.Err()
}
