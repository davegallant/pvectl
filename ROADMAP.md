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

- [x] **Distinguish interrupted waits from successful operations.**
  Both wait paths in [cmd/progress.go](cmd/progress.go) now return a
  cancellation error with the UPID. Scripts exit non-zero and multi-step
  commands such as [QM creation](cmd/qm_create.go) stop before their next
  action when task completion is unconfirmed. Cancelling a wait leaves the
  server task running. Covered for terminal and piped modes and VM creation
  with `--start`.

- [x] **Bound failures while monitoring a task.**
  [cmd/progress.go](cmd/progress.go) reports the first polling error,
  retries brief failures, and ends the wait after three consecutive errors
  with the last error and UPID. `--wait-timeout` provides an optional bound
  for the whole wait, separate from the 30-second per-request HTTP timeout.
  Neither path retries the original mutation. Covered for transient recovery,
  persistent failure, and deadline expiry.

- [x] **Make output modes predictable.**
  `-o json` now rejects commands that emit text, including mutating
  actions and `status`. `tasks list --watch` cannot be combined with JSON,
  and both watches require a terminal to prevent escape sequences in piped
  output. Supported list/summary/config reads and raw API calls retain
  JSON output. Errors are reported before any network call.

- [x] **Write credentials through private replacement files.**
  [FileStore](internal/secrets/file.go) and
  [config.Save](internal/config/config.go) now write a `0600` temporary
  file in the destination directory and rename it over the original after
  syncing it. This corrects a permissive existing file mode and leaves
  the original target untouched if replacement fails. Tests use temporary
  directories and never touch the real keychain. Atomic rename semantics
  can vary by platform and filesystem.

## Next: complete common workflows

- [x] **Add `tasks status`, `tasks logs`, and `tasks wait`.**
  A UPID printed after interruption can now be used to inspect a task,
  retrieve its log, or resume waiting. `tasks wait` reuses the corrected
  wait behavior, and [TaskLog](internal/api/tasks.go) pages through the
  endpoint's reported total so diagnostics include lines beyond the first
  page. The API behavior was checked against Proxmox documentation and
  developer discussion; live-cluster validation remains open.

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

- [x] **Reconcile project guidance with shipped behavior.**
  [AGENTS.md](AGENTS.md) now reflects release automation, API consoles,
  `qm config view`, and both CT and VM counts in `status`.
- [x] **Check generated documentation in CI.** The generator omits
  changing date footers; CI regenerates `docs/cli/` and fails on drift.
  Keep hand-written examples aligned with the command tree and do not
  hand-edit generated reference pages.
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
