package herdr

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrUnavailable means reviewr is not running with a usable Herdr host.
var ErrUnavailable = errors.New("Herdr agent sampling is unavailable")

// AgentSample is the minimal worktree turn signal from Herdr.
type AgentSample struct {
	Status string
	CWD    string
}

// AgentSampler owns the bounded Herdr subprocess used by turn tracking.
type AgentSampler struct {
	context Context
	runner  commandRunner
	timeout time.Duration
}

// NewAgentSampler creates a sampler from the immutable startup context.
func NewAgentSampler(host Context) *AgentSampler {
	return &AgentSampler{context: host, runner: execRunner{}, timeout: commandTimeout}
}

// Available reports whether the captured host can enumerate agents.
func (sampler *AgentSampler) Available() bool {
	return sampler != nil && sampler.context.canSampleAgents()
}

// Samples lists every Herdr agent; worktree membership belongs to the caller.
func (sampler *AgentSampler) Samples() ([]AgentSample, error) {
	if !sampler.Available() {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), sampler.timeout)
	defer cancel()
	output, err := sampler.runner.Run(ctx, sampler.context.BinPath(), "agent", "list")
	if err != nil {
		return nil, err
	}
	var response struct {
		Result *struct {
			Agents *[]struct {
				Agent  *string `json:"agent"`
				Status string  `json:"agent_status"`
				CWD    string  `json:"cwd"`
				PaneID string  `json:"pane_id"`
			} `json:"agents"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, err
	}
	if response.Result == nil || response.Result.Agents == nil {
		return nil, errors.New("Herdr agent list response has no agents array")
	}
	result := make([]AgentSample, 0, len(*response.Result.Agents))
	for _, agent := range *response.Result.Agents {
		if agent.Agent == nil || (sampler.context.PaneID() != "" && agent.PaneID == sampler.context.PaneID()) {
			continue
		}
		result = append(result, AgentSample{Status: agent.Status, CWD: agent.CWD})
	}
	return result, nil
}
