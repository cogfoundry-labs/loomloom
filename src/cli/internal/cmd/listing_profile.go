package cmd

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// The code grammar is shared with Core; category membership belongs to the Server.
var marketCategoryCode = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

type marketPublicValue struct {
	TargetAudience string   `json:"targetAudience"`
	ProblemSolved  string   `json:"problemSolved"`
	Scenarios      []string `json:"scenarios"`
	NotSuitableFor string   `json:"notSuitableFor"`
}

type listingProfileFlags struct {
	category       string
	targetAudience string
	problemSolved  string
	notSuitableFor string
	updateSummary  string
	scenarios      []string
}

func (f *listingProfileFlags) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.category, "category", "", "Category code from market list (categoryOptions); effective after approval, omit to preserve")
	cmd.Flags().StringVar(&f.targetAudience, "target-audience", "", "Public target audience; pass an empty string to clear")
	cmd.Flags().StringVar(&f.problemSolved, "problem-solved", "", "Public problem this SkillBot solves; pass an empty string to clear")
	cmd.Flags().StringArrayVar(&f.scenarios, "scenario", nil, "Public use scenario; repeat for multiple scenarios, pass an empty string to clear all")
	cmd.Flags().StringVar(&f.notSuitableFor, "not-suitable-for", "", "Public limitations or unsuitable uses; pass an empty string to clear")
	cmd.Flags().StringVar(&f.updateSummary, "update-summary", "", "Public summary of this update")
}

func (f *listingProfileFlags) categoryValue(cmd *cobra.Command) (*string, error) {
	if !cmd.Flags().Changed("category") {
		return nil, nil
	}
	value := strings.TrimSpace(f.category)
	if !marketCategoryCode.MatchString(value) {
		return nil, fmt.Errorf("--category must be a category code matching [a-z][a-z0-9_]{0,31}; use market list (categoryOptions) for current options; empty values cannot clear a category")
	}
	return &value, nil
}

func (f *listingProfileFlags) valueChanged(cmd *cobra.Command) bool {
	for _, flag := range []string{"target-audience", "problem-solved", "scenario", "not-suitable-for"} {
		if cmd.Flags().Changed(flag) {
			return true
		}
	}
	return false
}

// Both APIs replace value as a whole. Merge explicit flags to preserve siblings.
func (f *listingProfileFlags) mergeValue(cmd *cobra.Command, current marketPublicValue) *marketPublicValue {
	if cmd.Flags().Changed("target-audience") {
		current.TargetAudience = strings.TrimSpace(f.targetAudience)
	}
	if cmd.Flags().Changed("problem-solved") {
		current.ProblemSolved = strings.TrimSpace(f.problemSolved)
	}
	if cmd.Flags().Changed("not-suitable-for") {
		current.NotSuitableFor = strings.TrimSpace(f.notSuitableFor)
	}
	if cmd.Flags().Changed("scenario") {
		current.Scenarios = []string{}
		for _, value := range f.scenarios {
			if value = strings.TrimSpace(value); value != "" {
				current.Scenarios = append(current.Scenarios, value)
			}
		}
	}
	if current.Scenarios == nil {
		current.Scenarios = []string{}
	}
	return &current
}

func printMarketValue(w io.Writer, value marketPublicValue) error {
	for _, row := range [][2]string{{"target_audience", value.TargetAudience}, {"problem_solved", value.ProblemSolved}, {"not_suitable_for", value.NotSuitableFor}} {
		if row[1] != "" {
			if _, err := fmt.Fprintf(w, "%s\t%s\n", row[0], oneLine(row[1])); err != nil {
				return err
			}
		}
	}
	if len(value.Scenarios) > 0 {
		_, err := fmt.Fprintf(w, "scenarios\t%s\n", displayStringList(value.Scenarios))
		return err
	}
	return nil
}

func optionalPinPosition(value *flexInt64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d", *value)
}
