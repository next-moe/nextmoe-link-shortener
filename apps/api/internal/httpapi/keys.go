package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/engine"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/model"
)

// registerKeys wires the S2S API key management endpoints (admin-only).
func (h *handlers) registerKeys(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-keys",
		Method:      http.MethodGet,
		Path:        "/keys",
		Summary:     "List S2S API keys",
		Tags:        []string{"keys"},
	}, h.listKeys)

	huma.Register(api, huma.Operation{
		OperationID: "create-key",
		Method:      http.MethodPost,
		Path:        "/keys",
		Summary:     "Mint an S2S API key (plaintext shown exactly once)",
		Tags:        []string{"keys"},
	}, h.createKey)

	huma.Register(api, huma.Operation{
		OperationID: "set-key-disabled",
		Method:      http.MethodPut,
		Path:        "/keys/{id}/disabled",
		Summary:     "Disable or re-enable an S2S API key",
		Tags:        []string{"keys"},
	}, h.setKeyDisabled)

	huma.Register(api, huma.Operation{
		OperationID:   "delete-key",
		Method:        http.MethodDelete,
		Path:          "/keys/{id}",
		Summary:       "Delete an S2S API key permanently",
		Tags:          []string{"keys"},
		DefaultStatus: http.StatusNoContent,
	}, h.deleteKey)
}

// KeyDTO is the wire shape of an API key. The hash is never exposed; the
// prefix is enough to correlate with the plaintext the caller saved.
type KeyDTO struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix" doc:"First characters of the plaintext, for display"`
	Disabled   bool       `json:"disabled"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedBy  int64      `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
}

func toKeyDTO(k *model.APIKey) KeyDTO {
	return KeyDTO{
		ID:         k.ID,
		Name:       k.Name,
		KeyPrefix:  k.KeyPrefix,
		Disabled:   k.Disabled,
		LastUsedAt: k.LastUsedAt,
		CreatedBy:  k.CreatedBy,
		CreatedAt:  k.CreatedAt,
	}
}

// ---- list ----

type keyListOutput struct {
	Body struct {
		Keys []KeyDTO `json:"keys"`
	}
}

func (h *handlers) listKeys(ctx context.Context, _ *struct{}) (*keyListOutput, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	keys, err := h.engine.ListKeys()
	if err != nil {
		slog.Error("list keys", "error", err)
		return nil, huma.Error500InternalServerError("could not list keys")
	}
	out := &keyListOutput{}
	out.Body.Keys = make([]KeyDTO, len(keys))
	for i := range keys {
		out.Body.Keys[i] = toKeyDTO(&keys[i])
	}
	return out, nil
}

// ---- create ----

type createKeyInput struct {
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"100" doc:"Which product this key belongs to, e.g. \"kungal\""`
	}
}

type createKeyOutput struct {
	Body struct {
		Key       KeyDTO `json:"key"`
		Plaintext string `json:"plaintext" doc:"The full API key. Shown exactly once — store it now."`
	}
}

func (h *handlers) createKey(ctx context.Context, in *createKeyInput) (*createKeyOutput, error) {
	id, err := h.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	plaintext, key, err := h.engine.MintKey(in.Body.Name, id.UserID)
	if err != nil {
		if errors.Is(err, engine.ErrKeyNameTaken) {
			return nil, huma.Error409Conflict("an API key with this name already exists")
		}
		slog.Error("mint key", "error", err)
		return nil, huma.Error500InternalServerError("could not create key")
	}
	out := &createKeyOutput{}
	out.Body.Key = toKeyDTO(key)
	out.Body.Plaintext = plaintext
	return out, nil
}

// ---- disable / enable ----

type setKeyDisabledInput struct {
	ID   int64 `path:"id"`
	Body struct {
		Disabled bool `json:"disabled"`
	}
}

type okOutput struct {
	Body struct {
		OK bool `json:"ok"`
	}
}

func (h *handlers) setKeyDisabled(ctx context.Context, in *setKeyDisabledInput) (*okOutput, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.engine.SetKeyDisabled(in.ID, in.Body.Disabled); err != nil {
		if errors.Is(err, engine.ErrNotFound) {
			return nil, huma.Error404NotFound("key not found")
		}
		slog.Error("set key disabled", "error", err)
		return nil, huma.Error500InternalServerError("could not update key")
	}
	out := &okOutput{}
	out.Body.OK = true
	return out, nil
}

// ---- delete ----

type deleteKeyInput struct {
	ID int64 `path:"id"`
}

func (h *handlers) deleteKey(ctx context.Context, in *deleteKeyInput) (*struct{}, error) {
	if _, err := h.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.engine.DeleteKey(in.ID); err != nil {
		if errors.Is(err, engine.ErrNotFound) {
			return nil, huma.Error404NotFound("key not found")
		}
		slog.Error("delete key", "error", err)
		return nil, huma.Error500InternalServerError("could not delete key")
	}
	return &struct{}{}, nil
}
