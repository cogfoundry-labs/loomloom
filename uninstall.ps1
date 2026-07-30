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
$removeConfig = $true
if ($CliOnly) {
  $removeSkill = $false
  $removeConfig = $false
}
if ($SkillOnly) {
  $removeCli = $false
  $removeConfig = $false
}

$removedAny = $false
$tokenEnvNames = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)

function Add-TokenEnvironmentName {
  param([string]$Name)
  if ($Name -match '^LOOMLOOM_TOKEN(_[A-Z0-9_]+)?$') {
    [void]$tokenEnvNames.Add($Name)
  }
}

function Resolve-ConfigFile {
  $configRoot = if ($env:APPDATA) { $env:APPDATA } else { Join-Path $HOME "AppData\Roaming" }
  return Join-Path (Join-Path $configRoot "loomloom") "config.json"
}

function Collect-ConfigTokenEnvironmentNames {
  param([string]$ConfigFile)
  if (-not (Test-Path -LiteralPath $ConfigFile -PathType Leaf)) { return }
  try {
    $state = Get-Content -LiteralPath $ConfigFile -Raw | ConvertFrom-Json
    foreach ($server in @($state.servers)) {
      Add-TokenEnvironmentName -Name ([string]$server.token_env)
    }
  } catch {
    Write-Warning "could not inspect token environment variable names in $ConfigFile"
  }
}

function Collect-CurrentTokenEnvironmentNames {
  Get-ChildItem Env: | ForEach-Object {
    Add-TokenEnvironmentName -Name $_.Name
  }
}

function Report-TokenEnvironmentNames {
  if ($tokenEnvNames.Count -eq 0) { return }
  foreach ($name in @($tokenEnvNames) | Sort-Object) {
    Write-Host "environment token cleanup required: $name"
  }
  Write-Host "Agent action required: ask the user for confirmation before removing these variables from their permanent environment configuration."
}

$configFile = $null
if ($removeConfig) {
  $configFile = Resolve-ConfigFile
  Collect-ConfigTokenEnvironmentNames -ConfigFile $configFile
  Collect-CurrentTokenEnvironmentNames
}

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
  if (Test-Path -LiteralPath $finalSkillDir) {
    Remove-Item -LiteralPath $finalSkillDir -Recurse -Force
    Write-Host "removed: $finalSkillDir"
    $removedAny = $true
  } else {
    Write-Host "not found: $finalSkillDir"
  }
}

if ($removeConfig) {
  if (Test-Path -LiteralPath $configFile -PathType Leaf) {
    Remove-Item -LiteralPath $configFile -Force
    $configDir = Split-Path -Parent $configFile
    if ((Test-Path -LiteralPath $configDir -PathType Container) -and
        -not (Get-ChildItem -LiteralPath $configDir -Force | Select-Object -First 1)) {
      Remove-Item -LiteralPath $configDir -Force
    }
    Write-Host "removed: $configFile"
    $removedAny = $true
  } else {
    Write-Host "not found: $configFile"
  }
}

if (-not $removedAny) {
  Write-Host "nothing removed"
}

Report-TokenEnvironmentNames
