// Package engine is the shortlink domain logic: link CRUD, alias generation,
// redirect resolution with visit accounting, stats aggregation, and S2S API
// key management. Handlers stay thin; everything stateful lives here.
package engine

import (
	"crypto/rand"
	"errors"
	"math/big"
	"regexp"

	"gorm.io/gorm"
)

// Sentinel errors the HTTP layer maps to status codes.
var (
	ErrNotFound       = errors.New("engine: not found")
	ErrAliasTaken     = errors.New("engine: alias already taken")
	ErrAliasInvalid   = errors.New("engine: alias must be 4-32 chars of [A-Za-z0-9_-]")
	ErrAliasExhausted = errors.New("engine: could not generate a free alias")
	ErrLinkGone       = errors.New("engine: link disabled, expired, or over its visit limit")
	ErrKeyNameTaken   = errors.New("engine: api key name already taken")
)

// aliasAlphabet omits easily-confused characters (l/I/O/0/1) — kept from the
// pre-migration implementation so old and new random aliases look alike.
const aliasAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// customAliasRe validates dashboard/S2S-supplied aliases.
var customAliasRe = regexp.MustCompile(`^[A-Za-z0-9_-]{4,32}$`)

// Engine wraps the database handle.
type Engine struct {
	db *gorm.DB
}

// New builds an Engine over an opened GORM handle.
func New(db *gorm.DB) *Engine {
	return &Engine{db: db}
}

// randomAlias returns a crypto-random alias of the given length.
func randomAlias(length int) string {
	max := big.NewInt(int64(len(aliasAlphabet)))
	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err) // crypto/rand failure is unrecoverable
		}
		out[i] = aliasAlphabet[n.Int64()]
	}
	return string(out)
}

// truncate clips s to at most n bytes (the visit columns are bounded and an
// oversized header must never fail the redirect write).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
