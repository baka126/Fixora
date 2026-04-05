package notifications

import (
	"fmt"
	"fixora/pkg/config"
	"github.com/slack-go/slack"
)

func SendNotification(cfg *config.Config, message string) error {
	return send(cfg, slack.MsgOptionText(message, false))
}

func SendInteractiveNotification(cfg *config.Config, message, callbackID string) error {
	approveBtn := slack.NewButtonBlockElement("approve", "approve", slack.NewTextBlockObject("plain_text", "Approve", false, false))
	approveBtn.Style = slack.StylePrimary
	denyBtn := slack.NewButtonBlockElement("deny", "deny", slack.NewTextBlockObject("plain_text", "Deny", false, false))
	denyBtn.Style = slack.StyleDanger

	actionBlock := slack.NewActionBlock(callbackID, approveBtn, denyBtn)
	msg := slack.NewTextBlockObject("mrkdwn", message, false, false)
	msgSection := slack.NewSectionBlock(msg, nil, nil)

	return send(cfg, slack.MsgOptionBlocks(msgSection, actionBlock))
}

func send(cfg *config.Config, msgOptions ...slack.MsgOption) error {
	if cfg.SlackToken == "" || cfg.SlackToken == "xoxb-your-token" {
		fmt.Printf("Slack token not configured, skipping notification
")
		return nil
	}

	api := slack.New(cfg.SlackToken)
	_, _, err := api.PostMessage(cfg.SlackChannel, msgOptions...)
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	return nil
}
