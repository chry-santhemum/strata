package strata

import (
	"fmt"
	"io"
	"strings"
)

var focusPlanQuestions = []string{
	"How long do you plan to work on this?",
	"What are the immediate next actions?",
	"What concrete outputs do you expect by the end?",
}

func focusPlanAnswers(plan *FocusPlan) []string {
	if plan == nil {
		return []string{"", "", ""}
	}
	return []string{
		plan.PlannedDuration,
		plan.ImmediateNextActions,
		plan.ExpectedOutputs,
	}
}

func focusPlanFromAnswers(answers []string) FocusPlan {
	plan := FocusPlan{}
	if len(answers) > 0 {
		plan.PlannedDuration = answers[0]
	}
	if len(answers) > 1 {
		plan.ImmediateNextActions = answers[1]
	}
	if len(answers) > 2 {
		plan.ExpectedOutputs = answers[2]
	}
	return plan
}

func writePlanBlock(w io.Writer, plan *FocusPlan) {
	fmt.Fprintln(w, "Plan")
	answers := focusPlanAnswers(plan)
	for i, question := range focusPlanQuestions {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%d. %s\n", i+1, question)
		answer := strings.TrimSpace(answers[i])
		if answer == "" {
			fmt.Fprintln(w, "(no answer)")
			continue
		}
		fmt.Fprintln(w, answer)
	}
}
