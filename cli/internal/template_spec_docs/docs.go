package templatespecdocs

import (
	"embed"
	"fmt"
)

// FS contains the TemplateSpec documentation snapshot shipped with the CLI.
//
//go:embed template-spec/*.md
var FS embed.FS

type Topic struct {
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	Description string `json:"description"`
}

var topics = []Topic{
	{Name: "spec", Filename: "00-template-spec.md", Description: "TemplateSpec fields, step relations, port contracts, and capability registry"},
	{Name: "authoring", Filename: "01-authoring-guide.md", Description: "Authoring workflow and common validation failures"},
	{Name: "examples", Filename: "02-examples-and-patterns.md", Description: "Recommended patterns for single step, linear chain, and step-level fan-in"},
	{Name: "conversation", Filename: "03-conversational-authoring.md", Description: "Conversational authoring protocol for agent-generated TemplateSpec workflows"},
}

func Topics() []Topic {
	out := append([]Topic(nil), topics...)
	return out
}

func Read(topicName string) (Topic, string, error) {
	for _, topic := range topics {
		if topic.Name != topicName {
			continue
		}
		content, err := FS.ReadFile("template-spec/" + topic.Filename)
		if err != nil {
			return Topic{}, "", fmt.Errorf("read TemplateSpec docs %q: %w", topicName, err)
		}
		return topic, string(content), nil
	}
	return Topic{}, "", fmt.Errorf("unknown TemplateSpec docs topic %q", topicName)
}
