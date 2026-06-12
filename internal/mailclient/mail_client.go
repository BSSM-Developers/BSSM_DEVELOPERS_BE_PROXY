package mailclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type MailClient struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

type sendRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func New(baseURL string, logger *zap.Logger) *MailClient {
	return &MailClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
		logger:  logger,
	}
}

// NewNoop returns a disabled client that silently drops all sends.
func NewNoop(logger *zap.Logger) *MailClient {
	return &MailClient{logger: logger}
}

func (c *MailClient) Send(ctx context.Context, to, subject, body string) {
	if c.client == nil {
		return
	}

	payload := sendRequest{To: to, Subject: subject, Body: body}
	b, err := json.Marshal(payload)
	if err != nil {
		c.logger.Error("mail payload marshal 실패", zap.Error(err))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/mail/send", c.baseURL), bytes.NewReader(b))
	if err != nil {
		c.logger.Error("mail 요청 생성 실패", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("mail 발송 실패", zap.String("to", to), zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("mail service 오류 응답", zap.String("to", to), zap.Int("status", resp.StatusCode))
	}
}
