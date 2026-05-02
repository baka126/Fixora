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

	// Interactive Buttons
	var actionElements []slack.BlockElement

	if evidence.Namespace != "" && evidence.PodName != "" {
		logActionID := fmt.Sprintf("view-logs-%s-%s", evidence.Namespace, evidence.PodName)
		logBtn := slack.NewButtonBlockElement("view_logs", logActionID, slack.NewTextBlockObject("plain_text", "🔍 View Logs", false, false))
		actionElements = append(actionElements, logBtn)
	}

	if evidence.StackTrace != "" {
		traceActionID := fmt.Sprintf("view-trace-%s-%s", evidence.Namespace, evidence.PodName)
		traceBtn := slack.NewButtonBlockElement("view_trace", traceActionID, slack.NewTextBlockObject("plain_text", "📜 Show Stack Trace", false, false))
		actionElements = append(actionElements, traceBtn)
	}

	if evidence.FinOpsDetails != "" {
		finOpsActionID := fmt.Sprintf("view-finops-%s-%s", evidence.Namespace, evidence.PodName)
		finOpsBtn := slack.NewButtonBlockElement("view_finops", finOpsActionID, slack.NewTextBlockObject("plain_text", "💰 View FinOps Impact", false, false))
		actionElements = append(actionElements, finOpsBtn)
	}

	if evidence.ShowFixButton {
		fixBtn := slack.NewButtonBlockElement("execute_fix", "execute_fix", slack.NewTextBlockObject("plain_text", "⚡ Execute Fix", false, false))
		fixBtn.Style = slack.StylePrimary
		actionElements = append(actionElements, fixBtn)
	}

	if len(actionElements) > 0 {
		actionBlock := slack.NewActionBlock("", actionElements...)
		blocks = append(blocks, actionBlock)
	}

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

func sendSlackRemediationApproval(cfg *config.Config, namespace, pod, patch, callbackID string) error {
	if cfg.SlackToken == "" || cfg.SlackToken == "xoxb-your-token" {
		return nil
	}

	headerText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("🛠️ *Remediation Approval Required* for %s/%s", namespace, pod), false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	patchText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("Proposed Fix:\n```yaml\n%s\n```", patch), false, false)
	patchSection := slack.NewSectionBlock(patchText, nil, nil)

	approveBtn := slack.NewButtonBlockElement("approve", "approve", slack.NewTextBlockObject("plain_text", "Approve & Open PR", false, false))
	approveBtn.Style = slack.StylePrimary
	denyBtn := slack.NewButtonBlockElement("deny", "deny", slack.NewTextBlockObject("plain_text", "Ignore", false, false))
	denyBtn.Style = slack.StyleDanger

	actionBlock := slack.NewActionBlock(callbackID, approveBtn, denyBtn)

	return sendSlack(cfg, slack.MsgOptionBlocks(headerSection, patchSection, actionBlock))
}

func sendSlack(cfg *config.Config, msgOptions ...slack.MsgOption) error {
	api := slack.New(cfg.SlackToken)
	_, _, err := api.PostMessage(cfg.SlackChannel, msgOptions...)
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	return nil
}

// SendLogModal opens a Slack modal containing the scrubbed logs.
func SendLogModal(cfg *config.Config, triggerID, namespace, podName, title, content string) error {
	api := slack.New(cfg.SlackToken)

	titleText := slack.NewTextBlockObject("plain_text", title, false, false)
	closeText := slack.NewTextBlockObject("plain_text", "Close", false, false)

	headerText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("📄 *%s for %s/%s*", title, namespace, podName), false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	formattedContent := "Empty."
	if content != "" {
		formattedContent = fmt.Sprintf("```\n%s\n```", content)
	}

	// Split content if too long (Slack limit is 3000 chars per block)
	var blocks []slack.Block
	blocks = append(blocks, headerSection)

	if len(formattedContent) > 2900 {
		formattedContent = "```\n... [truncated] ...\n" + content[len(content)-2800:] + "\n```"
	}

	contentText := slack.NewTextBlockObject("mrkdwn", formattedContent, false, false)
	contentSection := slack.NewSectionBlock(contentText, nil, nil)
	blocks = append(blocks, contentSection)

	modalRequest := slack.ModalViewRequest{
		Type:   slack.VTModal,
		Title:  titleText,
		Close:  closeText,
		Blocks: slack.Blocks{BlockSet: blocks},
	}

	_, err := api.OpenView(triggerID, modalRequest)
	if err != nil {
		return fmt.Errorf("failed to open slack modal: %w", err)
	}
	return nil
}
