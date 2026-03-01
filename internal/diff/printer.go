package diff

import (
	"fmt"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/z0mbix/cliout"
	"github.com/z0mbix/hostcfg/internal/resource"
)

// Printer handles printing resource diffs with colors
type Printer struct {
	out *cliout.Output
}

// NewPrinter creates a new diff printer
func NewPrinter(out *cliout.Output) *Printer {
	return &Printer{
		out: out,
	}
}

// PrintPlan prints a resource plan with colored diff output
func (p *Printer) PrintPlan(r resource.Resource, plan *resource.Plan) {
	if plan == nil {
		return
	}

	// Handle skipped resources
	if plan.Action == resource.ActionSkip {
		p.printSkipped(r, plan)
		return
	}

	if !plan.HasChanges() {
		return
	}

	// Print description first if present
	if desc := r.Description(); desc != "" {
		p.out.Infof("%s", p.out.Colorize(fmt.Sprintf("» %s", desc), cliout.ColorBrightBlack))
	}

	// Print resource header with action symbol
	switch plan.Action {
	case resource.ActionCreate:
		p.out.Infof("%s %s", p.out.Colorize("+", cliout.ColorGreen), p.out.Colorize(resource.ID(r), cliout.ColorGreen))
	case resource.ActionUpdate:
		p.out.Infof("%s %s", p.out.Colorize("~", cliout.ColorYellow), p.out.Colorize(resource.ID(r), cliout.ColorYellow))
	case resource.ActionDelete:
		p.out.Infof("%s %s", p.out.Colorize("-", cliout.ColorRed), p.out.Colorize(resource.ID(r), cliout.ColorRed))
	}

	// Print each change
	for _, change := range plan.Changes {
		p.printChange(plan.Action, change)
	}

	p.out.Info("")
}

// printSkipped prints a skipped resource with its skip reason
func (p *Printer) printSkipped(r resource.Resource, plan *resource.Plan) {
	skipReason := plan.SkipReason
	if skipReason == "" {
		skipReason = "condition not met"
	}

	// Print description first if present
	if desc := r.Description(); desc != "" {
		p.out.Infof("%s", p.out.Colorize(fmt.Sprintf("» %s", desc), cliout.ColorBrightBlack))
	}

	p.out.Infof("%s (skipped: %s)", p.out.Colorize("# "+resource.ID(r), cliout.ColorCyan), skipReason)
	p.out.Info("")
}

func (p *Printer) printChange(action resource.Action, change resource.Change) {
	switch action {
	case resource.ActionCreate:
		p.printAddition(change)
	case resource.ActionUpdate:
		p.printModification(change)
	case resource.ActionDelete:
		p.printDeletion(change)
	}
}

func (p *Printer) printAddition(change resource.Change) {
	p.out.Infof("    %s", p.out.Colorize(fmt.Sprintf("+ %s = %s", change.Attribute, p.formatValue(change.New)), cliout.ColorGreen))
}

func (p *Printer) printDeletion(change resource.Change) {
	p.out.Infof("    %s", p.out.Colorize(fmt.Sprintf("- %s = %s", change.Attribute, p.formatValue(change.Old)), cliout.ColorRed))
}

func (p *Printer) printModification(change resource.Change) {
	// Check if this is content that should show a text diff
	if change.Attribute == "content" {
		oldStr, oldOk := change.Old.(string)
		newStr, newOk := change.New.(string)
		if oldOk && newOk {
			p.out.Infof("    %s", p.out.Colorize(fmt.Sprintf("~ %s: (changed)", change.Attribute), cliout.ColorYellow))
			p.printTextDiff(oldStr, newStr)
			return
		}
	}

	// Regular attribute change
	p.out.Infof("    %s", p.out.Colorize(fmt.Sprintf("~ %s: %s => %s",
		change.Attribute,
		p.formatValue(change.Old),
		p.formatValue(change.New)), cliout.ColorYellow))
}

func (p *Printer) printTextDiff(old, new string) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(old, new, false)

	// Print line-by-line diff
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	// Use unified diff style
	for i := 0; i < len(oldLines) || i < len(newLines); i++ {
		var oldLine, newLine string
		hasOld := i < len(oldLines)
		hasNew := i < len(newLines)

		if hasOld {
			oldLine = oldLines[i]
		}
		if hasNew {
			newLine = newLines[i]
		}

		if hasOld && hasNew && oldLine == newLine {
			// Unchanged line
			p.out.Infof("        %s", oldLine)
		} else {
			if hasOld && oldLine != "" {
				p.out.Infof("      %s", p.out.Colorize(fmt.Sprintf("- %s", oldLine), cliout.ColorRed))
			}
			if hasNew && newLine != "" {
				p.out.Infof("      %s", p.out.Colorize(fmt.Sprintf("+ %s", newLine), cliout.ColorGreen))
			}
		}
	}

	_ = diffs // suppress unused warning, we may use this for more detailed diffs later
}

func (p *Printer) formatValue(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// PrintSummary prints the plan summary
func (p *Printer) PrintSummary(toAdd, toChange, toDestroy, toSkip int) {
	if toSkip > 0 {
		p.out.Infof("Plan: %d to add, %d to change, %d to destroy, %d to skip.",
			toAdd, toChange, toDestroy, toSkip)
	} else {
		p.out.Infof("Plan: %d to add, %d to change, %d to destroy.",
			toAdd, toChange, toDestroy)
	}
}

// PrintNoChanges prints when there are no changes
func (p *Printer) PrintNoChanges() {
	p.out.Success("No changes. Infrastructure is up-to-date.")
}
