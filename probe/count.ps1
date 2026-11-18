# S394 probe (throwaway): each entry with its own log; phase a without places of one's own, phase b with them.
# USERPROFILE is the runner's own (C:\Users\runneradmin), as a user's.
$ErrorActionPreference = 'Continue'
$F = Join-Path $PWD 'floe-probe.exe'
$W = 'C:\s394'
Remove-Item -Recurse -Force $W -ErrorAction SilentlyContinue
New-Item -ItemType Directory "$W\logs", "$W\work" | Out-Null
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
Run 'b-new-part' @('part', 'new', 'p1')
Run 'b-new-theme' @('theme', 'new', 't1')
Run 'b-new-template' @('template', 'new', 'tp1')
New-Item -ItemType Directory -Force "$env:USERPROFILE\.local\share\floe-icons" | Out-Null
Entries 'b'
Get-ChildItem "$W\logs\*.err" | Where-Object Length -gt 0 | ForEach-Object { "== $($_.Name)"; Get-Content $_ }
