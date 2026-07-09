package skill

import (
	"strings"
	"testing"
)

func TestSkillNameUsesLoomLoomPrefix(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		fallback    string
		want        string
	}{
		{
			name:        "display name",
			displayName: "Xiaohongshu Note Generator Standard",
			fallback:    "listing-1",
			want:        "loomloom-xiaohongshu-note-generator-standard",
		},
		{
			name:        "already prefixed",
			displayName: "loomloom Existing Skill",
			fallback:    "listing-1",
			want:        "loomloom-existing-skill",
		},
		{
			name:        "fallback",
			displayName: "!!!",
			fallback:    "template-1-version-1",
			want:        "loomloom-template-1-version-1",
		},
		{
			name:        "empty",
			displayName: "",
			fallback:    "",
			want:        "loomloom-skill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SkillName(tt.displayName, tt.fallback); got != tt.want {
				t.Fatalf("SkillName()=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestSkillNameKeepsMaxLength(t *testing.T) {
	got := SkillName("This is a very long template name that should be truncated safely for local agent skill naming", "")
	if len(got) > 63 {
		t.Fatalf("len(SkillName())=%d, want <=63: %q", len(got), got)
	}
	if !strings.HasPrefix(got, "loomloom-") {
		t.Fatalf("SkillName()=%q, want loomloom prefix", got)
	}
}
