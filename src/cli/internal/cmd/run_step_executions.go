package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

type runStepExecution struct {
	ExecutionID      string                  `json:"executionId"`
	TaskID           string                  `json:"taskId"`
	SourceRowIndex   int                     `json:"sourceRowIndex"`
	StepID           string                  `json:"stepId"`
	LoopID           string                  `json:"loopId,omitempty"`
	Iteration        int                     `json:"iteration,omitempty"`
	ExecutionIndex   int                     `json:"executionIndex"`
	RuntimeKind      string                  `json:"runtimeKind"`
	Status           string                  `json:"status"`
	StartedAtUnix    int64                   `json:"startedAtUnix,omitempty"`
	FinishedAtUnix   int64                   `json:"finishedAtUnix,omitempty"`
	Input            json.RawMessage         `json:"input,omitempty"`
	InputUnavailable string                  `json:"inputUnavailable,omitempty"`
	Values           []runStepExecutionValue `json:"values"`
	Artifacts        []json.RawMessage       `json:"artifacts"`
	ErrorCode        string                  `json:"errorCode,omitempty"`
	ErrorMessage     string                  `json:"errorMessage,omitempty"`
	Code             *runStepExecutionCode   `json:"code,omitempty"`
	Model            *runStepExecutionModel  `json:"model,omitempty"`
}

type runStepExecutionValue struct {
	PortID string          `json:"portId"`
	Value  json.RawMessage `json:"value"`
}

type runStepExecutionCode struct {
	Attempts   []json.RawMessage `json:"attempts"`
	Diagnostic json.RawMessage   `json:"diagnostic,omitempty"`
}

type runStepExecutionModel struct {
	ModelName        string `json:"modelName"`
	PromptTokens     int64  `json:"promptTokens"`
	CompletionTokens int64  `json:"completionTokens"`
	Cost             int64  `json:"cost"`
}

type runLoopIteration struct {
	TaskID         string          `json:"taskId"`
	LoopID         string          `json:"loopId"`
	Iteration      int             `json:"iteration"`
	Status         string          `json:"status"`
	State          json.RawMessage `json:"state,omitempty"`
	StateOmitted   bool            `json:"stateOmitted,omitempty"`
	DecisionKind   string          `json:"decisionKind,omitempty"`
	DecisionStepID string          `json:"decisionStepId,omitempty"`
	DecisionPortID string          `json:"decisionPortId,omitempty"`
	DecisionAction string          `json:"decisionAction,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	StartedAtUnix  int64           `json:"startedAtUnix,omitempty"`
	FinishedAtUnix int64           `json:"finishedAtUnix,omitempty"`
}

type listRunStepExecutionsResponse struct {
	Items          []runStepExecution `json:"items"`
	LoopIterations []runLoopIteration `json:"loopIterations"`
	NextPageToken  string             `json:"nextPageToken,omitempty"`
	TotalCount     int                `json:"totalCount"`
}

func newRunStepExecutionsCmd(opts *rootOptions) *cobra.Command {
	var (
		taskID    string
		pageSize  int
		pageToken string
	)

	cmd := &cobra.Command{
		Use:   "step-executions <run-id>",
		Short: "List every node execution of your own template's run: inputs, outputs, errors and Loop rounds",
		Long: "Lists each node execution of a run you started from a template you own: the input it received, " +
			"its outputs, errors, Code attempts with traceback and output tails, model call usage, and every Loop round " +
			"with its state and decision. Market runs and other authors' templates are rejected. Use -o json for full detail.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := newHTTPClient(opts)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			query := url.Values{}
			if strings.TrimSpace(taskID) != "" {
				query.Set("taskId", strings.TrimSpace(taskID))
			}
			if pageSize > 0 {
				query.Set("pageSize", fmt.Sprintf("%d", pageSize))
			}
			if strings.TrimSpace(pageToken) != "" {
				query.Set("pageToken", strings.TrimSpace(pageToken))
			}
			var resp listRunStepExecutionsResponse
			path := "/users/me/runs/" + strings.TrimSpace(args[0]) + "/stepExecutions"
			if err := httpClient.GetJSONWithQuery(ctx, path, query, &resp); err != nil {
				return err
			}
			if opts.output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(resp)
			}
			return writeRunStepExecutions(cmd, resp)
		},
	}
	cmd.Flags().StringVar(&taskID, "task-id", "", "Only list executions of this task")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Executions per page, server clamps to its maximum")
	cmd.Flags().StringVar(&pageToken, "page-token", "", "Pagination token returned by the previous call")
	return cmd
}

func writeRunStepExecutions(cmd *cobra.Command, resp listRunStepExecutionsResponse) error {
	tw := newTabWriter(cmd.OutOrStdout())
	if _, err := fmt.Fprintln(tw, "row\tstep\tloop\tkind\tstatus\tattempts\tmodel\toutputs\terror\tinput"); err != nil {
		return err
	}
	for _, item := range resp.Items {
		loop := "-"
		if item.LoopID != "" {
			loop = fmt.Sprintf("%s#%d", item.LoopID, item.Iteration)
		}
		attempts, model := "-", "-"
		if item.Code != nil {
			attempts = fmt.Sprintf("%d", len(item.Code.Attempts))
		}
		if item.Model != nil {
			model = fmt.Sprintf("%s tokens=%d/%d cost=%d", item.Model.ModelName, item.Model.PromptTokens, item.Model.CompletionTokens, item.Model.Cost)
		}
		outputs := make([]string, 0, len(item.Values))
		for _, value := range item.Values {
			outputs = append(outputs, value.PortID+"="+compactRunJSON(value.Value))
		}
		if len(item.Artifacts) > 0 {
			outputs = append(outputs, fmt.Sprintf("files=%d", len(item.Artifacts)))
		}
		errorText := strings.TrimSpace(item.ErrorCode + " " + item.ErrorMessage)
		input := compactRunJSON(item.Input)
		if input == "" {
			input = "(" + item.InputUnavailable + ")"
		}
		if _, err := fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", item.SourceRowIndex, item.StepID, loop, item.RuntimeKind, item.Status,
			attempts, oneLine(model), truncateRunText(strings.Join(outputs, " | "), 120), truncateRunText(errorText, 160), truncateRunText(input, 120)); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(resp.LoopIterations) > 0 {
		tw = newTabWriter(cmd.OutOrStdout())
		if _, err := fmt.Fprintln(tw, "\nloop\tround\tstatus\tdecision\treason\tstate"); err != nil {
			return err
		}
		for _, round := range resp.LoopIterations {
			decision := "-"
			if round.DecisionAction != "" {
				decision = fmt.Sprintf("%s %s/%s=%s", round.DecisionKind, round.DecisionStepID, round.DecisionPortID, round.DecisionAction)
			}
			state := compactRunJSON(round.State)
			if round.StateOmitted {
				state = "(over 256 KiB, not stored)"
			}
			if _, err := fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\n", round.LoopID, round.Iteration, round.Status, decision, truncateRunText(round.Reason, 120), truncateRunText(state, 120)); err != nil {
				return err
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "total_count\t%d\nnext_page_token\t%s\n", resp.TotalCount, resp.NextPageToken)
	return err
}

func compactRunJSON(raw json.RawMessage) string {
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return string(raw)
	}
	return compact.String()
}

func truncateRunText(value string, limit int) string {
	value = oneLine(value)
	if len(value) > limit {
		return value[:limit-3] + "..."
	}
	return value
}
