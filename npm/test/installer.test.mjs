import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const installer = require("../launcher/installer.cjs");

test("installer pins the Skill source and global CLI to one package version", () => {
  assert.equal(
    installer.skillSourceURL("1.2.3-beta.4"),
    "https://github.com/cogfoundry-labs/loomloom/tree/v1.2.3-beta.4/agent-guidance/loomloom",
  );
  assert.equal(installer.globalPackageSpec("1.2.3"), "@cogfoundry/loomloom@1.2.3");
  assert.deepEqual(installer.skillsAddArgs("https://example.invalid/skill", "codex"), [
    "--yes", "skills", "add", "https://example.invalid/skill", "--skill", "loomloom", "--global", "--agent", "codex", "--yes",
  ]);
});

test("installer requires an explicit Agent without a terminal", async () => {
  await assert.rejects(
    () => installer.install([], { packageVersion: "1.2.3", promptForAgent: async () => { throw new Error("--agent is required when stdin is not interactive"); } }),
    /--agent is required/,
  );
});

test("installer verifies the selected Agent Skill after installing", async () => {
  const calls = [];
  const output = [];
  const result = await installer.install(["--agent", "codex", "--yes"], {
    packageVersion: "1.2.3",
    runCommand(command, args, options) {
      calls.push({ command, args, options });
      if (command === "npx" && args.includes("ls")) {
        return { stdout: JSON.stringify([{ name: "loomloom", path: "/tmp/skills/loomloom" }]) };
      }
      return { stdout: "" };
    },
    output: { write(value) { output.push(value); } },
  });
  assert.equal(result.agent, "codex");
  assert.equal(calls[0].command, "npm");
  assert.deepEqual(calls[0].args, ["install", "--global", "@cogfoundry/loomloom@1.2.3"]);
  assert.equal(calls[1].command, "npx");
  assert.equal(calls[2].command, "npx");
  assert.match(output.join(""), /Skill loomloom for codex/);
});

test("installer fails when skills reports success without the selected Skill", async () => {
  await assert.rejects(
    () => installer.install(["--agent", "codex"], {
      packageVersion: "1.2.3",
      runCommand(command, args) {
        if (command === "npx" && args.includes("ls")) return { stdout: "[]" };
        return { stdout: "" };
      },
    }),
    /Skill was not installed for codex.*Retry:/,
  );
});
