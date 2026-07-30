param(
  [string]$Agent = "codex",
  [string]$InstallDir = "$HOME\AppData\Local\Programs\loomloom",
  [string]$SkillDir = "",
  [switch]$CliOnly,
  [switch]$SkillOnly
)

$ErrorActionPreference = "Stop"

if ($CliOnly -and $SkillOnly) {
  throw "cli-only and skill-only cannot be used together"
}

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
} elseif ($SkillOnly) {
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

function Add-ConfigTokenEnvironmentName {
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

function Add-CurrentTokenEnvironmentName {
  Get-ChildItem Env: | ForEach-Object {
    Add-TokenEnvironmentName -Name $_.Name
  }
}

function Write-TokenEnvironmentNameReport {
  if ($tokenEnvNames.Count -eq 0) { return }
  foreach ($name in @($tokenEnvNames) | Sort-Object) {
    Write-Host "environment token cleanup required: $name"
  }
  Write-Host "Agent action required: ask the user for confirmation before removing these variables from their permanent environment configuration."
}

function Test-PathEqual {
  param([string]$Left, [string]$Right)
  $isMacOSValue = Get-Variable -Name IsMacOS -ValueOnly -ErrorAction SilentlyContinue
  $comparison = if ($env:OS -eq "Windows_NT" -or $isMacOSValue) {
    [System.StringComparison]::OrdinalIgnoreCase
  } else {
    [System.StringComparison]::Ordinal
  }
  return [string]::Equals($Left, $Right, $comparison)
}

function Resolve-CanonicalExistingDirectory {
  param([string]$Path)

  $fullPath = [System.IO.Path]::GetFullPath($Path)
  $root = [System.IO.Path]::GetPathRoot($fullPath)
  $current = $root
  $relative = $fullPath.Substring($root.Length)
  $separators = [char[]]@([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar)

  foreach ($part in $relative.Split($separators, [System.StringSplitOptions]::RemoveEmptyEntries)) {
    $current = Join-Path $current $part
    $item = Get-Item -LiteralPath $current -Force -ErrorAction Stop
    if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
      throw "refusing to remove unsafe Skill directory: path contains a symbolic link or reparse point: $current"
    }
    $current = $item.FullName
  }

  $resolved = [System.IO.Path]::GetFullPath($current)
  if (-not (Test-Path -LiteralPath $resolved -PathType Container)) {
    throw "target is not a directory: $Path"
  }
  return $resolved
}

function Test-LoomLoomSkillFrontmatter {
  param([string]$SkillFile)

  $lines = @(Get-Content -LiteralPath $SkillFile)
  if ($lines.Count -eq 0 -or $lines[0] -cne "---") { return $false }

  $nameCount = 0
  for ($index = 1; $index -lt $lines.Count; $index++) {
    $line = [string]$lines[$index]
    if ($line -cmatch '^---\s*$') {
      return $nameCount -eq 1
    }
    if ($line -cmatch '^\s*name\s*:\s*(.*?)\s*$') {
      $value = $Matches[1]
      if ($value -cnotin @("loomloom", '"loomloom"', "'loomloom'")) { return $false }
      $nameCount++
    }
  }
  return $false
}

function Assert-SafeSkillDirectory {
  param([string]$Path)

  $normalizedPath = [System.IO.Path]::GetFullPath($Path)
  $root = [System.IO.Path]::GetPathRoot($normalizedPath)
  if (-not (Test-PathEqual -Left $normalizedPath -Right $root)) {
    $normalizedPath = $normalizedPath.TrimEnd(
      [System.IO.Path]::DirectorySeparatorChar,
      [System.IO.Path]::AltDirectorySeparatorChar
    )
  }

  $targetItem = Get-Item -LiteralPath $normalizedPath -Force -ErrorAction Stop
  if (-not $targetItem.PSIsContainer) {
    throw "refusing to remove unsafe Skill directory: target is not a directory: $Path"
  }
  if (($targetItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
    throw "refusing to remove unsafe Skill directory: target is a symbolic link or reparse point: $Path"
  }

  $canonicalPath = Resolve-CanonicalExistingDirectory -Path $normalizedPath
  $root = [System.IO.Path]::GetPathRoot($canonicalPath)
  if (Test-PathEqual -Left $canonicalPath -Right $root) {
    throw "refusing to remove unsafe Skill directory: dangerous path: $canonicalPath"
  }

  $dangerousPaths = @(
    $HOME,
    (Join-Path (Join-Path $HOME ".codex") "skills"),
    (Join-Path (Join-Path $HOME ".claude") "skills"),
    (Join-Path (Join-Path (Join-Path $HOME ".openclaw") "workspace") "skills")
  )
  foreach ($dangerousPath in $dangerousPaths) {
    if (Test-Path -LiteralPath $dangerousPath -PathType Container) {
      $canonicalDangerousPath = Resolve-CanonicalExistingDirectory -Path $dangerousPath
      if (Test-PathEqual -Left $canonicalPath -Right $canonicalDangerousPath) {
        throw "refusing to remove unsafe Skill directory: dangerous path: $canonicalPath"
      }
    }
  }

  $skillFile = Join-Path $canonicalPath "SKILL.md"
  $referencesDir = Join-Path $canonicalPath "references"
  if (-not (Test-Path -LiteralPath $skillFile -PathType Leaf)) {
    throw "refusing to remove unsafe Skill directory: SKILL.md is missing or is not a regular file"
  }
  $skillItem = Get-Item -LiteralPath $skillFile -Force
  if (($skillItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
    throw "refusing to remove unsafe Skill directory: SKILL.md is a symbolic link or reparse point"
  }
  if (-not (Test-LoomLoomSkillFrontmatter -SkillFile $skillFile)) {
    throw "refusing to remove unsafe Skill directory: SKILL.md frontmatter must contain exactly 'name: loomloom'"
  }
  if (-not (Test-Path -LiteralPath $referencesDir -PathType Container)) {
    throw "refusing to remove unsafe Skill directory: references is missing or is not a directory"
  }
  $referencesItem = Get-Item -LiteralPath $referencesDir -Force
  if (($referencesItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
    throw "refusing to remove unsafe Skill directory: references is a symbolic link or reparse point"
  }

  foreach ($entry in @(Get-ChildItem -LiteralPath $canonicalPath -Force)) {
    if ($entry.Name -cnotin @("SKILL.md", "references")) {
      throw "refusing to remove unsafe Skill directory: unexpected top-level entry: $($entry.Name)"
    }
  }

  $skillText = Get-Content -LiteralPath $skillFile -Raw
  foreach ($entry in @(Get-ChildItem -LiteralPath $referencesDir -Force)) {
    if ($entry.PSIsContainer -or
        (($entry.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0)) {
      throw "refusing to remove unsafe Skill directory: unexpected reference entry: $($entry.Name)"
    }
    if (-not ($skillText.Contains("](references/$($entry.Name))") -or
        $skillText.Contains("](references/$($entry.Name)#"))) {
      throw "refusing to remove unsafe Skill directory: reference is not explicitly referenced by SKILL.md: references/$($entry.Name)"
    }
  }

  return $canonicalPath
}

$configFile = $null
$finalSkillDir = $null
$skillDirPresent = $false
if ($removeSkill) {
  $requestedSkillDir = Resolve-SkillDir -AgentName $Agent -Override $SkillDir
  if (Test-Path -LiteralPath $requestedSkillDir) {
    $finalSkillDir = Assert-SafeSkillDirectory -Path $requestedSkillDir
    $skillDirPresent = $true
  } else {
    $finalSkillDir = $requestedSkillDir
  }
}
if ($removeConfig) {
  $configFile = Resolve-ConfigFile
  Add-ConfigTokenEnvironmentName -ConfigFile $configFile
  Add-CurrentTokenEnvironmentName
}

function Uninstall-HomebrewCli {
  $brewCmd = Get-Command brew -ErrorAction SilentlyContinue
  if (-not $brewCmd) { return $false }

  & $brewCmd.Source list --versions loomloom *> $null
  if ($LASTEXITCODE -ne 0) { return $false }

  & $brewCmd.Source uninstall loomloom
  if ($LASTEXITCODE -ne 0) {
    throw "failed to uninstall Homebrew formula: loomloom"
  }
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
  if ($skillDirPresent) {
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

Write-TokenEnvironmentNameReport
