# S394 probe (throwaway): each entry with its own log. Phase a: no places of one's own; phase c: parts alone;
# phase b: the three and the icons' directory. Then the start of the server, Starts times: floe stop, then a
# floe publish that starts it (ms each).
# -UserHome: the USERPROFILE to use, made anew (a real machine: not the user's own); none: the runner's own.
param([string]$UserHome = '', [int]$Starts = 10, [string]$Exe = '')
$ErrorActionPreference = 'Continue'
$F = if ($Exe) { $Exe } else { Join-Path $PWD 'floe-probe.exe' }
$W = 'C:\s394'
Remove-Item -Recurse -Force $W -ErrorAction SilentlyContinue
New-Item -ItemType Directory "$W\logs", "$W\work" | Out-Null
if ($UserHome) {
  Remove-Item -Recurse -Force $UserHome -ErrorAction SilentlyContinue
  New-Item -ItemType Directory $UserHome | Out-Null
  $env:USERPROFILE = $UserHome
}
foreach ($k in 'XDG_DATA_HOME', 'XDG_CONFIG_HOME', 'FLOE_DIR', 'FLOE_PARTS_DIR', 'FLOE_THEMES_DIR', 'FLOE_TEMPLATES_DIR', 'FLOE_ICONS_DIR') {
  Remove-Item "Env:$k" -ErrorAction SilentlyContinue
}
$env:FLOE_PORT = '7391'
Set-Location "$W\work"
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

function Run([string]$name, [string[]]$a) {
  $env:FLOE_PROBE_LOG = "$W\logs\$name.log"
  $sw = [Diagnostics.Stopwatch]::StartNew()
  & $F @a > $null 2> "$W\logs\$name.err"
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
Entries 'a'
Run 'c-new-part' @('part', 'new', 'p1')
Entries 'c'
Run 'b-new-theme' @('theme', 'new', 't1')
Run 'b-new-template' @('template', 'new', 'tp1')
New-Item -ItemType Directory -Force "$env:USERPROFILE\.local\share\floe-icons" | Out-Null
Entries 'b'
# The start, without the probe's log (its writes would be timed too).
$ms = @()
for ($i = 0; $i -lt $Starts; $i++) {
  & $F stop > $null 2>&1
  $sw = [Diagnostics.Stopwatch]::StartNew()
  & $F publish page.yaml > $null 2>&1
  $ms += $sw.ElapsedMilliseconds
}
& $F stop > $null 2>&1
$sorted = $ms | Sort-Object
"starts ms: $($ms -join ' ')"
"starts median=$($sorted[[int][math]::Floor($sorted.Count / 2)]) max=$($sorted[-1])"
Get-ChildItem "$W\logs\*.err" | Where-Object Length -gt 0 | ForEach-Object { "== $($_.Name)"; Get-Content $_ }
