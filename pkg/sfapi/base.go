package sfapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Connection struct {
	ApiVersion   string
	BaseUrl      string
	OrgId        string
	ClientId     string
	ClientSecret string
	accessToken  string
	HttpClient   *http.Client
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

func (c *Connection) getAccessToken(ctx context.Context) (string, error) {
	if c.accessToken != "" {
		return c.accessToken, nil
	}

	return c.refreshToken(ctx)
}

func (c *Connection) refreshToken(ctx context.Context) (string, error) {
	return c.getTokenClientCredentials(ctx)
}

func (c *Connection) DoRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return []byte{}, fmt.Errorf("context canceled: %w", err)
	}

	token, err := c.getAccessToken(ctx)
	if err != nil {
		return []byte{}, fmt.Errorf("c.getAccessToken: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	req = req.WithContext(ctx)
	if c.HttpClient == nil {
		c.HttpClient = &http.Client{}
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("c.httpClient.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		token, errToken := c.refreshToken(ctx)
		if errToken != nil {
			return []byte{}, fmt.Errorf("c.refreshToken: %w", errToken)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		newReq := req.Clone(ctx)
		newResp, errToken := c.HttpClient.Do(newReq)
		if errToken != nil {
			return []byte{}, fmt.Errorf("c.httpClient.Do: %w", errToken)
		}
		defer newResp.Body.Close()

		if newResp.StatusCode == 200 {
			resp = newResp
		}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("io.ReadAll: %w", err)
	}

	if resp.StatusCode == 200 {
		return respBody, nil
	}

	return respBody, fmt.Errorf(
		"unexpected status code returned: %s, body: %s",
		strconv.Itoa(resp.StatusCode),
		string(respBody),
	)
}

func (c *Connection) getTokenClientCredentials(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context canceled: %w", err)
	}

	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("client_id", c.ClientId)
	formData.Set("client_secret", c.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseUrl+"/services/oauth2/token", nil)
	if err != nil {
		return "", fmt.Errorf("http.NewRequestWithContext: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(strings.NewReader(formData.Encode()))

	if c.HttpClient == nil {
		c.HttpClient = &http.Client{}
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("httpClient.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.New("token request returned " + strconv.Itoa(resp.StatusCode))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("io.ReadAll: %w", err)
	}

	var token TokenResponse
	err = json.Unmarshal(respBody, &token)
	if err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}

	return token.AccessToken, nil
}
