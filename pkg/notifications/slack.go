package notifications

import (
	"fmt"

	"fixora/pkg/config"
	"github.com/slack-go/slack"
)

func sendSlackEvidenceChain(cfg *config.Config, evidence EvidenceChain) error {
	if cfg.SlackToken == "" || cfg.SlackToken == "xoxb-your-token" {
		return nil
	}

	color := "#E01E5A" // Default Red-ish
	headerTitle := "*Fixora: Forensic Diagnostic Report*"

	if evidence.PredictiveWarning {
		color = "#ECB22E" // Slack Orange
		headerTitle = "*Fixora: Predictive Leak Warning*"
	}

	headerText := slack.NewTextBlockObject("mrkdwn", headerTitle, false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	metricSection := createSlackSection("📊 *Metric Proof*", evidence.MetricProof)
	contextSection := createSlackSection("🔍 *Cluster Context*", evidence.ClusterContext)
	patternSection := createSlackSection("📈 *Historical Pattern*", evidence.HistoricalPattern)
	timelineSection := createSlackSection("🕒 *Event Timeline*", evidence.EventTimeline)
	rootCauseSection := createSlackSection("🧠 *Root Cause*", evidence.RootCause)
	finOpsSection := createSlackSection("💰 *FinOps Impact*", evidence.FinOpsImpact)

	divider := slack.NewDividerBlock()

	blocks := []slack.Block{
		headerSection,
		divider,
		metricSection,
	}

	if evidence.PredictiveWarning && evidence.EstimatedHoursToOOM > 0 {
		oomText := fmt.Sprintf("⏳ *Estimated Hours until OOM:* %.1f hours", evidence.EstimatedHoursToOOM)
		oomSection := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", oomText, false, false), nil, nil)
		blocks = append(blocks, oomSection)
	}

	blocks = append(blocks,
		contextSection,
		patternSection,
		timelineSection,
		divider,
		rootCauseSection,
		finOpsSection,
	)

	attachment := slack.Attachment{
		Color:  color,
		Blocks: slack.Blocks{BlockSet: blocks},
	}

	return sendSlack(cfg, slack.MsgOptionAttachments(attachment))
}

func createSlackSection(title, content string) *slack.SectionBlock {
	text := fmt.Sprintf("%s\n%s", title, content)
	txtObj := slack.NewTextBlockObject("mrkdwn", text, false, false)
	return slack.NewSectionBlock(txtObj, nil, nil)
}

func sendSlackNotification(cfg *config.Config, message string) error {
	if cfg.SlackToken == "" || cfg.SlackToken == "xoxb-your-token" {
		return nil
	}
	return sendSlack(cfg, slack.MsgOptionText(message, false))
}

func sendSlackInteractiveNotification(cfg *config.Config, message, callbackID string) error {
	if cfg.SlackToken == "" || cfg.SlackToken == "xoxb-your-token" {
		return nil
	}

	approveBtn := slack.NewButtonBlockElement("approve", "approve", slack.NewTextBlockObject("plain_text", "Approve", false, false))
	approveBtn.Style = slack.StylePrimary
	denyBtn := slack.NewButtonBlockElement("deny", "deny", slack.NewTextBlockObject("plain_text", "Deny", false, false))
	denyBtn.Style = slack.StyleDanger

	actionBlock := slack.NewActionBlock(callbackID, approveBtn, denyBtn)
	msg := slack.NewTextBlockObject("mrkdwn", message, false, false)
	msgSection := slack.NewSectionBlock(msg, nil, nil)

	return sendSlack(cfg, slack.MsgOptionBlocks(msgSection, actionBlock))
}

func sendSlack(cfg *config.Config, msgOptions ...slack.MsgOption) error {
	api := slack.New(cfg.SlackToken)
	_, _, err := api.PostMessage(cfg.SlackChannel, msgOptions...)
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	return nil
}
