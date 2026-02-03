package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/z0mbix/hostcfg/internal/resource"
)

// Printer handles printing resource diffs with colors
type Printer struct {
	out       io.Writer
	useColors bool
}

// NewPrinter creates a new diff printer
func NewPrinter(out io.Writer, useColors bool) *Printer {
	if !useColors {
		color.NoColor = true
	}
	return &Printer{
		out:       out,
		useColors: useColors,
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

	// Print resource header with action symbol
	switch plan.Action {
	case resource.ActionCreate:
		p.printHeader("+", resource.ID(r), color.FgGreen)
	case resource.ActionUpdate:
		p.printHeader("~", resource.ID(r), color.FgYellow)
	case resource.ActionDelete:
		p.printHeader("-", resource.ID(r), color.FgRed)
	}

	// Print each change
	for _, change := range plan.Changes {
		p.printChange(plan.Action, change)
	}

	_, _ = fmt.Fprintln(p.out)
}

// printSkipped prints a skipped resource with its skip reason
func (p *Printer) printSkipped(r resource.Resource, plan *resource.Plan) {
	skipReason := plan.SkipReason
	if skipReason == "" {
		skipReason = "condition not met"
	}

	if p.useColors {
		cyan := color.New(color.FgCyan, color.Bold)
		_, _ = cyan.Fprintf(p.out, "# %s", resource.ID(r))
		_, _ = fmt.Fprintf(p.out, " (skipped: %s)\n", skipReason)
	} else {
		_, _ = fmt.Fprintf(p.out, "# %s (skipped: %s)\n", resource.ID(r), skipReason)
	}
	_, _ = fmt.Fprintln(p.out)
}

func (p *Printer) printHeader(symbol, resourceID string, c color.Attribute) {
	if p.useColors {
		colored := color.New(c, color.Bold)
		_, _ = colored.Fprintf(p.out, "%s %s\n", symbol, resourceID)
	} else {
		_, _ = fmt.Fprintf(p.out, "%s %s\n", symbol, resourceID)
	}
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
	green := color.New(color.FgGreen)
	if p.useColors {
		_, _ = green.Fprintf(p.out, "    + %s = %s\n", change.Attribute, p.formatValue(change.New))
	} else {
		_, _ = fmt.Fprintf(p.out, "    + %s = %s\n", change.Attribute, p.formatValue(change.New))
	}
}

func (p *Printer) printDeletion(change resource.Change) {
	red := color.New(color.FgRed)
	if p.useColors {
		_, _ = red.Fprintf(p.out, "    - %s = %s\n", change.Attribute, p.formatValue(change.Old))
	} else {
		_, _ = fmt.Fprintf(p.out, "    - %s = %s\n", change.Attribute, p.formatValue(change.Old))
	}
}

func (p *Printer) printModification(change resource.Change) {
	yellow := color.New(color.FgYellow)

	// Check if this is content that should show a text diff
	if change.Attribute == "content" {
		oldStr, oldOk := change.Old.(string)
		newStr, newOk := change.New.(string)
		if oldOk && newOk {
			if p.useColors {
				_, _ = yellow.Fprintf(p.out, "    ~ %s: (changed)\n", change.Attribute)
			} else {
				_, _ = fmt.Fprintf(p.out, "    ~ %s: (changed)\n", change.Attribute)
			}
			p.printTextDiff(oldStr, newStr)
			return
		}
	}

	// Regular attribute change
	if p.useColors {
		_, _ = yellow.Fprintf(p.out, "    ~ %s: %s => %s\n",
			change.Attribute,
			p.formatValue(change.Old),
			p.formatValue(change.New))
	} else {
		_, _ = fmt.Fprintf(p.out, "    ~ %s: %s => %s\n",
			change.Attribute,
			p.formatValue(change.Old),
			p.formatValue(change.New))
	}
}

func (p *Printer) printTextDiff(old, new string) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(old, new, false)

	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

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
			_, _ = fmt.Fprintf(p.out, "        %s\n", oldLine)
		} else {
			if hasOld && oldLine != "" {
				if p.useColors {
					_, _ = red.Fprintf(p.out, "      - %s\n", oldLine)
				} else {
					_, _ = fmt.Fprintf(p.out, "      - %s\n", oldLine)
				}
			}
			if hasNew && newLine != "" {
				if p.useColors {
					_, _ = green.Fprintf(p.out, "      + %s\n", newLine)
				} else {
					_, _ = fmt.Fprintf(p.out, "      + %s\n", newLine)
				}
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
		_, _ = fmt.Fprintf(p.out, "Plan: %d to add, %d to change, %d to destroy, %d to skip.\n",
			toAdd, toChange, toDestroy, toSkip)
	} else {
		_, _ = fmt.Fprintf(p.out, "Plan: %d to add, %d to change, %d to destroy.\n",
			toAdd, toChange, toDestroy)
	}
}

// PrintNoChanges prints when there are no changes
func (p *Printer) PrintNoChanges() {
	green := color.New(color.FgGreen)
	if p.useColors {
		_, _ = green.Fprintln(p.out, "No changes. Infrastructure is up-to-date.")
	} else {
		_, _ = fmt.Fprintln(p.out, "No changes. Infrastructure is up-to-date.")
	}
}
