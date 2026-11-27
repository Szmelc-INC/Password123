# Agent prompts for the team key register

Copy-paste prompts for AI coding agents (Claude Code, Codex, Grok, Qwen and others) that work on Password123. Replace every `<…>` before you send a prompt. Each prompt is self-contained, and the agent should not need to explore beyond the files it names.

The operator (the human who runs the agent) stays responsible for the agent's key. Agents never get the `maintainer` role.

---

## Prompt 1: enroll yourself (agent onboarding)

```text
You are joining the Password123 team as an agent member. Goal: create your own
official keypairs, publish ONLY the public parts in team/members/<handle>/, and
open one PR. Repository: Szmelc-INC/Password123. Read team/README.md first
(the "Rules" and "Joining the team" sections only).

Your identity:
  handle:   <agent-handle>          (e.g. claude-sx; ^[a-z0-9][a-z0-9-]{1,31}$)
  operator: <human-handle>          (must be, or become, a registered human member)
  email:    <noreply address used for your commits, or leave out>

Steps:
1. git switch -c team/enroll-<agent-handle> origin/main
2. scripts/team.sh keygen <agent-handle> --kind agent --operator <human-handle> \
     --no-passphrase [--email <email>] --expires-days 180
   Private keys go to ~/.config/password123/team/<agent-handle>/ (mode 600).
3. scripts/team.sh verify   -> must end with "team register valid".
4. Configure signing for THIS checkout only (team/INTEGRATION.md section 1),
   then: git add team && git commit -S -m "team: enroll <agent-handle>"
   Check it with: git verify-commit HEAD
5. If the branch conflicts with main: git merge origin/main, then
   scripts/team.sh update && scripts/team.sh verify, commit (never rebase,
   never hand-edit generated files).
6. Push the branch and open a PR titled "team: enroll <agent-handle>". In the
   body, list the ssh_fingerprint and the age_recipient from member.yaml, and
   state that the operator must confirm them out of band.

Hard rules (a violation ends the task):
- NEVER print, cat, copy, paste, upload, log or commit any file without a .pub
  suffix from ~/.config/password123/ (age.key, ssh_ed25519). Do not read them.
  Only scripts/team.sh and ssh-keygen/age may open them.
- Never put keys anywhere inside the repository, /tmp, or a shared directory.
- The PR changes only team/members/<agent-handle>/, team/allowed_signers,
  team/recipients.txt and team/SHA256SUMS. Do not touch recipients/,
  .sops.yaml or payloads/: payload access is a maintainer step.
- Do not use --roles maintainer.
- If keygen reports that keys already exist, stop and report. Do not delete
  or overwrite them.

Report (max 10 lines): handle, ssh_fingerprint, age_recipient, PR URL, CI status.
```

---

## Prompt 2: integrate your key into your environment

Use this after the enrollment PR is merged.

```text
Configure your Password123 checkout to use your registered team keys. Handle:
<handle>. Keys: ~/.config/password123/team/<handle>/. Follow team/INTEGRATION.md
sections 1 and 2 exactly.

1. Run git config --local for gpg.format ssh, user.signingkey (the .pub file),
   commit.gpgsign true, tag.gpgsign true,
   gpg.ssh.allowedSignersFile team/allowed_signers.
   Set user.email to the email: value in team/members/<handle>/member.yaml.
2. Make an empty signed commit on a scratch branch, run git verify-commit HEAD,
   then delete the scratch branch. It must say Good "git" signature for <handle>.
3. Sign and verify a test file in your job's temporary directory:
     scripts/team.sh sign-file <tmpfile> --handle <handle>
     scripts/team.sh verify-file <tmpfile> --handle <handle>
4. Run age -R team/members/<handle>/age.pub on a test string, then
   age -d -i ~/.config/password123/team/<handle>/age.key (use
   go run filippo.io/age/cmd/age@<version in go.mod> if age is missing), and
   confirm the round trip.
Never print the private key files. Report: which checks passed, the git config
lines you set, and anything that failed with its exact error.
```

---

## Prompt 3: review an enrollment PR (maintainer-side agent)

```text
Review Password123 PR #<n> ("team: enroll <handle>"). You prepare the
review. A human maintainer approves the out-of-band fingerprint check.

Check, and report each item as PASS or FAIL:
1. gh pr diff <n> --name-only shows ONLY team/members/<handle>/{member.yaml,
   member.yaml.sig,age.pub,ssh.pub}, team/allowed_signers,
   team/recipients.txt and team/SHA256SUMS (an existing-member rotation may
   also replace that member's files).
2. Check out the PR (gh pr checkout <n>) and run scripts/team.sh verify --strict.
   It must report "self-signed" for <handle>. The only allowed warning is the
   stale SHA256SUMS.sig.
3. member.yaml: kind and operator are plausible. An agent's operator is a
   registered human. roles is member, unless a maintainer explicitly approved
   maintainer for a human. expires is no more than 365 days after enrolled.
4. No private key material: p123 validate --skip-vectors and gitleaks pass in CI.
5. CI is green on every job.
Then print, for the human maintainer, the ssh_fingerprint and the age_recipient
to confirm with <handle> or its operator over a second channel.
Do NOT merge until the human confirms the fingerprints. After merge, open a
follow-up PR from a maintainer machine that runs
scripts/team.sh sign-sums --handle <maintainer>. If payload access was
requested, follow the CONTRIBUTING.md enrollment runbook. That needs the
maintainer's age identity, which is a human step.
```

---

## Prompt 4: periodic register audit

```text
Audit the Password123 team register on main. Run scripts/team.sh verify --strict
and scripts/team.sh list. Report: members whose key expires within 30 days,
expired or revoked members still listed in recipients/maintainers.txt or
.sops.yaml (compare with team/members/*/age.pub), certificates expiring within
30 days, and whether team/SHA256SUMS.sig verifies. Open an issue with the
findings. Change nothing.
```
