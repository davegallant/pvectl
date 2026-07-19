#!/usr/bin/env bash
set -euo pipefail

# Types each command out (fake keystrokes) then runs it. Meant to be run
# inside `asciinema rec -c ./scripts/demo.sh` so takes are reproducible
# instead of hand-typed live.

PROMPT="\033[1;32m$\033[0m "

prompt() { printf '%b' "$PROMPT"; }

type_cmd() {
  local cmd=$1
  sleep 2
  for ((i = 0; i < ${#cmd}; i++)); do
    printf '%s' "${cmd:$i:1}"
    sleep 0.03
  done
  printf '\n'
  sleep 0.4
  eval "$cmd" || true
  prompt
}

type_tab_complete() {
  local words=$1
  local completion=$2
  local partial="pvectl $words "

  sleep 2
  for ((i = 0; i < ${#partial}; i++)); do
    printf '%s' "${partial:$i:1}"
    sleep 0.03
  done
  sleep 0.6

  local candidates
  candidates=$(pvectl __complete $words "" 2>/dev/null | grep -v '^:')
  local list_output
  list_output=$(printf '%s\n' "$candidates" | column)
  local n_lines
  n_lines=$(printf '%s\n' "$list_output" | wc -l)

  printf '\n%s\n' "$list_output"
  sleep 1
  printf '\033[%dA\r\033[J' "$((n_lines + 1))"
  prompt
  printf '%s' "$partial"

  local prefix=""
  for ((i = 1; i <= ${#completion}; i++)); do
    prefix=${completion:0:i}
    if [ "$(grep -c "^${prefix}" <<<"$candidates")" -le 1 ]; then
      break
    fi
  done

  for ((i = 0; i < ${#prefix}; i++)); do
    printf '%s' "${prefix:$i:1}"
    sleep 0.05
  done
  sleep 0.5
  printf '%s\n' "${completion:${#prefix}}"
  sleep 0.4
  eval "${partial}${completion}" || true
  prompt
}

clear
prompt
type_cmd "pvectl status"
type_cmd "pvectl nodes"
type_tab_complete "ct start" "gyb"
type_cmd "pvectl ct stop gyb"
type_cmd "pvectl ct snapshots create gyb --snapshot-name pvectl-demo"
type_cmd "pvectl ct snapshots list gyb"
type_cmd "pvectl ct snapshots delete gyb --snapshot-name pvectl-demo -y"
