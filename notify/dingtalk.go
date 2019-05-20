package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-kit/kit/log"
	commoncfg "github.com/prometheus/common/config"

	"github.com/prometheus/alertmanager/config"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
)

// Dingtalk implements a Notifier for dingtalk notifications.
type Dingtalk struct {
	conf   *config.DingtalkConfig
	tmpl   *template.Template
	logger log.Logger
}

// NewDingtalk returns a new Dingtalk notifier.
func NewDingtalk(c *config.DingtalkConfig, t *template.Template, l log.Logger) *Dingtalk {
	return &Dingtalk{conf: c, tmpl: t, logger: l}
}

// Notify implements the Notifier interface.
func (d *Dingtalk) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {

	var (
		tmplErr error
		data    = d.tmpl.Data(receiverName(ctx, d.logger), groupLabels(ctx, d.logger), as...)
		tmpl    = tmplText(d.tmpl, data, &tmplErr)
		title   = tmpl(d.conf.Title)
		content = tmpl(d.conf.Content)
	)
	if tmplErr != nil {
		return false, fmt.Errorf("failed to template 'title' or 'content': %v", tmplErr)
	}

	var msg = &dingTalkNotification{
		MessageType: "markdown",
		Markdown: &dingTalkNotificationMarkdown{
			Title: title,
			Text:  content,
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(msg); err != nil {
		return false, err
	}

	req, err := http.NewRequest("POST", d.conf.WebhookURL, &buf)
	if err != nil {
		return true, err
	}
	req.Header.Set("Content-Type", "application/json")

	c, err := commoncfg.NewClientFromConfig(*d.conf.HTTPConfig, "dingtalk")
	if err != nil {
		return false, err
	}

	resp, err := c.Do(req.WithContext(ctx))
	if err != nil {
		return true, err
	}
	resp.Body.Close()

	return d.retry(resp.StatusCode)
}

func (d *Dingtalk) retry(statusCode int) (bool, error) {
	// Webhooks are assumed to respond with 2xx response codes on a successful
	// request and 5xx response codes are assumed to be recoverable.
	if statusCode/100 != 2 {
		return (statusCode/100 == 5), fmt.Errorf("unexpected status code %v from %s", statusCode, d.conf.WebhookURL)
	}

	return false, nil
}

/*
Dingtalk
*/

type dingTalkNotification struct {
	MessageType string                        `json:"msgtype"`
	Markdown    *dingTalkNotificationMarkdown `json:"markdown,omitempty"`
}

type dingTalkNotificationMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}
