package notifier

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/config"
)

// Notifier는 외부 알림 발송 계약이다.
type Notifier interface {
	NotifyWarning(ctx context.Context, tokenID int64, tokenName, tokenUUID string, requestCount int64) error
}

// NtfyNotifier는 ntfy.sh HTTP API를 통해 알림을 발송한다.
type NtfyNotifier struct {
	serverURL     string
	topic         string
	proxyPublicURL string
	webhookSecret  string
	client        *http.Client
}

// NoopNotifier는 알림 비활성화 환경에서 사용하는 no-op 구현체다.
type NoopNotifier struct{}

func (n *NoopNotifier) NotifyWarning(_ context.Context, _ int64, _, _ string, _ int64) error {
	return nil
}

func New(cfg config.NtfyConfig, proxyPublicURL string) Notifier {
	if !cfg.Enabled {
		return &NoopNotifier{}
	}
	return &NtfyNotifier{
		serverURL:      strings.TrimRight(cfg.ServerURL, "/"),
		topic:          cfg.Topic,
		proxyPublicURL: strings.TrimRight(proxyPublicURL, "/"),
		webhookSecret:  cfg.WebhookSecret,
		client:         &http.Client{Timeout: 5 * time.Second},
	}
}

func (n *NtfyNotifier) NotifyWarning(ctx context.Context, tokenID int64, tokenName, tokenUUID string, requestCount int64) error {
	url := fmt.Sprintf("%s/%s", n.serverURL, n.topic)

	blockURL := fmt.Sprintf("%s/webhook/api/token/%d/block", n.proxyPublicURL, tokenID)
	actions := fmt.Sprintf(
		"http, 차단, %s, method=POST, headers.X-Webhook-Secret=%s; http, 무시, #",
		blockURL,
		n.webhookSecret,
	)

	maskedUUID := maskUUID(tokenUUID)
	body := fmt.Sprintf(
		"🚨 토큰 이상 사용 감지\n\n토큰: %s\n토큰명: %s\n분당 요청: %d건\n\n토큰을 차단하시겠습니까?",
		maskedUUID, tokenName, requestCount,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Title", "BSSM Developers - API Token WARNING")
	req.Header.Set("Tags", "warning,rotating_light")
	req.Header.Set("Priority", "high")
	req.Header.Set("Actions", actions)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func maskUUID(uuid string) string {
	if len(uuid) <= 8 {
		return uuid
	}
	return uuid[:8] + "..."
}
