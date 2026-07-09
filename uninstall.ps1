param(
  [string]$Agent = "codex",
  [string]$InstallDir = "$HOME\AppData\Local\Programs\loomloom",
  [string]$SkillDir = "",
  [switch]$CliOnly,
  [switch]$SkillOnly
)

$ErrorActionPreference = "Stop"

function Resolve-SkillDir {
  param([string]$AgentName, [string]$Override)
  if ($Override) { return $Override }
  switch ($AgentName) {
    "codex" { return "$HOME\.codex\skills\loomloom" }
    "claude" { return "$HOME\.claude\skills\loomloom" }
    "openclaw" { return "$HOME\.openclaw\workspace\skills\loomloom" }
    default { throw "unsupported agent: $AgentName" }
  }
}

$removeCli = $true
$removeSkill = $true
if ($CliOnly) {
  $removeSkill = $false
}
if ($SkillOnly) {
  $removeCli = $false
}

$removedAny = $false

function Uninstall-HomebrewCli {
  $brewCmd = Get-Command brew -ErrorAction SilentlyContinue
  if (-not $brewCmd) { return $false }

  & $brewCmd.Source list --versions loomloom *> $null
  if ($LASTEXITCODE -ne 0) { return $false }

  & $brewCmd.Source uninstall loomloom
  Write-Host "removed Homebrew formula: loomloom"
  return $true
}

if ($removeCli) {
  if (Uninstall-HomebrewCli) {
    $removedAny = $true
  }
  $cliPath = Join-Path $InstallDir "loomloom.exe"
  if (Test-Path -LiteralPath $cliPath) {
    Remove-Item -LiteralPath $cliPath -Force
    Write-Host "removed: $cliPath"
    $removedAny = $true
  } else {
    Write-Host "not found: $cliPath"
  }
}

if ($removeSkill) {
  $finalSkillDir = Resolve-SkillDir -AgentName $Agent -Override $SkillDir
  $skillPath = Join-Path $finalSkillDir "SKILL.md"
  if (Test-Path -LiteralPath $skillPath) {
    Remove-Item -LiteralPath $skillPath -Force
    if (Test-Path -LiteralPath $finalSkillDir) {
      try {
        Remove-Item -LiteralPath $finalSkillDir -Force
      } catch {
      }
    }
    Write-Host "removed: $skillPath"
    $removedAny = $true
  } else {
    Write-Host "not found: $skillPath"
  }
}

if (-not $removedAny) {
  Write-Host "nothing removed"
}
