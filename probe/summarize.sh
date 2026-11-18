#!/usr/bin/env bash
# S394 probe: per log, per process (c: the command, s: a server it started), count the calls by kind and by the
# first floe frame above files; the server's split into its start (serverConfig, first read) and its looks.
cd "$1" || exit 1
for f in *.log; do
  awk -F'\t' -v name="${f%.log}" '
    NR==1 {cmd=$1}
    {
      proc = ($1==cmd) ? "c" : "s"
      n = split($4, c, " < ")
      g = c[1]
      if (g ~ /^kit\./) g = "kit-read"
      if (g ~ /^store\./ || g ~ /^server\.OpenLog/) g = "store"
      if (g ~ /marks/) g = "marks"
      if (proc == "s" && $4 ~ /look < server\.\(\*Server\)\.latest|look$|\.look </) g = g "@look"
      k = $2
      if (k == "allowed" || k == "allowed-last" || k == "allowedWritten") k = "allowed"
      cnt[name "\t" proc "\t" k "\t" g]++
    }
    END { for (x in cnt) print x "\t" cnt[x] }
  ' "$f"
done | sort
