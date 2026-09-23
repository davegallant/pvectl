# Roadmap

Proposed priorities from a repository review on 2026-09-22. This is a
direction for development, not a release schedule or a promise to implement
every item. Findings below come from source inspection; live-cluster behavior
and release infrastructure were not exercised during this review.

## Current position

pvectl already covers guest lifecycle operations, cloning, resizing,
snapshots, backups and restore, migration, configuration viewing/editing,
console access, guest execution, tags, cloud-init creation, template/ISO
downloads, cluster status, and node reboot. JSON output, command schema
introspection, and raw API access provide a useful automation foundation.

Keep the direct REST API architecture, explicit subcommands, narrow secret
store interface, and deliberate separation between CT and QEMU operations.
Existing regression tests capture valuable real-world Proxmox differences.
The next milestone should make existing workflows more dependable.

## Now: reliability and automation correctness

- [ ] **Distinguish interrupted waits from successful operations.**
  Both wait paths in [cmd/progress.go](cmd/progress.go) return `nil` on
  cancellation. Scripts consequently receive success without a confirmed
  result. More seriously, [QM creation](cmd/qm_create.go) can proceed to
  its start step after an interrupted create wait. Return a distinct
  interruption result, preserve the UPID, and prevent subsequent workflow
  steps. Cancelling a wait must continue to leave the server task running.
  Verify interruption in both terminal and piped modes, including create
  with `--start`.

- [ ] **Bound failures while monitoring a task.**
  [cmd/progress.go](cmd/progress.go) silently retries every polling error
  indefinitely. The 30-second HTTP timeout bounds a request, not the wait:
  persistent authentication failures or an unavailable node can strand a
  script forever. Surface polling errors, bound consecutive failures, and
  offer an explicit wait timeout. Preserve the last error and UPID on exit;
  never retry the original mutation automatically. Test transient recovery,
  persistent failure, and deadline expiry.

- [ ] **Make output modes predictable.**
  [Task watch](cmd/tasks.go) always emits terminal escape sequences and
  refresh text, even with `-o json`; [status watch](cmd/status.go) also
  always renders terminal controls. Reject unsupported combinations or
  define a documented streaming format. The global output flag is also
  accepted by actions whose [progress renderer](cmd/progress.go) emits
  plain text. Either support structured action results (including UPID,
  outcome, and warnings) or reject JSON where unsupported. Keep diagnostics
  on stderr. Validate actual CLI stdout, not just rendering helpers.

- [ ] **Write credentials atomically and enforce file permissions.**
  [FileStore](internal/secrets/file.go) and
  [config.Save](internal/config/config.go) overwrite files directly with
  `os.WriteFile(..., 0600)`. Interrupted writes can leave truncated files;
  the supplied mode does not repair permissions on an existing file.
  Write a private temporary file in the same directory and replace the
  destination atomically, with platform-appropriate permission handling.
  Test replacing a permissive existing file and failed writes without
  touching the real keychain.

## Next: complete common workflows

- [ ] **Add `tasks status`, `tasks logs`, and `tasks wait`.**
  [The task command group](cmd/tasks.go) currently exposes only `list`.
  Make the UPID printed after interruption directly usable to inspect a
  result, retrieve diagnostics, or resume waiting. Reuse the corrected
  wait implementation. Audit log pagination: [TaskLog](internal/api/tasks.go)
  currently makes one request without pagination parameters despite its
  full-log contract; confirm endpoint behavior and test multi-page logs.

- [ ] **Add explicit scripted configuration updates.**
  Consider `ct/qm config set` and `config unset` for regular API fields,
  with digest protection and a clear change summary. Today users must use
  an editor or assemble raw API calls; deleting fields in the editor is
  deliberately unsupported. Keep raw `lxc.*` lines outside this feature.
  Define and test Proxmox field-removal behavior before implementation.

- [ ] **Make unattended creation explicit.**
  Add a non-interactive mode that reports all missing inputs before making
  changes, while preserving existing prompts for human use. An omitted
  optional ISO or post-create start choice should have documented behavior
  in this mode. Test complete commands with closed stdin and incomplete
  commands that must fail before sending a create request.

- [ ] **Provide a read-only diagnostic command.**
  A `doctor` command could report config location/backend, API connectivity,
  TLS validation, and effective permissions for common operations, with
  remediation hints. Setup's version check cannot establish resource
  permissions. Never print secrets or change ACLs automatically; report
  denied diagnostic checks without interpreting an empty list as proof
  that the cluster has no guests.

## Later: additions driven by actual use

- [ ] **Complete the cloud-image workflow.**
  Cloud-init flags already exist, but importing and attaching a bootable
  cloud disk still needs additional steps. Evaluate a focused disk import/
  attach command or an end-to-end documented recipe before expanding
  `qm create`. Confirm supported API paths and storage requirements first;
  label any unavoidable SSH operation consistently.

- [ ] **Document recovery drills.**
  Provide tested examples for restoring to a new VMID, restoring after a
  node loss, handling storage mappings, and inspecting task failures.
  Exercise these manually against disposable guests; ordinary CI must
  remain independent of a live cluster.

## Maintenance alongside those milestones

- [ ] **Reconcile project guidance with shipped behavior.**
  [AGENTS.md](AGENTS.md) still calls packaging automation a non-goal even
  though GoReleaser and Homebrew configuration exist, describes consoles
  as SSH-only despite API console support, and describes `qm config` as
  only containing `edit` despite `view` existing. Update historical notes
  without losing the reasons behind remaining deliberate limitations.
- [ ] **Check generated documentation in CI.** Regenerate `docs/cli/`
  and fail on drift. Keep hand-written examples aligned with the command
  tree; do not hand-edit generated reference pages.
- [ ] **Harden the existing installer.**
  [scripts/install.sh](scripts/install.sh) streams an archive directly
  into extraction without checking the release's `checksums.txt`.
  Download first, verify its checksum, then install. Allow a pinned release
  and document `INSTALL_DIR` for user-local installs. Test checksum failure
  and unsupported platforms. Checksums detect corruption; they do not
  independently authenticate a compromised release source.
- [ ] **Gate publishing on validation.**
  [Release CI](.github/workflows/release.yml) and validation are separate
  workflows, so publishing does not depend on tests/lint succeeding.
  Require validation before release publication. Add platform smoke tests
  for the Linux, macOS, and Windows artifacts already configured for build,
  and document which console/keychain combinations are actually tested.
- [ ] **Carry cancellation through command execution.** Gradually replace
  command-level `context.Background()` calls with a shared command context,
  especially in watches and multi-step operations. Verify that cancellation
  stops pending client work without claiming to cancel a server task.

## Scope boundaries

No full-screen picker, multi-cluster manager, generic node/storage config
editor, broad CT/QEMU abstraction rewrite, or speculative filtering/bulk
operations. Existing raw API access remains the escape hatch for uncommon
endpoints. Add features when they remove a concrete repeated workflow cost.

For each implemented item: add regression coverage appropriate to the risk,
run `just check`, regenerate CLI docs when commands/flags change, and add
the required changelog entry. Changes involving Proxmox-specific behavior
also need documented live validation or an explicit unverified limitation.
