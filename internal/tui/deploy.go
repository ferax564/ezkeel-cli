package tui

import (
	"fmt"
	"strings"
)

// StepStatus represents the current state of a deploy step.
type StepStatus int

const (
	StepPending StepStatus = iota
	StepActive
	StepDone
	StepFailed
)

// DeployStep is a single step in the deploy pipeline.
type DeployStep struct {
	Label   string
	Status  StepStatus
	Message string
}

// DeployModel holds the full deploy progress state.
type DeployModel struct {
	AppName string
	Steps   []DeployStep
}

// DeployResult carries the outcome of a successful deployment.
type DeployResult struct {
	AppName string
	URL     string
	Server  string
	Stack   string
	TimeSec int
}

// NewDeployModel creates a DeployModel with all steps in pending state.
func NewDeployModel(appName string, steps []string) *DeployModel {
	m := &DeployModel{AppName: appName}
	for _, label := range steps {
		m.Steps = append(m.Steps, DeployStep{Label: label, Status: StepPending})
	}
	return m
}

// StartStep marks step i as active.
func (m *DeployModel) StartStep(i int) {
	if i >= 0 && i < len(m.Steps) {
		m.Steps[i].Status = StepActive
	}
}

// CompleteStep marks step i as done with an optional message.
func (m *DeployModel) CompleteStep(i int, msg string) {
	if i >= 0 && i < len(m.Steps) {
		m.Steps[i].Status = StepDone
		m.Steps[i].Message = msg
	}
}

// FailStep marks step i as failed with an error message.
func (m *DeployModel) FailStep(i int, errMsg string) {
	if i >= 0 && i < len(m.Steps) {
		m.Steps[i].Status = StepFailed
		m.Steps[i].Message = errMsg
	}
}

// View renders the deploy progress as a styled string.
func (m *DeployModel) View() string {
	var sb strings.Builder
	sb.WriteString(BoldStyle.Render("deploying " + m.AppName))
	sb.WriteString("\n\n")

	for _, step := range m.Steps {
		switch step.Status {
		case StepDone:
			display := step.Label
			if step.Message != "" {
				display = step.Message
			}
			sb.WriteString(GreenStyle.Render(IconDone+" "+display) + "\n")
		case StepActive:
			sb.WriteString(GreenStyle.Render(IconActive+" "+step.Label) + "\n")
		case StepPending:
			sb.WriteString(DimStyle.Render(IconPending+" "+step.Label) + "\n")
		case StepFailed:
			sb.WriteString(ErrorStyle.Render(IconFail+" "+step.Label) + "\n")
			if step.Message != "" {
				sb.WriteString(ErrorStyle.Render("  "+step.Message) + "\n")
			}
		}
	}

	return BoxStyle.Render(sb.String())
}

// RenderSuccess renders a success summary box.
func RenderSuccess(r *DeployResult) string {
	var sb strings.Builder
	sb.WriteString(GreenStyle.Render(IconDone + " " + r.AppName + " deployed"))
	sb.WriteString("\n\n")
	sb.WriteString(DimStyle.Render("url:    ") + r.URL + "\n")
	sb.WriteString(DimStyle.Render("server: ") + r.Server + "\n")
	sb.WriteString(DimStyle.Render("stack:  ") + r.Stack + "\n")
	sb.WriteString(DimStyle.Render("time:   ") + fmt.Sprintf("%ds", r.TimeSec))
	return BoxStyle.Render(sb.String())
}

// RenderFailure renders a failure summary box.
func RenderFailure(appName string, errMsg string) string {
	var sb strings.Builder
	sb.WriteString(ErrorStyle.Render(IconFail + " " + appName + " deploy failed"))
	sb.WriteString("\n\n")
	sb.WriteString(ErrorStyle.Render(errMsg))
	return FailureBoxStyle.Render(sb.String())
}
