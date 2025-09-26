package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Config struct {
	UserName   string
	AvatarUrl  string
	WebhookUrl string
}

type Client interface {
	Send(message string) error
	SendWithPooling(message string) error
}

type client struct {
	httpClient *http.Client
	userName   string
	avatarUrl  string
	webhookUrl string
}

func NewClient(config Config) Client {
	return &client{
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 5 * time.Second,
			},
			Timeout: 10 * time.Second,
		},
		userName:   config.UserName,
		avatarUrl:  config.AvatarUrl,
		webhookUrl: config.WebhookUrl,
	}
}

func (c *client) Send(message string) error {
	payload, err := json.Marshal(map[string]string{
		"username":   c.userName,
		"avatar_url": c.avatarUrl,
		"content":    message,
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(c.webhookUrl, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook failed with status %s", resp.Status)
	}

	return nil
}

func (c *client) SendWithPooling(message string) error {
	payload, err := json.Marshal(map[string]string{
		"username":   c.userName,
		"avatar_url": c.avatarUrl,
		"content":    message,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.webhookUrl, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook failed with status %s", resp.Status)
	}

	return nil
}
