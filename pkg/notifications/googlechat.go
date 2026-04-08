package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"fixora/pkg/config"
)

type GoogleChatPayload struct {
	Text    string             `json:"text,omitempty"`
	CardsV2 []GoogleChatCardV2 `json:"cardsV2,omitempty"`
}

type GoogleChatCardV2 struct {
	CardId string         `json:"cardId"`
	Card   GoogleChatCard `json:"card"`
}

type GoogleChatCard struct {
	Header   GoogleChatHeader    `json:"header"`
	Sections []GoogleChatSection `json:"sections"`
}

type GoogleChatHeader struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
}

type GoogleChatSection struct {
	Header  string             `json:"header,omitempty"`
	Widgets []GoogleChatWidget `json:"widgets"`
}

type GoogleChatWidget struct {
	TextParagraph *GoogleChatTextParagraph `json:"textParagraph,omitempty"`
	ButtonList    *GoogleChatButtonList    `json:"buttonList,omitempty"`
}

type GoogleChatTextParagraph struct {
	Text string `json:"text"`
}

type GoogleChatButtonList struct {
	Buttons []GoogleChatButton `json:"buttons"`
}

type GoogleChatButton struct {
	Text    string            `json:"text"`
	OnClick GoogleChatOnClick `json:"onClick"`
}

type GoogleChatOnClick struct {
	Action *GoogleChatAction `json:"action,omitempty"`
}

type GoogleChatAction struct {
	Function   string                  `json:"function"`
	Parameters []GoogleChatActionParam `json:"parameters"`
}

type GoogleChatActionParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func sendGoogleChatEvidenceChain(cfg *config.Config, evidence EvidenceChain) error {
	if cfg.GoogleChatWebhookURL == "" {
		return nil
	}

	payload := GoogleChatPayload{
		CardsV2: []GoogleChatCardV2{
			{
				CardId: "forensic_report",
				Card: GoogleChatCard{
					Header: GoogleChatHeader{
						Title:    "Fixora: Forensic Diagnostic Report",
						Subtitle: "Automated root cause analysis",
					},
					Sections: []GoogleChatSection{
						{
							Widgets: []GoogleChatWidget{
								{TextParagraph: &GoogleChatTextParagraph{Text: "<b>📊 Metric Proof</b><br>" + evidence.MetricProof}},
								{TextParagraph: &GoogleChatTextParagraph{Text: "<b>🔍 Cluster Context</b><br>" + evidence.ClusterContext}},
								{TextParagraph: &GoogleChatTextParagraph{Text: "<b>📈 Historical Pattern</b><br>" + evidence.HistoricalPattern}},
								{TextParagraph: &GoogleChatTextParagraph{Text: "<b>🕒 Event Timeline</b><br>" + evidence.EventTimeline}},
							},
						},
						{
							Widgets: []GoogleChatWidget{
								{TextParagraph: &GoogleChatTextParagraph{Text: "<b>🧠 Root Cause</b><br>" + evidence.RootCause}},
								{TextParagraph: &GoogleChatTextParagraph{Text: "<b>💰 FinOps Impact</b><br>" + evidence.FinOpsImpact}},
							},
						},
					},
				},
			},
		},
	}

	return sendGoogleChat(cfg, payload)
}

func sendGoogleChatNotification(cfg *config.Config, message string) error {
	if cfg.GoogleChatWebhookURL == "" {
		return nil
	}

	payload := GoogleChatPayload{
		Text: message,
	}

	return sendGoogleChat(cfg, payload)
}

func sendGoogleChatInteractiveNotification(cfg *config.Config, message, callbackID string) error {
	if cfg.GoogleChatWebhookURL == "" {
		return nil
	}

	// For simple webhooks, interaction events are more complex to handle (requires Chat App).
	// We'll send the message with an explanation if interaction isn't supported, or format the buttons.
	payload := GoogleChatPayload{
		CardsV2: []GoogleChatCardV2{
			{
				CardId: "interactive_notification",
				Card: GoogleChatCard{
					Header: GoogleChatHeader{
						Title: "Fixora Action Required",
					},
					Sections: []GoogleChatSection{
						{
							Widgets: []GoogleChatWidget{
								{TextParagraph: &GoogleChatTextParagraph{Text: message}},
								{
									ButtonList: &GoogleChatButtonList{
										Buttons: []GoogleChatButton{
											{
												Text: "Approve",
												OnClick: GoogleChatOnClick{
													Action: &GoogleChatAction{
														Function: "approve_action",
														Parameters: []GoogleChatActionParam{
															{Key: "callback_id", Value: callbackID},
														},
													},
												},
											},
											{
												Text: "Deny",
												OnClick: GoogleChatOnClick{
													Action: &GoogleChatAction{
														Function: "deny_action",
														Parameters: []GoogleChatActionParam{
															{Key: "callback_id", Value: callbackID},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return sendGoogleChat(cfg, payload)
}

func sendGoogleChat(cfg *config.Config, payload GoogleChatPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal google chat payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, cfg.GoogleChatWebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create google chat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send google chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("google chat webhook returned non-200 status: %d", resp.StatusCode)
	}

	return nil
}
