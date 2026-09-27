package cmd

import "net/http"

func (s *dashServer) handleAgentSettings(w http.ResponseWriter, _ *http.Request) {
	writeErr(w, http.StatusGone, "Agent settings are configured in Orca. Set enabled agents and the default there; existing workers resume with their recorded agent and model.")
}
