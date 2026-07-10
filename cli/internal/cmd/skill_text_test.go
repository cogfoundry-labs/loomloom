package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledSkillsUseDoctorPlatformFacts(t *testing.T) {
	root := findRepoRoot(t)
	for _, rel := range []string{
		"skills/codex/loomloom/SKILL.md",
		"skills/claude/loomloom/SKILL.md",
		"skills/openclaw/loomloom/SKILL.md",
	} {
		t.Run(rel, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				t.Fatalf("ReadFile error=%v", err)
			}
			text := string(data)
			for _, want := range []string{
				"loomloom doctor --output json",
				"credential_action",
				"你还没有完整配置 LoomLoom Server 和密钥。请选择要使用的平台：",
				"https://loomloom.shengsuanyun.com/loom/v1",
				"https://console.shengsuanyun.com/user/keys",
				"当前未检测到胜算云密钥。请前往胜算云控制台创建或复制密钥后配置到本地环境：",
				"当前胜算云账户余额不足，请前往胜算云控制台充值后再继续：",
				"https://console.shengsuanyun.com/user/recharge",
				"在 CogFoundry 计费功能上线前，请使用胜算云控制台创建 API 密钥",
				"CogFoundry 面向新加坡及其他海外地区用户，当前支付和交易能力仍在建设中，敬请期待。当前阶段请继续使用胜算云。",
			} {
				if !strings.Contains(text, want) {
					t.Fatalf("%s missing %q", rel, want)
				}
			}
			for _, forbidden := range []string{
				"https://cogfoundry.ai",
				"https://console-dev.cogfoundry",
				"https://console.cogfoundry",
			} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s should not include CogFoundry console URL %q", rel, forbidden)
				}
			}
		})
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error=%v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "skills")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root with skills directory not found")
		}
		dir = parent
	}
}
