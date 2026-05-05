param(
  [int]$Port = 18765
)

$ErrorActionPreference = "Stop"

$AppName = "CursorPro3 x64"
$ExePath = Join-Path $PSScriptRoot "CursorPro.exe"
$BundleMarker = "com.yuxin.CursorPr3"
$SourceTokenDir = Join-Path $env:APPDATA "NVIDIA_NV\codex_tokens"
$StateRoot = Join-Path $env:APPDATA "CursorPro3"
$ExportDir = Join-Path $StateRoot "exports\codex"
$LogDir = Join-Path $StateRoot "logs"
$StateFile = Join-Path $StateRoot "control_state.json"
$PidFile = Join-Path $StateRoot "control_server.pid"
$LogFile = Join-Path $LogDir "control_server.log"
$WorkerScript = Join-Path $PSScriptRoot "cursorpro3_register_worker.ps1"

New-Item -ItemType Directory -Force -Path $ExportDir | Out-Null
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

function NowIso {
  return [DateTimeOffset]::UtcNow.ToString("o")
}

function Write-Log([string]$Message) {
  Add-Content -LiteralPath $LogFile -Value ("[{0}] {1}" -f (NowIso), $Message)
}

function Save-State([hashtable]$State) {
  $tmp = "$StateFile.tmp"
  ($State | ConvertTo-Json -Depth 8) | Set-Content -LiteralPath $tmp -Encoding UTF8
  Move-Item -Force -LiteralPath $tmp -Destination $StateFile
}

$State = @{
  task_id = $null
  status = "idle"
  started_at = $null
  finished_at = $null
  created_count = 0
  updated_count = 0
  error_code = $null
  error_message = $null
  last_export_at = $null
  last_export_count = 0
}

if (Test-Path $StateFile) {
  try {
    $loaded = Get-Content -LiteralPath $StateFile -Raw | ConvertFrom-Json -AsHashtable
    foreach ($key in $loaded.Keys) { $State[$key] = $loaded[$key] }
  } catch {
    Write-Log "failed to load state: $($_.Exception.Message)"
  }
}

function Get-TokenFiles {
  if (-not (Test-Path $SourceTokenDir)) { return @() }
  return Get-ChildItem -LiteralPath $SourceTokenDir -Filter *.json | Sort-Object Name
}

function Export-Tokens {
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
  $State.last_export_at = NowIso
  $State.last_export_count = $count
  Save-State $State
  Write-Log "exported $count token files"
  return @{
    exported_count = $count
    exported_at = $State.last_export_at
  }
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

function Start-RegisterTask {
  if ($State.status -eq "running") {
    return @{
      statusCode = 409
      body = $State
    }
  }

  $State.task_id = [guid]::NewGuid().ToString()
  $State.status = "running"
  $State.started_at = NowIso
  $State.finished_at = $null
  $State.created_count = 0
  $State.updated_count = 0
  $State.error_code = $null
  $State.error_message = $null
  Save-State $State

  Start-Process powershell -WindowStyle Hidden -ArgumentList @(
    "-ExecutionPolicy", "Bypass",
    "-File", $WorkerScript,
    "-ExePath", $ExePath,
    "-SourceTokenDir", $SourceTokenDir,
    "-ExportDir", $ExportDir,
    "-StateFile", $StateFile,
    "-LogFile", $LogFile
  ) | Out-Null

  return @{
    statusCode = 202
    body = $State
  }
}

function Get-Health {
  $running = @(Get-Process | Where-Object { $_.Path -eq $ExePath }).Count -gt 0
  return @{
    ok = $true
    app_name = $AppName
    app_running = $running
    source_token_dir = $SourceTokenDir
    source_token_dir_exists = (Test-Path $SourceTokenDir)
    export_dir = $ExportDir
    export_dir_exists = (Test-Path $ExportDir)
    state = $State
  }
}

[int]$pid = $PID
Set-Content -LiteralPath $PidFile -Value $pid -Encoding UTF8
Export-Tokens | Out-Null

$listener = [System.Net.HttpListener]::new()
$listener.Prefixes.Add("http://127.0.0.1:$Port/")
$listener.Start()
Write-Log "control server listening on http://127.0.0.1:$Port"

try {
  while ($listener.IsListening) {
    $context = $listener.GetContext()
    $request = $context.Request
    $response = $context.Response

    $payload = $null
    $statusCode = 200

    switch ("$($request.HttpMethod) $($request.Url.AbsolutePath)") {
      "GET /v1/health" {
        $payload = Get-Health
      }
      "GET /v1/register/status" {
        $payload = $State
      }
      "GET /v1/tokens" {
        $items = @()
        foreach ($file in Get-TokenFiles) {
          try {
            $raw = Get-Content -LiteralPath $file.FullName -Raw | ConvertFrom-Json -AsHashtable
          } catch { continue }
          $items += @{
            filename = $file.Name
            provider = if ($raw.type) { $raw.type } else { "codex" }
            account_id = $raw.account_id
            email = $raw.email
            token_type = "oauth_token_bundle"
            expires_at = $raw.expired
            source = "cursorpro3"
            status = "new"
            updated_at = $file.LastWriteTimeUtc.ToString("o")
          }
        }
        $payload = @{
          items = $items
          count = $items.Count
        }
      }
      "POST /v1/tokens/export" {
        $payload = Export-Tokens
      }
      "POST /v1/register/trigger" {
        $result = Start-RegisterTask
        $statusCode = $result.statusCode
        $payload = $result.body
      }
      default {
        $statusCode = 404
        $payload = @{ error = "not_found" }
      }
    }

    $json = [Text.Encoding]::UTF8.GetBytes(($payload | ConvertTo-Json -Depth 8))
    $response.StatusCode = $statusCode
    $response.ContentType = "application/json; charset=utf-8"
    $response.ContentLength64 = $json.Length
    $response.OutputStream.Write($json, 0, $json.Length)
    $response.OutputStream.Close()
    Write-Log "$($request.HttpMethod) $($request.Url.AbsolutePath) -> $statusCode"
  }
} finally {
  $listener.Stop()
  Remove-Item -Force -ErrorAction SilentlyContinue $PidFile
}
