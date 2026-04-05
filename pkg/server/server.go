package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"fixora/pkg/controller"
	"github.com/slack-go/slack"
)

type Server struct {
	controller *controller.Controller
}

func New(ctrl *controller.Controller) *Server {
	return &Server{
		controller: ctrl,
	}
}

func (s *Server) Start() {
	http.HandleFunc("/slack/interactive", s.handleInteractive)
	fmt.Println("[INFO] Server listening on port 8080")
	http.ListenAndServe(":8080", nil)
}

func (s *Server) handleInteractive(w http.ResponseWriter, r *http.Request) {
	var payload slack.InteractionCallback
	err := json.Unmarshal([]byte(r.FormValue("payload")), &payload)
	if err != nil {
		fmt.Printf("Error unmarshalling interaction callback: %s
", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if payload.Type != slack.InteractionTypeBlockActions {
		return
	}

	if len(payload.ActionCallback.BlockActions) == 0 {
		return
	}

	action := payload.ActionCallback.BlockActions[0]
	if action.ActionID != "approve" {
		fmt.Printf("Received deny action for %s
", payload.CallbackID)
		// TODO: Handle deny action
		return
	}

	parts := strings.Split(payload.CallbackID, "-")
	if len(parts) != 4 || parts[0] != "rollout" || parts[1] != "restart" {
		fmt.Printf("Invalid callback ID: %s
", payload.CallbackID)
		return
	}

	namespace := parts[2]
	deploymentName := parts[3]

	fmt.Printf("Received approval to rollout restart Deployment %s/%s
", namespace, deploymentName)
	s.controller.PerformRolloutRestart(namespace, deploymentName)

	w.WriteHeader(http.StatusOK)
}
