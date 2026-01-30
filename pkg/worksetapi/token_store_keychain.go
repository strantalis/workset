package worksetapi

import (
	"context"
	"errors"

	"github.com/zalando/go-keyring"
)

const keyringService = "workset"

// KeyringTokenStore stores tokens in the OS keychain.
type KeyringTokenStore struct{}

func (KeyringTokenStore) Get(_ context.Context, key string) (string, error) {
	value, err := keyring.Get(keyringService, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrTokenNotFound
		}
		return "", err
	}
	return value, nil
}

func (KeyringTokenStore) Set(_ context.Context, key, value string) error {
	if err := keyring.Set(keyringService, key, value); err != nil {
		return err
	}
	return nil
}

func (KeyringTokenStore) Delete(_ context.Context, key string) error {
	if err := keyring.Delete(keyringService, key); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return err
	}
	return nil
}
