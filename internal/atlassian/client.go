package atlassian

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"atlassian-jira-conf-mcp/internal/auth"
	"atlassian-jira-conf-mcp/internal/vault"
)

type Client struct {
	authSvc    *auth.Service
	db         *vault.DB
	httpClient *http.Client
	downloadDir string
}

func NewClient(authSvc *auth.Service, db *vault.DB, httpClient *http.Client, downloadDir string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if downloadDir == "" {
		downloadDir = "data/downloads"
	}
	return &Client{
		authSvc:     authSvc,
		db:          db,
		httpClient:  httpClient,
		downloadDir: downloadDir,
	}
}

// ResolveSite determines the effective approved site for a user, using overrideCloudID if provided.
func (c *Client) ResolveSite(userID, overrideCloudID string) (*vault.ApprovedSite, error) {
	if overrideCloudID != "" {
		site, err := c.db.GetSiteByCloudID(userID, overrideCloudID)
		if err == nil {
			return site, nil
		}
		// If not in database, construct minimal approved site representation with overrideCloudID
		return &vault.ApprovedSite{
			UserID:  userID,
			CloudID: overrideCloudID,
		}, nil
	}
	return c.db.GetDefaultSite(userID)
}

// ExecuteRequest performs an authenticated HTTP request with automatic retry on 429 / 503
// and parses Atlassian error responses.
func (c *Client) ExecuteRequest(ctx context.Context, userID, method, rawURL string, body []byte, headers map[string]string) ([]byte, int, error) {
	token, err := c.authSvc.GetValidAccessToken(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("retrieve oauth access token: %w", err)
	}

	maxRetries := 3
	var respBody []byte
	var statusCode int

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
		if err != nil {
			return nil, 0, fmt.Errorf("create http request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		if body != nil && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt < maxRetries {
				sleepBackoff(attempt, 0)
				continue
			}
			return nil, 0, fmt.Errorf("http request failed: %w", err)
		}

		statusCode = resp.StatusCode
		respBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, statusCode, fmt.Errorf("read response body: %w", err)
		}

		// Handle rate limiting (429) or service unavailable (503)
		if statusCode == http.StatusTooManyRequests || statusCode == http.StatusServiceUnavailable {
			if attempt < maxRetries {
				retryAfterSec := 0
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					if sec, err := strconv.Atoi(ra); err == nil && sec > 0 {
						retryAfterSec = sec
					}
				}
				sleepBackoff(attempt, retryAfterSec)
				continue
			}
		}

		// Check for success or error
		if statusCode >= 200 && statusCode < 300 {
			return respBody, statusCode, nil
		}

		// Translate error
		return respBody, statusCode, c.translateError(statusCode, respBody)
	}

	return respBody, statusCode, fmt.Errorf("request exceeded maximum retries")
}

func sleepBackoff(attempt, retryAfterSec int) {
	if retryAfterSec > 0 {
		time.Sleep(time.Duration(retryAfterSec) * time.Second)
		return
	}
	base := time.Duration(1<<attempt) * 500 * time.Millisecond
	jitter := time.Duration(rand.Intn(250)) * time.Millisecond
	time.Sleep(base + jitter)
}

func (c *Client) translateError(statusCode int, body []byte) error {
	var errData map[string]interface{}
	_ = json.Unmarshal(body, &errData)

	var errMsgs []string
	if em, ok := errData["errorMessages"].([]interface{}); ok {
		for _, m := range em {
			if s, ok := m.(string); ok {
				errMsgs = append(errMsgs, s)
			}
		}
	}
	if errMap, ok := errData["errors"].(map[string]interface{}); ok {
		for k, v := range errMap {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", k, v))
		}
	}
	if message, ok := errData["message"].(string); ok {
		errMsgs = append(errMsgs, message)
	}

	combinedMsg := strings.Join(errMsgs, "; ")
	if combinedMsg == "" {
		combinedMsg = string(body)
		if len(combinedMsg) > 250 {
			combinedMsg = combinedMsg[:250] + "..."
		}
	}

	switch statusCode {
	case http.StatusForbidden:
		return fmt.Errorf("Permission Denied (403): Your Atlassian Cloud account lacks the necessary permissions for this operation. Atlassian message: %s", combinedMsg)
	case http.StatusNotFound:
		return fmt.Errorf("Resource Not Found (404): The requested issue, project, space, or page does not exist or you do not have permission to view it. Details: %s", combinedMsg)
	case http.StatusBadRequest:
		return fmt.Errorf("Invalid Request (400): %s", combinedMsg)
	case http.StatusUnauthorized:
		return errors.New("Unauthorized (401): Atlassian session expired or invalid credentials")
	default:
		return fmt.Errorf("Atlassian API Error (Status %d): %s", statusCode, combinedMsg)
	}
}
