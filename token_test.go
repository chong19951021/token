package jwt_token

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/stretchr/testify/assert"
)

const pluginsURL = "https://plugins.traefik.io/public/"
const hashHeader = "X-Plugin-Hash"

func buildArchivePath(pName, pVersion string) string {
	return filepath.Join("/Users/hao/go-projects/src/github.com/token/archivedir", filepath.FromSlash(pName), pVersion+".zip")
}

func computeHash(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}

	hash := sha256.New()

	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	sum := hash.Sum(nil)

	return hex.EncodeToString(sum), nil
}

func Download(ctx context.Context, pName, pVersion string) (string, error) {
	filename := buildArchivePath(pName, pVersion)

	var hash string
	_, err := os.Stat(filename)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to read archive %s: %w", filename, err)
	}

	if err == nil {
		hash, err = computeHash(filename)
		if err != nil {
			return "", fmt.Errorf("failed to compute hash: %w", err)
		}
	}

	baseURL, err := url.Parse(pluginsURL)

	endpoint, err := baseURL.Parse(path.Join(baseURL.Path, "download", pName, pVersion))
	if err != nil {
		return "", fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	if hash != "" {
		req.Header.Set(hashHeader, hash)
	}

	resp, err := retryablehttp.NewClient().StandardClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call service: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNotModified:
		// noop
		return hash, nil

	case http.StatusOK:
		err = os.MkdirAll(filepath.Dir(filename), 0o755)
		if err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}

		var file *os.File
		file, err = os.Create(filename)
		if err != nil {
			return "", fmt.Errorf("failed to create file %q: %w", filename, err)
		}

		defer func() { _ = file.Close() }()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to write response: %w", err)
		}

		hash, err = computeHash(filename)
		if err != nil {
			return "", fmt.Errorf("failed to compute hash: %w", err)
		}

		return hash, nil

	default:
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("error: %d: %s", resp.StatusCode, string(data))
	}
}

func TestDownload(t *testing.T) {
	download, err := Download(context.Background(), "github.com/chong19951021/token", "v1.0.0")
	assert.NoError(t, err)
	fmt.Println(download)

}
