package utils

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/afero"
	"github.com/supabase/cli/internal/utils/credentials"
	"github.com/zalando/go-keyring"
)

var (
	AccessTokenPattern = regexp.MustCompile(`^sbp_[a-f0-9]{40}$`)
	ErrInvalidToken    = errors.New("Invalid access token format. Must be like `sbp_0102...1920`.")
	ErrMissingToken    = errors.New("Access token not provided. Supply an access token by running " + Aqua("supabase login") + " or setting the SUPABASE_ACCESS_TOKEN environment variable.")
	ErrNotLoggedIn     = errors.New("You were not logged in, nothing to do.")
)

const AccessTokenKey = "access-token"

func LoadAccessToken() (string, error) {
	return LoadAccessTokenFS(afero.NewOsFs())
}

func LoadAccessTokenFS(fsys afero.Fs) (string, error) {
	accessToken, err := loadAccessToken(fsys)
	if err != nil {
		return "", err
	}
	if !AccessTokenPattern.MatchString(accessToken) {
		return "", ErrInvalidToken
	}
	return accessToken, err
}

func loadAccessToken(fsys afero.Fs) (string, error) {
	// Env takes precedence
	if accessToken := os.Getenv("SUPABASE_ACCESS_TOKEN"); accessToken != "" {
		return accessToken, nil
	}
	// Load from native credentials store
	if accessToken, err := credentials.Get(AccessTokenKey); err == nil {
		return accessToken, nil
	}
	// Fallback to token file
	return fallbackLoadToken(fsys)
}

func fallbackLoadToken(fsys afero.Fs) (string, error) {
	path, err := getAccessTokenPath()
	if err != nil {
		return "", err
	}
	accessToken, err := afero.ReadFile(fsys, path)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrMissingToken
	} else if err != nil {
		return "", err
	}
	return string(accessToken), nil
}

func SaveAccessToken(accessToken string, fsys afero.Fs) error {
	// Validate access token
	if !AccessTokenPattern.MatchString(accessToken) {
		return ErrInvalidToken
	}
	// Save to native credentials store
	if err := credentials.Set(AccessTokenKey, accessToken); err == nil {
		return nil
	}
	// Fallback to token file
	return fallbackSaveToken(accessToken, fsys)
}

func fallbackSaveToken(accessToken string, fsys afero.Fs) error {
	path, err := getAccessTokenPath()
	if err != nil {
		return err
	}
	if err := MkdirIfNotExistFS(fsys, filepath.Dir(path)); err != nil {
		return err
	}
	return afero.WriteFile(fsys, path, []byte(accessToken), 0600)
}

func DeleteAccessToken(fsys afero.Fs) error {
	// Always delete the fallback token file to handle legacy CLI
	if err := fallbackDeleteToken(fsys); err == nil {
		// Typically user system should only have either token file or keyring.
		// But we delete from both just in case.
		_ = credentials.Delete(AccessTokenKey)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Fallback not found, delete from native credentials store
	err := credentials.Delete(AccessTokenKey)
	if errors.Is(err, credentials.ErrNotSupported) || errors.Is(err, keyring.ErrNotFound) {
		return ErrNotLoggedIn
	}
	return err
}

func fallbackDeleteToken(fsys afero.Fs) error {
	path, err := getAccessTokenPath()
	if err != nil {
		return err
	}
	return fsys.Remove(path)
}

func getAccessTokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// TODO: fallback to workdir
	return filepath.Join(home, ".supabase", AccessTokenKey), nil
}
