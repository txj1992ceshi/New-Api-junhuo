param(
  [string]$ExePath,
  [string]$SourceTokenDir,
  [string]$ExportDir,
  [string]$StateFile,
  [string]$LogFile
)

$ErrorActionPreference = "Stop"

function NowIso {
  return [DateTimeOffset]::UtcNow.ToString("o")
}

function Write-Log([string]$Message) {
  $logDir = Split-Path -Parent $LogFile
  New-Item -ItemType Directory -Force -Path $logDir | Out-Null
  Add-Content -LiteralPath $LogFile -Value ("[{0}] {1}" -f (NowIso), $Message)
}

function Load-State {
  if (Test-Path $StateFile) {
    return Get-Content -LiteralPath $StateFile -Raw | ConvertFrom-Json -AsHashtable
  }
  return @{}
}

function Save-State([hashtable]$State) {
  $tmp = "$StateFile.tmp"
  ($State | ConvertTo-Json -Depth 8) | Set-Content -LiteralPath $tmp -Encoding UTF8
  Move-Item -Force -LiteralPath $tmp -Destination $StateFile
}

function Get-TokenFiles {
  if (-not (Test-Path $SourceTokenDir)) { return @() }
  return Get-ChildItem -LiteralPath $SourceTokenDir -Filter *.json | Sort-Object Name
}

function Export-Tokens {
  New-Item -ItemType Directory -Force -Path $ExportDir | Out-Null
  $count = 0
  foreach ($file in Get-TokenFiles) {
    try {
      $raw = Get-Content -LiteralPath $file.FullName -Raw | ConvertFrom-Json -AsHashtable
    } catch {
      Write-Log "failed to parse token file $($file.Name): $($_.Exception.Message)"
      continue
    }
    $payload = @{
      filename = $file.Name
      provider = if ($raw.type) { $raw.type } else { "codex" }
      account_id = $raw.account_id
      email = $raw.email
      token_type = "oauth_token_bundle"
      expires_at = $raw.expired
      source = "cursorpro3"
      status = "new"
      updated_at = $file.LastWriteTimeUtc.ToString("o")
      exported_at = (NowIso)
      raw = @{
        access_token = $raw.access_token
        refresh_token = $raw.refresh_token
        id_token = $raw.id_token
      }
    }
    $tmp = Join-Path $ExportDir ($file.BaseName + ".tmp")
    $out = Join-Path $ExportDir $file.Name
    ($payload | ConvertTo-Json -Depth 8) | Set-Content -LiteralPath $tmp -Encoding UTF8
    Move-Item -Force -LiteralPath $tmp -Destination $out
    $count++
  }
  return $count
}

function Get-Snapshot {
  $map = @{}
  foreach ($file in Get-TokenFiles) {
    $map[$file.Name] = @($file.Length, $file.LastWriteTimeUtc.Ticks)
  }
  return $map
}

function Compare-Snapshots($Before, $After) {
  $created = 0
  $updated = 0
  foreach ($name in $After.Keys) {
    if (-not $Before.ContainsKey($name)) {
      $created++
    } elseif (($Before[$name][0] -ne $After[$name][0]) -or ($Before[$name][1] -ne $After[$name][1])) {
      $updated++
    }
  }
  return @($created, $updated)
}

function Start-AppIfNeeded {
  $proc = Get-Process | Where-Object { $_.Path -eq $ExePath } | Select-Object -First 1
  if (-not $proc) {
    Start-Process -FilePath $ExePath | Out-Null
    Start-Sleep -Seconds 3
  }
}

function Invoke-OneClickRegister {
  Add-Type -AssemblyName UIAutomationClient
  Add-Type -AssemblyName UIAutomationTypes
  $proc = Get-Process | Where-Object { $_.Path -eq $ExePath } | Select-Object -First 1
  if (-not $proc) { throw "process not running" }
  $root = [System.Windows.Automation.AutomationElement]::FromHandle($proc.MainWindowHandle)
  if (-not $root) { throw "failed to get automation root" }
  $condition = New-Object System.Windows.Automation.PropertyCondition(
    [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
    [System.Windows.Automation.ControlType]::Button
  )
  $buttons = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $condition)
  for ($i = 0; $i -lt $buttons.Count; $i++) {
    $btn = $buttons.Item($i)
    if ($btn.Current.Name -eq "一键换号") {
      $pattern = $btn.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
      $pattern.Invoke()
      return
    }
  }
  throw "button 一键换号 not found"
}

$state = Load-State
$before = Get-Snapshot

try {
  Start-AppIfNeeded
  Invoke-OneClickRegister
  Write-Log "triggered one-click register via UI automation"
  $deadline = (Get-Date).AddMinutes(5)
  do {
    Start-Sleep -Seconds 2
    $after = Get-Snapshot
    $counts = Compare-Snapshots $before $after
    if ($counts[0] -gt 0 -or $counts[1] -gt 0) {
      $exported = Export-Tokens
      $state.status = "succeeded"
      $state.finished_at = NowIso
      $state.created_count = $counts[0]
      $state.updated_count = $counts[1]
      $state.last_export_at = NowIso
      $state.last_export_count = $exported
      $state.error_code = $null
      $state.error_message = $null
      Save-State $state
      exit 0
    }
  } while ((Get-Date) -lt $deadline)

  $exported = Export-Tokens
  $state.status = "failed"
  $state.finished_at = NowIso
  $state.last_export_at = NowIso
  $state.last_export_count = $exported
  $state.error_code = "register_timeout"
  $state.error_message = "No token file changes were detected before timeout."
  Save-State $state
  exit 1
} catch {
  $state.status = "failed"
  $state.finished_at = NowIso
  $state.error_code = "register_trigger_failed"
  $state.error_message = $_.Exception.Message
  Save-State $state
  Write-Log "register task failed: $($_.Exception.Message)"
  exit 1
}
