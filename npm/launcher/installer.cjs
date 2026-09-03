"use strict";

const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const readline = require("node:readline/promises");

const PACKAGE_NAME = "@cogfoundry/loomloom";
const SKILL_NAME = "loomloom";
const BUNDLED_SKILL_PATH = path.join(__dirname, "..", "skill", SKILL_NAME);

function parseInstallArgs(argv) {
  const result = { agent: "", yes: false };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === "--yes") {
      result.yes = true;
      continue;
    }
    if (argument === "--agent") {
      const agent = argv[index + 1];
      if (!agent || agent.startsWith("-")) {
        throw new Error("--agent requires a skills CLI agent id, such as codex");
      }
      result.agent = agent;
      index += 1;
      continue;
    }
    throw new Error(`unsupported install option: ${argument}`);
  }
  return result;
}

function bundledSkillPath(source = BUNDLED_SKILL_PATH) {
  const skillFile = path.join(source, "SKILL.md");
  if (!fs.existsSync(skillFile)) {
    throw new Error(`npm package does not contain bundled ${SKILL_NAME} Skill at ${source}`);
  }
  return source;
}

function globalPackageSpec(version) {
  return `${PACKAGE_NAME}@${version}`;
}

function skillsAddArgs(source, agent) {
  return ["--yes", "skills", "add", source, "--skill", SKILL_NAME, "--global", "--agent", agent, "--yes", "--copy"];
}

function skillsListArgs(agent) {
  return ["--yes", "skills", "ls", "--global", "--agent", agent, "--json"];
}

function runCommand(command, args, options = {}) {
  const result = spawnSync(command, args, {
    encoding: "utf8",
    env: options.env ?? process.env,
    stdio: options.capture ? ["ignore", "pipe", "pipe"] : "inherit",
    shell: process.platform === "win32" && /\.(cmd|bat)$/i.test(command),
  });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    const details = `${result.stdout ?? ""}\n${result.stderr ?? ""}`.trim();
    throw new Error(`${command} ${args.join(" ")} failed with exit ${result.status}${details ? `: ${details}` : ""}`);
  }
  return result;
}

function skillInstalled(listJSON) {
  let entries;
  try {
    entries = JSON.parse(listJSON);
  } catch (error) {
    throw new Error(`skills list returned invalid JSON: ${error.message}`);
  }
  return Array.isArray(entries) && entries.some((entry) => entry?.name === SKILL_NAME && typeof entry.path === "string");
}

async function promptForAgent(prompt = {}) {
  const input = prompt.input ?? process.stdin;
  const output = prompt.output ?? process.stdout;
  if (!input.isTTY) {
    throw new Error("--agent is required when stdin is not interactive; for example: --agent codex");
  }
  const terminal = readline.createInterface({ input, output });
  try {
    const agent = (await terminal.question("Target Agent id (for example codex): ")).trim();
    if (!agent) {
      throw new Error("an Agent id is required");
    }
    return agent;
  } finally {
    terminal.close();
  }
}

function retryCommand(source, agent) {
  return `npx ${skillsAddArgs(source, agent).map((argument) => JSON.stringify(argument)).join(" ")}`;
}

async function install(argv, options = {}) {
  const parsed = parseInstallArgs(argv);
  const version = options.packageVersion ?? require("../package.json").version;
  const source = bundledSkillPath(options.bundledSkillPath);
  const agent = parsed.agent || await (options.promptForAgent ?? promptForAgent)();
  const execute = options.runCommand ?? runCommand;

  execute("npm", ["install", "--global", globalPackageSpec(version)]);
  try {
    execute("npx", skillsAddArgs(source, agent));
    const listed = execute("npx", skillsListArgs(agent), { capture: true });
    if (!skillInstalled(listed.stdout)) {
      throw new Error(`skills manager did not report ${SKILL_NAME} for agent ${agent}`);
    }
  } catch (error) {
    throw new Error(
      `CLI ${version} is installed, but the ${SKILL_NAME} Skill was not installed for ${agent}. ` +
      `Retry: ${retryCommand(source, agent)}. Cause: ${error.message}`,
    );
  }

  const output = options.output ?? process.stdout;
  output.write(`Installed LoomLoom CLI ${version} and Skill ${SKILL_NAME} for ${agent}.\n`);
  return { agent, source, version };
}

module.exports = {
  PACKAGE_NAME,
  SKILL_NAME,
  BUNDLED_SKILL_PATH,
  bundledSkillPath,
  globalPackageSpec,
  install,
  parseInstallArgs,
  retryCommand,
  skillInstalled,
  skillsAddArgs,
  skillsListArgs,
};
