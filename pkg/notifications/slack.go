package notifications

import (
	"fmt"

	"fixora/pkg/config"
	"github.com/slack-go/slack"
)

type EvidenceChain struct {
	MetricProof       string
	ClusterContext    string
	HistoricalPattern string
	EventTimeline     string
	RootCause         string
	FinOpsImpact      string
}

func SendEvidenceChain(cfg *config.Config, evidence EvidenceChain) error {
	headerText := slack.NewTextBlockObject("mrkdwn", "*Fixora: Forensic Diagnostic Report*", false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	metricSection := createSection("📊 *Metric Proof*", evidence.MetricProof)
	contextSection := createSection("🔍 *Cluster Context*", evidence.ClusterContext)
	patternSection := createSection("📈 *Historical Pattern*", evidence.HistoricalPattern)
	timelineSection := createSection("🕒 *Event Timeline*", evidence.EventTimeline)
	rootCauseSection := createSection("🧠 *Root Cause*", evidence.RootCause)
	finOpsSection := createSection("💰 *FinOps Impact*", evidence.FinOpsImpact)

	divider := slack.NewDividerBlock()

	blocks := []slack.Block{
		headerSection,
		divider,
		metricSection,
		contextSection,
		patternSection,
		timelineSection,
		divider,
		rootCauseSection,
		finOpsSection,
	}

	return send(cfg, slack.MsgOptionBlocks(blocks...))
}

func createSection(title, content string) *slack.SectionBlock {
	text := fmt.Sprintf("%s\n%s", title, content)
	txtObj := slack.NewTextBlockObject("mrkdwn", text, false, false)
	return slack.NewSectionBlock(txtObj, nil, nil)
}

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
		fmt.Printf("Slack token not configured, skipping notification\n")
		return nil
	}

	api := slack.New(cfg.SlackToken)
	_, _, err := api.PostMessage(cfg.SlackChannel, msgOptions...)
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	return nil
}
