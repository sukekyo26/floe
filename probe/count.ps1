#Requires -Version 7.4
# S394 probe (throwaway, never merged). Runs one floe build (floe-probe.exe, made with the probe's log) through the
# commands of three homes, counting the files checks, then times 20 starts of its server. Run it as
#   pwsh -NoProfile -File <this> -Exe <floe-probe.exe> -Out C:\s394-out\run-<v> -UserHome C:\s394-out\home-<v>
# It reads and writes only under C:\s394-out: the environment it sets is this process's and its children's, in a home
# of its own (USERPROFILE, HOME) and on port 7391, so the user's floe (their home, port 7325) is not touched.
param(
  [Parameter(Mandatory)] [string]$Exe,
  [Parameter(Mandatory)] [string]$Out,
  [Parameter(Mandatory)] [string]$UserHome,
  [int]$Starts = 20
)
$ErrorActionPreference = 'Stop'
foreach ($p in $Exe, $Out, $UserHome) {
  if (-not $p.StartsWith('C:\s394-out\')) { throw "$p is not under C:\s394-out\" }
}
foreach ($k in 'XDG_DATA_HOME', 'XDG_CONFIG_HOME', 'XDG_STATE_HOME', 'XDG_CACHE_HOME', 'FLOE_DIR', 'FLOE_PARTS_DIR',
  'FLOE_THEMES_DIR', 'FLOE_TEMPLATES_DIR', 'FLOE_ICONS_DIR', 'FLOE_PROBE_LOG') {
  Remove-Item "Env:$k" -ErrorAction SilentlyContinue
}
$env:USERPROFILE = $UserHome
$env:HOME = $UserHome
$env:FLOE_PORT = '7391'
$logs = Join-Path $Out 'logs'
$work = Join-Path $Out 'work'
foreach ($d in $Out, $UserHome) { if (Test-Path $d) { Remove-Item -Recurse -Force $d } }
New-Item -ItemType Directory $logs, $work, $UserHome | Out-Null

function Run([string]$name, [string[]]$a) {
  $env:FLOE_PROBE_LOG = "$logs\$name.log"
  $sw = [Diagnostics.Stopwatch]::StartNew()
  & $Exe @a > $null 2> "$logs\$name.err"
  "$name exit=$LASTEXITCODE ms=$($sw.ElapsedMilliseconds)"
  Remove-Item Env:FLOE_PROBE_LOG
}
function Entries([string]$p) {
  Run "$p-check" @('check', 'page.yaml')
  Run "$p-render" @('render', 'page.yaml')
  Run "$p-part-list" @('part', 'list')
  Run "$p-part-check" @('part', 'check')
  Run "$p-theme-list" @('theme', 'list')
  Run "$p-template-list" @('template', 'list')
  Run "$p-template-check" @('template', 'check')
  Run "$p-diagram-inspect" @('diagram', 'inspect', 'page.yaml')
  Run "$p-publish" @('publish', 'page.yaml')
  Run "$p-publish-again" @('publish', 'page.yaml')
  Run "$p-rebuild" @('rebuild')
  Run "$p-stop" @('stop')
}

Push-Location $work
try {
  git init -q .
  @'
template: report
lang: en
title: 'Probe'
sections:
  - heading: One
    parts:
      - text: 'Hello.'
'@ | Set-Content -NoNewline page.yaml
  # a: no places of one's own; c: parts alone; b: the three and the icons' directory.
  Entries 'a'
  Run 'c-new-part' @('part', 'new', 'p1')
  Entries 'c'
  Run 'b-new-theme' @('theme', 'new', 't1')
  Run 'b-new-template' @('template', 'new', 'tp1')
  New-Item -ItemType Directory -Force "$UserHome\.local\share\floe-icons" | Out-Null
  Entries 'b'
  # The start, without the probe's log: stop, then a publish that starts the server.
  $ms = @()
  for ($i = 0; $i -lt $Starts; $i++) {
    & $Exe stop *> $null
    $sw = [Diagnostics.Stopwatch]::StartNew()
    & $Exe publish page.yaml *> $null
    $ms += $sw.ElapsedMilliseconds
  }
  $sorted = $ms | Sort-Object
  "starts ms: $($ms -join ' ')"
  "starts median=$($sorted[[int][math]::Floor($sorted.Count / 2)]) max=$($sorted[-1])"
} finally {
  # The server this run started: this home, port 7391.
  & $Exe stop *> $null
  Pop-Location
  for ($i = 0; $i -lt 50 -and (Get-Process -Name floe-probe -ErrorAction SilentlyContinue); $i++) { Start-Sleep -Milliseconds 200 }
  $listening = @(Get-NetTCPConnection -LocalPort 7391 -State Listen -ErrorAction SilentlyContinue).Count
  $left = @(Get-Process -Name floe-probe -ErrorAction SilentlyContinue).Count
  "left: port 7391 listening=$listening, floe-probe processes=$left"
}

# Totals: per command, its process (c) or the server it started (s), the checks (allowed: allowed, allowedWritten,
# RealAbove) and Windows' Lstat on the way.
$tot = [ordered]@{}
foreach ($f in Get-ChildItem "$logs\*.log" | Sort-Object Name) {
  $cmd = $null
  foreach ($line in [IO.File]::ReadLines($f.FullName)) {
    $c = $line.Split("`t")
    if ($null -eq $cmd) { $cmd = $c[0] }
    $proc = if ($c[0] -eq $cmd) { 'c' } else { 's' }
    $kind = switch ($c[1]) { 'lstat' { 'lstat' } 'walk-realabove' { $null } default { 'allowed' } }
    if ($kind) { $key = "$($f.BaseName)`t$proc`t$kind"; $tot[$key] = 1 + $tot[$key] }
  }
}
'== totals'
foreach ($k in $tot.Keys) { "$k`t$($tot[$k])" }
Get-ChildItem "$logs\*.err" | Where-Object Length -GT 0 | ForEach-Object { "== $($_.Name)"; Get-Content $_ }
