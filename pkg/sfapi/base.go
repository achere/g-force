package sfapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type Connection struct {
	ApiVersion   string
	BaseUrl      string
	OrgId        string
	ClientId     string
	ClientSecret string
	accessToken  string
	httpClient   *http.Client
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

func (c *Connection) getAccessToken() (string, error) {
	if c.accessToken != "" {
		return c.accessToken, nil
	}

	return c.refreshToken()
}

func (c *Connection) refreshToken() (string, error) {
	return c.getTokenClientCredentials()
}

func (c *Connection) makeRequest(req *http.Request) ([]byte, error) {
	token, err := c.getAccessToken()
	if err != nil {
		return []byte{}, fmt.Errorf("c.getAccessToken: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("c.httpClient.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		token, err := c.refreshToken()
		if err != nil {
			return []byte{}, fmt.Errorf("c.refreshToken: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		newResp, err := c.httpClient.Do(req)
		if err != nil {
			return []byte{}, fmt.Errorf("c.httpClient.Do: %w", err)
		}
		defer newResp.Body.Close()

		if newResp.StatusCode == 200 {
			resp = newResp
		}
	}
	if resp.StatusCode == 200 {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return []byte{}, fmt.Errorf("io.ReadAll: %w", err)
		}
		return respBody, nil
	}

	return []byte{}, errors.New("Unexpected status code returned: " + strconv.Itoa(resp.StatusCode))
}

func (c *Connection) getTokenClientCredentials() (string, error) {
	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("client_id", c.ClientId)
	formData.Set("client_secret", c.ClientSecret)

	resp, err := http.PostForm(c.BaseUrl+"/services/oauth2/token", formData)
	if err != nil {
		return "", fmt.Errorf("http.PostForm: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.New("Token request returned " + strconv.Itoa(resp.StatusCode))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("io.ReadAll: %w", err)
	}

	var token tokenResponse
	err = json.Unmarshal(respBody, &token)
	if err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}

	return token.AccessToken, nil
}
