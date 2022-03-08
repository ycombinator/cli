package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/xataio/cli/client/spec"
)

// NewXataClient creates a new Xata client.
func NewXataClient(key, workspaceID string) (*spec.Client, error) {
	url, err := url.Parse(GetXataURL())
	if err != nil {
		return nil, fmt.Errorf("Failed to understand url: %w", err)
	}

	return spec.NewClient(url.String(),
		spec.WithRequestEditorFn(withAPIKey(key)),
		spec.WithRequestEditorFn(withWorkspacesHost(workspaceID)),
	)
}

// NewXataClient creates a new Xata client.
func NewXataClientWithResponses(key, workspaceID string) (*spec.ClientWithResponses, error) {
	client, err := NewXataClient(key, workspaceID)
	if err != nil {
		return nil, err
	}

	return &spec.ClientWithResponses{ClientInterface: client}, nil
}

func GetXataURL() string {
	url := os.Getenv("XATA_URL")
	if url == "" {
		return "https://api.xata.io"
	}
	return url
}

func withAPIKey(key string) spec.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
		return nil
	}
}

func withWorkspacesHost(workspaceID string) spec.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		if req.URL.Path == "/dbs" ||
			strings.HasPrefix(req.URL.Path, "/dbs/") ||
			strings.HasPrefix(req.URL.Path, "/db/") {
			req.Host = workspaceID + ".api.xata.io"
		}

		return nil
	}
}
