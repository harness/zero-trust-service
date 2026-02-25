package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// blockedOrgs — requests from these orgs are blocked.
var blockedOrgs = map[string]bool{
	"production": true,
}

// shellTaskTypes — task types considered shell script execution.
var shellTaskTypes = map[string]bool{
	"SHELL_SCRIPT_TASK_NG": true,
	"COMMAND_TASK_NG":      true,
}

type ztsMetadata struct {
	OrgIdentifier string `json:"orgIdentifier"`
}

type taskData struct {
	TaskType string `json:"taskType"`
}

type taskPackage struct {
	ZTSMetadata *ztsMetadata `json:"ztsMetadata"`
	Data        *taskData    `json:"data"`
}

type verifyRequest struct {
	TaskPackage *taskPackage `json:"taskPackage"`
}

type validateResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

func validate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	org := ""
	taskType := ""
	if req.TaskPackage != nil {
		if req.TaskPackage.ZTSMetadata != nil {
			org = req.TaskPackage.ZTSMetadata.OrgIdentifier
		}
		if req.TaskPackage.Data != nil {
			taskType = req.TaskPackage.Data.TaskType
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if blockedOrgs[org] && shellTaskTypes[taskType] {
		resp := validateResponse{
			Allowed: false,
			Reason:  fmt.Sprintf("Shell scripts are blocked in org '%s' by policy", org),
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	json.NewEncoder(w).Encode(validateResponse{Allowed: true})
}

func main() {
	http.HandleFunc("/zts/validate", validate)
	log.Println("Webhook server running on http://localhost:5050")
	log.Fatal(http.ListenAndServe(":5050", nil))
}
