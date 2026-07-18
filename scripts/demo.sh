#!/usr/bin/env bash
set -euo pipefail

# Types each command out (fake keystrokes) then runs it. Meant to be run
# inside `asciinema rec -c ./scripts/demo.sh` so takes are reproducible
# instead of hand-typed live.

PROMPT="\033[1;32m$\033[0m "

prompt() { printf '%b' "$PROMPT"; }

# The prompt for the *next* command is printed right after the current
# command's output, then this function pauses for a beat before typing —
# matching how a real shell returns to the prompt immediately, with the
# human pause happening while the prompt is already on screen. Printing the
# prompt right before typing (with no preceding pause) makes the prompt and
# the first keystroke land in the same instant, which reads as uncanny.
type_cmd() {
  local cmd=$1
  sleep 2
  for ((i = 0; i < ${#cmd}; i++)); do
    printf '%s' "${cmd:$i:1}"
    sleep 0.03
  done
  printf '\n'
  sleep 0.4
  # `|| true`: a real command failing (e.g. a Proxmox-side limitation like a
  # storage backend that doesn't support snapshots) shouldn't kill the rest
  # of the recording under `set -e`.
  eval "$cmd" || true
  prompt
}

clear
prompt
type_cmd "pvectl status"
type_cmd "pvectl nodes"
type_cmd "pvectl ct list"
type_cmd "pvectl qm list"
type_cmd "pvectl ct start gyb"
type_cmd "pvectl ct stop gyb"
type_cmd "pvectl ct snapshots create gyb --snapshot-name test"
type_cmd "pvectl ct snapshots delete gyb --snapshot-name test -y"
type_cmd "pvectl storage"
