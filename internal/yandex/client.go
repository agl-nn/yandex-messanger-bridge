// Путь: internal/yandex/client.go
package yandex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

type SendMessageRequest struct {
	ChatID string `json:"chat_id,omitempty"`
	Login  string `json:"login,omitempty"`
	Text   string `json:"text"`
}

type SendMessageResponse struct {
	MessageID int64 `json:"message_id"`
	Ok        bool  `json:"ok"`
}

func NewClient(token string) *Client {
	return &Client{
		token:   token,
		baseURL: "https://botapi.messenger.yandex.net/bot/v1",
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) SendToChat(ctx context.Context, chatID, text string, keyboard interface{}) error {
	req := SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}

	return c.sendMessage(ctx, "/messages/sendText/", req)
}

func (c *Client) SendToLogin(ctx context.Context, login, text string, keyboard interface{}) error {
	req := SendMessageRequest{
		Login: login,
		Text:  text,
	}

	return c.sendMessage(ctx, "/messages/sendText/", req)
}

func (c *Client) sendMessage(ctx context.Context, path string, req interface{}) error {
	url := c.baseURL + path

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "OAuth "+c.token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	var result SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Ok {
		return fmt.Errorf("API returned not ok")
	}

	return nil
}
