package evolutiongo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"disparago/internal/config"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	mu         sync.RWMutex
	tokens     map[string]string
}

type SendTextInput struct {
	InstanceID string `json:"-"`
	Number     string `json:"number"`
	Text       string `json:"text"`
	ID         string `json:"id,omitempty"`
	Delay      int    `json:"delay,omitempty"`
}

type SendTextResponse struct {
	Message   string `json:"message"`
	MessageID string `json:"messageId"`
	Data      struct {
		Info struct {
			ID string `json:"ID"`
		} `json:"Info"`
	} `json:"data"`
}

type listInstancesResponse struct {
	Data []Instance `json:"data"`
}

type Instance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	Connected bool   `json:"connected"`
}

func New(cfg config.Config) *Client {
	return &Client{
		baseURL: cfg.EvolutionGO.BaseURL,
		apiKey:  cfg.EvolutionGO.APIKey,
		httpClient: &http.Client{
			Timeout: cfg.EvolutionGO.Timeout,
		},
		tokens: make(map[string]string),
	}
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) SendText(ctx context.Context, input SendTextInput) (SendTextResponse, error) {
	instanceToken, err := c.resolveInstanceToken(ctx, input.InstanceID)
	if err != nil {
		return SendTextResponse{}, err
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return SendTextResponse{}, fmt.Errorf("marshal send text payload: %w", err)
	}

	url := strings.TrimRight(c.baseURL, "/") + "/send/text"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return SendTextResponse{}, fmt.Errorf("build send text request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", instanceToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return SendTextResponse{}, fmt.Errorf("send text request: %w", err)
	}
	defer resp.Body.Close()

	var decoded SendTextResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return SendTextResponse{}, fmt.Errorf("decode send text response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SendTextResponse{}, fmt.Errorf("send text failed with status %d: %s", resp.StatusCode, decoded.Message)
	}

	if decoded.MessageID == "" {
		decoded.MessageID = decoded.Data.Info.ID
	}

	return decoded, nil
}

func (c *Client) resolveInstanceToken(ctx context.Context, instanceID string) (string, error) {
	c.mu.RLock()
	token, ok := c.tokens[instanceID]
	c.mu.RUnlock()
	if ok && token != "" {
		return token, nil
	}

	instances, err := c.ListInstances(ctx)
	if err != nil {
		return "", fmt.Errorf("list instances: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, instance := range instances {
		if instance.Name != "" && instance.Token != "" {
			c.tokens[instance.Name] = instance.Token
		}
	}

	if token = c.tokens[instanceID]; token != "" {
		return token, nil
	}

	return instanceID, nil
}

func (c *Client) ListInstances(ctx context.Context) ([]Instance, error) {
	url := strings.TrimRight(c.baseURL, "/") + "/instance/all"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build list instances request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list instances request: %w", err)
	}
	defer resp.Body.Close()

	var decoded listInstancesResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode list instances response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list instances failed with status %d", resp.StatusCode)
	}

	return decoded.Data, nil
}
