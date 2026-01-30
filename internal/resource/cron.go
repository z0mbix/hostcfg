package resource

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/z0mbix/hostcfg/internal/config"
)

func init() {
	Register("cron", NewCronResource)
}

// CronResource manages cron job entries
type CronResource struct {
	name      string
	config    config.CronResourceConfig
	dependsOn []string
}

// NewCronResource creates a new cron resource from HCL
func NewCronResource(name string, body hcl.Body, dependsOn []string, ctx *hcl.EvalContext) (Resource, error) {
	var cfg config.CronResourceConfig
	diags := gohcl.DecodeBody(body, ctx, &cfg)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode cron resource: %s", diags.Error())
	}

	return &CronResource{
		name:      name,
		config:    cfg,
		dependsOn: dependsOn,
	}, nil
}

func (r *CronResource) Type() string { return "cron" }
func (r *CronResource) Name() string { return r.name }

func (r *CronResource) Validate() error {
	if r.config.Command == "" {
		return fmt.Errorf("cron.%s: command is required", r.name)
	}
	if r.config.Schedule == "" {
		return fmt.Errorf("cron.%s: schedule is required", r.name)
	}
	return nil
}

func (r *CronResource) Dependencies() []string {
	return r.dependsOn
}

// cronEntry returns the formatted cron entry line
func (r *CronResource) cronEntry() string {
	return fmt.Sprintf("%s %s # hostcfg: %s", r.config.Schedule, r.config.Command, r.name)
}

// marker returns the comment marker used to identify this cron job
func (r *CronResource) marker() string {
	return fmt.Sprintf("# hostcfg: %s", r.name)
}

func (r *CronResource) Read(ctx context.Context) (*State, error) {
	state := NewState()

	cronUser := r.getUser()

	// Read the user's crontab
	cmd := exec.CommandContext(ctx, "crontab", "-u", cronUser, "-l")
	output, err := cmd.Output()
	if err != nil {
		// No crontab for user is not an error
		state.Exists = false
		return state, nil
	}

	// Parse the crontab to find our entry
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	marker := r.marker()

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, marker) {
			// Found our managed cron entry
			state.Exists = true
			// Extract schedule and command (everything before the marker)
			parts := strings.Split(line, marker)
			if len(parts) > 0 {
				entry := strings.TrimSpace(parts[0])
				// Parse schedule (first 5 fields) and command
				fields := strings.Fields(entry)
				if len(fields) >= 6 {
					state.Attributes["schedule"] = strings.Join(fields[:5], " ")
					state.Attributes["command"] = strings.Join(fields[5:], " ")
				}
			}
			state.Attributes["user"] = cronUser
			return state, nil
		}
	}

	state.Exists = false
	return state, nil
}

func (r *CronResource) Diff(ctx context.Context, current *State) (*Plan, error) {
	plan := &Plan{
		Before: current,
		After:  NewState(),
	}

	ensure := "present"
	if r.config.Ensure != nil {
		ensure = *r.config.Ensure
	}

	// Handle ensure = absent
	if ensure == "absent" {
		if current.Exists {
			plan.Action = ActionDelete
			plan.Changes = append(plan.Changes, Change{
				Attribute: "entry",
				Old:       r.cronEntry(),
				New:       nil,
			})
		}
		return plan, nil
	}

	// Entry doesn't exist - create it
	if !current.Exists {
		plan.Action = ActionCreate
		plan.After.Exists = true
		plan.Changes = append(plan.Changes, Change{
			Attribute: "schedule",
			Old:       nil,
			New:       r.config.Schedule,
		})
		plan.Changes = append(plan.Changes, Change{
			Attribute: "command",
			Old:       nil,
			New:       r.config.Command,
		})
		plan.Changes = append(plan.Changes, Change{
			Attribute: "user",
			Old:       nil,
			New:       r.getUser(),
		})
		return plan, nil
	}

	// Entry exists - check for changes
	plan.After.Exists = true

	currentSchedule, _ := current.Attributes["schedule"].(string)
	currentCommand, _ := current.Attributes["command"].(string)

	if currentSchedule != r.config.Schedule {
		plan.Changes = append(plan.Changes, Change{
			Attribute: "schedule",
			Old:       currentSchedule,
			New:       r.config.Schedule,
		})
	}

	if currentCommand != r.config.Command {
		plan.Changes = append(plan.Changes, Change{
			Attribute: "command",
			Old:       currentCommand,
			New:       r.config.Command,
		})
	}

	if len(plan.Changes) > 0 {
		plan.Action = ActionUpdate
	}

	return plan, nil
}

func (r *CronResource) Apply(ctx context.Context, plan *Plan, apply bool) error {
	if !apply || !plan.HasChanges() {
		return nil
	}

	cronUser := r.getUser()
	marker := r.marker()

	// Get current crontab
	cmd := exec.CommandContext(ctx, "crontab", "-u", cronUser, "-l")
	output, _ := cmd.Output()
	lines := strings.Split(string(output), "\n")

	switch plan.Action {
	case ActionDelete:
		var newLines []string
		for _, line := range lines {
			if !strings.Contains(line, marker) {
				newLines = append(newLines, line)
			}
		}
		return r.writeCrontab(ctx, cronUser, strings.Join(newLines, "\n"))

	case ActionCreate:
		newContent := string(output)
		if !strings.HasSuffix(newContent, "\n") && newContent != "" {
			newContent += "\n"
		}
		newContent += r.cronEntry() + "\n"
		return r.writeCrontab(ctx, cronUser, newContent)

	case ActionUpdate:
		var newLines []string
		for _, line := range lines {
			if strings.Contains(line, marker) {
				newLines = append(newLines, r.cronEntry())
			} else {
				newLines = append(newLines, line)
			}
		}
		return r.writeCrontab(ctx, cronUser, strings.Join(newLines, "\n"))
	}

	return nil
}

func (r *CronResource) writeCrontab(ctx context.Context, cronUser, content string) error {
	// Write to a temp file
	tmpfile, err := os.CreateTemp("", "crontab")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	if _, err := tmpfile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpfile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Install the crontab
	cmd := exec.CommandContext(ctx, "crontab", "-u", cronUser, tmpfile.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install crontab: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (r *CronResource) getUser() string {
	if r.config.User != nil {
		return *r.config.User
	}
	// Default to current user
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "root"
}
