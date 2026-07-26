package engine

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/kungal/kungal-link-shortener/apps/api/internal/model"
)

// keyByteLen sizes the random part of an API key (32 bytes = 64 hex chars).
const keyByteLen = 32

// keyPrefixDisplay is how many leading plaintext characters are stored for
// display ("slk_ab12cd34…").
const keyPrefixDisplay = 12

// MintKey creates an S2S API key for a sibling product. The plaintext
// ("slk_" + 64 hex chars) is returned exactly once; only its SHA-256 hash is
// stored.
func (e *Engine) MintKey(name string, createdBy int64) (string, *model.APIKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil, errors.New("engine: api key name is required")
	}
	raw := make([]byte, keyByteLen)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	plaintext := "slk_" + hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plaintext))

	key := &model.APIKey{
		Name:      name,
		KeyHash:   hex.EncodeToString(sum[:]),
		KeyPrefix: plaintext[:keyPrefixDisplay],
		CreatedBy: createdBy,
	}
	if err := e.db.Create(key).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return "", nil, ErrKeyNameTaken
		}
		return "", nil, err
	}
	return plaintext, key, nil
}

// ListKeys returns every API key (hashes included — they are one-way and the
// dashboard is admin-only; the DTO layer still omits them).
func (e *Engine) ListKeys() ([]model.APIKey, error) {
	var keys []model.APIKey
	err := e.db.Order("id").Find(&keys).Error
	return keys, err
}

// SetKeyDisabled toggles a key without destroying it (revoke-with-undo).
func (e *Engine) SetKeyDisabled(id int64, disabled bool) error {
	res := e.db.Model(&model.APIKey{}).Where("id = ?", id).Update("disabled", disabled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteKey removes a key permanently.
func (e *Engine) DeleteKey(id int64) error {
	res := e.db.Delete(&model.APIKey{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// VerifyKey authenticates an S2S bearer token. Fail-closed: unknown hash or a
// disabled key both return ErrNotFound (the caller answers 401 without
// distinguishing — existence must not leak). last_used_at updates
// best-effort; a failed touch never fails the request.
func (e *Engine) VerifyKey(plaintext string) (*model.APIKey, error) {
	if !strings.HasPrefix(plaintext, "slk_") {
		return nil, ErrNotFound
	}
	sum := sha256.Sum256([]byte(plaintext))
	want := hex.EncodeToString(sum[:])

	var key model.APIKey
	err := e.db.Where("key_hash = ?", want).First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// Constant-time recheck is redundant after an indexed hash lookup but
	// costs nothing and keeps the comparison uniformly non-short-circuiting.
	if subtle.ConstantTimeCompare([]byte(key.KeyHash), []byte(want)) != 1 || key.Disabled {
		return nil, ErrNotFound
	}
	now := time.Now()
	_ = e.db.Model(&model.APIKey{}).Where("id = ?", key.ID).Update("last_used_at", now).Error
	key.LastUsedAt = &now
	return &key, nil
}
