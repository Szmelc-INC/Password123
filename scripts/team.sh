#!/usr/bin/env bash
# Password123 team key register: generate, register, verify and certify the
# PUBLIC keys of every team member (human or agent).
#
# Private keys are written only under $P123_TEAM_KEYS (default
# ${XDG_CONFIG_HOME:-$HOME/.config}/password123/team/<handle>) and never into
# the repository. See team/README.md and team/INTEGRATION.md.
set -euo pipefail

VERSION=1
NS_TEAM="password123-team"   # namespace for member.yaml proof-of-possession signatures
NS_FILE="password123-file"   # namespace for file and SHA256SUMS signatures
SIGNER_NAMESPACES="git,${NS_TEAM},${NS_FILE}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEAM="$ROOT/team"
MEMBERS="$TEAM/members"
KEYS_BASE="${P123_TEAM_KEYS:-${XDG_CONFIG_HOME:-$HOME/.config}/password123/team}"

# ---------- output ----------
for a in "$@"; do
	case "$a" in --no-color) P123_COLOR=never ;; --color=*) P123_COLOR="${a#--color=}" ;; esac
done
if [[ -t 1 && -z "${NO_COLOR:-}" && "${P123_COLOR:-auto}" != never ]] || [[ "${P123_COLOR:-auto}" == always ]]; then
	C_OK=$'\e[38;2;31;138;138m' C_ERR=$'\e[38;2;184;58;58m' C_WARN=$'\e[38;2;192;192;192m' C_DIM=$'\e[2m' C_B=$'\e[1m' C_0=$'\e[0m'
else
	C_OK="" C_ERR="" C_WARN="" C_DIM="" C_B="" C_0=""
fi
ERRORS=0 WARNINGS=0 STRICT=0
ok() { printf '%s  ok %s %s\n' "$C_OK" "$C_0" "$*"; }
warn() {
	WARNINGS=$((WARNINGS + 1))
	printf '%swarn %s %s\n' "$C_WARN" "$C_0" "$*"
	if ((STRICT)); then ERRORS=$((ERRORS + 1)); fi
}
err() { ERRORS=$((ERRORS + 1)); printf '%s err %s %s\n' "$C_ERR" "$C_0" "$*" >&2; }
die() { printf '%serror:%s %s\n' "$C_ERR" "$C_0" "$*" >&2; exit 1; }
info() { printf '%s%s%s\n' "$C_DIM" "$*" "$C_0"; }

usage() {
	cat <<EOF
${C_B}team.sh${C_0} - Password123 team key register (v$VERSION)

${C_B}USAGE${C_0}
  scripts/team.sh <command> [args]

${C_B}MEMBER COMMANDS${C_0}
  keygen <handle> [meta]      generate an age identity and an ed25519 SSH key
                              OUTSIDE the repo, then run 'register'
  register <handle> [meta]    copy the public keys into team/members/<handle>/,
                              write member.yaml and sign it (proof of possession)
  sign-file <file> --handle H sign a file with your SSH key (writes <file>.sig)
  verify-file <file> --handle H
                              verify <file>.sig against team/allowed_signers

${C_B}REGISTER COMMANDS${C_0}
  update                      regenerate allowed_signers, recipients.txt, SHA256SUMS
  verify [--strict]           check every record, signature, certificate and checksum
  list                        aligned table of members

${C_B}MAINTAINER COMMANDS${C_0}
  sign-sums --handle H        sign team/SHA256SUMS with a maintainer key
  revoke <handle> --handle H  mark a member revoked, signed by maintainer H
  ca-init                     create the team SSH CA (private part stays outside the repo)
  issue-cert <handle> [--validity +52w]
                              sign the member's SSH key with the team CA (ssh-cert.pub)

${C_B}METADATA FLAGS${C_0} (keygen, register)
  --kind human|agent   --email ADDR   --github LOGIN   --operator HANDLE (agents)
  --roles member[,maintainer]   --expires-days N (default 365)
  --key-dir DIR (default \$P123_TEAM_KEYS/<handle>)   --no-passphrase (keygen only)

${C_B}ENVIRONMENT${C_0}
  P123_TEAM_KEYS  base directory for private keys (default ~/.config/password123/team)
  NO_COLOR, P123_COLOR=auto|always|never (or --no-color / --color=WHEN)
EOF
}

# ---------- helpers ----------
valid_handle() { [[ "$1" =~ ^[a-z0-9][a-z0-9-]{1,31}$ ]]; }
field() { sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed -e 's/[[:space:]]*$//' -e 's/^"\(.*\)"$/\1/'; }
today() { date -u +%F; }
ssh_fp() { ssh-keygen -l -E sha256 -f "$1" | awk '{print $2}'; }
key_dir_for() { printf '%s/%s' "$KEYS_BASE" "$1"; }

# Patterns are assembled at runtime so this file never matches its own scan.
private_pattern() {
	local a="AGE-SECRET" b="-KEY-" c="PRIVATE KEY-----"
	printf '%s' "${a}${b}(PQ-)?1[0-9A-Z]{20}|AGE-PLUGIN-[0-9A-Z-]+-1[0-9A-Z]{20}|-----BEGIN ([A-Z]+ )?${c}"
}

refuse_inside_repo() {
	local target
	target="$(realpath -m "$1")"
	case "$target/" in
	"$ROOT"/*) die "refusing to keep private keys inside the repository: $target" ;;
	esac
}

age_keygen() {
	if command -v age-keygen >/dev/null 2>&1; then
		age-keygen "$@"
	elif command -v go >/dev/null 2>&1; then
		local v
		v="$(awk '$1=="filippo.io/age"{print $2}' "$ROOT/go.mod")"
		(cd "$ROOT" && go run "filippo.io/age/cmd/age-keygen@${v:-v1.3.2}" "$@")
	else
		die "age-keygen not found (install age, or Go to run the pinned version)"
	fi
}

parse_meta() {
	KIND=human EMAIL="" GITHUB="" OPERATOR="" ROLES=member EXPIRES_DAYS=365 KEY_DIR="" NOPASS=0 HANDLE_FLAG=""
	POSITIONAL=()
	while (($#)); do
		case "$1" in
		--kind) KIND="${2:?}"; shift ;;
		--email) EMAIL="${2:?}"; shift ;;
		--github) GITHUB="${2:?}"; shift ;;
		--operator) OPERATOR="${2:?}"; shift ;;
		--roles) ROLES="${2:?}"; shift ;;
		--expires-days) EXPIRES_DAYS="${2:?}"; shift ;;
		--key-dir) KEY_DIR="${2:?}"; shift ;;
		--handle) HANDLE_FLAG="${2:?}"; shift ;;
		--no-passphrase) NOPASS=1 ;;
		--no-color | --color=*) ;;
		--strict) STRICT=1 ;;
		--validity) VALIDITY="${2:?}"; shift ;;
		-h | --help) usage; exit 0 ;;
		-*) die "unknown flag: $1" ;;
		*) POSITIONAL+=("$1") ;;
		esac
		shift
	done
	[[ "$KIND" =~ ^(human|agent)$ ]] || die "--kind must be human or agent"
	[[ "$ROLES" =~ ^(member|maintainer)(,(member|maintainer))*$ ]] || die "--roles: member and/or maintainer"
	[[ "$EXPIRES_DAYS" =~ ^[0-9]+$ ]] || die "--expires-days must be a number"
	[[ -z "$EMAIL" || "$EMAIL" =~ ^[^[:space:],\"]+@[^[:space:],\"]+$ ]] || die "--email looks invalid"
	[[ -z "$GITHUB" || "$GITHUB" =~ ^[A-Za-z0-9-]{1,39}$ ]] || die "--github looks invalid"
	[[ -z "$OPERATOR" ]] || valid_handle "$OPERATOR" || die "--operator must be a team handle"
	if [[ "$KIND" == agent && -z "$OPERATOR" ]]; then die "agents need --operator <human handle>"; fi
}

# ---------- keygen ----------
cmd_keygen() {
	parse_meta "$@"
	local h="${POSITIONAL[0]:-}"
	valid_handle "$h" || die "handle must match ^[a-z0-9][a-z0-9-]{1,31}\$"
	local dir="${KEY_DIR:-$(key_dir_for "$h")}"
	refuse_inside_repo "$dir"
	[[ ! -e "$dir/age.key" && ! -e "$dir/ssh_ed25519" ]] || die "keys already exist in $dir (refusing to overwrite)"
	umask 077
	mkdir -p "$dir"
	chmod 700 "$dir"
	age_keygen -o "$dir/age.key" 2>/dev/null
	age_keygen -y "$dir/age.key" >"$dir/age.pub"
	local pass=()
	if ((NOPASS)); then pass=(-N ""); fi
	ssh-keygen -q -t ed25519 -a 100 -C "${h}@password123" -f "$dir/ssh_ed25519" "${pass[@]}"
	chmod 600 "$dir/age.key" "$dir/ssh_ed25519"
	ok "private keys written to $dir (mode 600; back them up offline, never commit them)"
	register_from "$h" "$dir"
}

# ---------- register ----------
cmd_register() {
	parse_meta "$@"
	local h="${POSITIONAL[0]:-}"
	valid_handle "$h" || die "handle must match ^[a-z0-9][a-z0-9-]{1,31}\$"
	local dir="${KEY_DIR:-$(key_dir_for "$h")}"
	register_from "$h" "$dir"
}

register_from() {
	local h="$1" dir="$2"
	local agepub="$dir/age.pub" sshpub="$dir/ssh_ed25519.pub" sshkey="$dir/ssh_ed25519"
	[[ -f "$agepub" ]] || die "missing $agepub (run: age-keygen -y age.key > age.pub)"
	[[ -f "$sshpub" && -f "$sshkey" ]] || die "missing $sshkey(.pub)"
	local rec
	rec="$(grep -Eo '^age1[0-9a-z]{58}$' "$agepub" | head -n1)" || true
	[[ -n "$rec" ]] || die "$agepub: expected one classic age1 recipient (the team uses classic age keys only)"
	[[ "$(awk '{print $1}' "$sshpub")" == ssh-ed25519 ]] || die "$sshpub: expected an ssh-ed25519 key"
	local out="$MEMBERS/$h"
	if [[ -e "$out/member.yaml" ]]; then die "$out already exists; key rotation is a new PR that edits it by hand (see team/README.md)"; fi
	mkdir -p "$out"
	printf '%s\n' "$rec" >"$out/age.pub"
	awk '{print $1, $2, $3}' "$sshpub" | sed 's/ *$//' >"$out/ssh.pub"
	cat >"$out/member.yaml" <<EOF
# Password123 team member record. PUBLIC data only. Signed by the member's own
# SSH key (member.yaml.sig, namespace $NS_TEAM) as proof of possession.
schema: 1
handle: $h
kind: $KIND
email: "$EMAIL"
github: "$GITHUB"
operator: "$OPERATOR"
roles: $ROLES
age_recipient: $rec
ssh_fingerprint: $(ssh_fp "$out/ssh.pub")
enrolled: $(today)
expires: $(date -u -d "+${EXPIRES_DAYS} days" +%F)
status: active
EOF
	rm -f "$out/member.yaml.sig"
	ssh-keygen -q -Y sign -f "$sshkey" -n "$NS_TEAM" "$out/member.yaml" >/dev/null
	ok "registered $h in team/members/$h (public keys + signed member.yaml)"
	cmd_update
	info "next: scripts/team.sh verify && git add team && open a PR titled 'team: enroll $h'"
}

# ---------- generated files ----------
active_members() {
	local d
	[[ -d "$MEMBERS" ]] || return 0
	for d in "$MEMBERS"/*/; do
		[[ -f "$d/member.yaml" ]] || continue
		[[ "$(field "$d/member.yaml" status)" == active ]] || continue
		basename "$d"
	done | LC_ALL=C sort
}

gen_allowed_signers() {
	echo "# GENERATED by scripts/team.sh update. Do not edit. Format: ssh-keygen(1) ALLOWED SIGNERS."
	echo "# Principals: <handle>[,<email>]. Namespaces: $SIGNER_NAMESPACES."
	local h y principals
	while read -r h; do
		[[ -n "$h" ]] || continue
		y="$MEMBERS/$h/member.yaml"
		principals="$h"
		if [[ -n "$(field "$y" email)" ]]; then principals="$h,$(field "$y" email)"; fi
		printf '%s namespaces="%s" %s\n' "$principals" "$SIGNER_NAMESPACES" "$(awk '{print $1, $2}' "$MEMBERS/$h/ssh.pub")"
	done < <(active_members)
}

gen_recipients() {
	echo "# GENERATED by scripts/team.sh update. Do not edit."
	echo "# Age recipients of ACTIVE team members. These are NOT automatically enrolled:"
	echo "# payload access is still recipients/maintainers.txt (CONTRIBUTING.md runbook)."
	local h y
	while read -r h; do
		[[ -n "$h" ]] || continue
		y="$MEMBERS/$h/member.yaml"
		printf '%s # %s (%s), enrolled %s\n' "$(cat "$MEMBERS/$h/age.pub")" "$h" "$(field "$y" kind)" "$(field "$y" enrolled)"
	done < <(active_members)
}

gen_sums() {
	(
		cd "$ROOT"
		find team scripts/team.sh -type f ! -path team/SHA256SUMS ! -path team/SHA256SUMS.sig -print0 |
			LC_ALL=C sort -z | xargs -0 sha256sum
	)
}

cmd_update() {
	mkdir -p "$MEMBERS"
	gen_allowed_signers >"$TEAM/allowed_signers"
	gen_recipients >"$TEAM/recipients.txt"
	gen_sums >"$TEAM/SHA256SUMS"
	ok "regenerated team/allowed_signers, team/recipients.txt, team/SHA256SUMS"
	if [[ -f "$TEAM/SHA256SUMS.sig" ]]; then warn "team/SHA256SUMS changed; a maintainer must re-run sign-sums"; fi
}

# ---------- verify ----------
verify_member() {
	local h="$1" d="$MEMBERS/$1" y f
	y="$d/member.yaml"
	for f in member.yaml member.yaml.sig age.pub ssh.pub; do
		[[ -f "$d/$f" ]] || { err "$h: missing $f"; return; }
	done
	valid_handle "$h" || err "$h: directory name is not a valid handle"
	[[ "$(field "$y" schema)" == 1 ]] || err "$h: schema must be 1"
	[[ "$(field "$y" handle)" == "$h" ]] || err "$h: handle field does not match the directory"
	[[ "$(field "$y" kind)" =~ ^(human|agent)$ ]] || err "$h: kind must be human or agent"
	[[ "$(field "$y" roles)" =~ ^(member|maintainer)(,(member|maintainer))*$ ]] || err "$h: invalid roles"
	[[ "$(field "$y" status)" =~ ^(active|revoked)$ ]] || err "$h: status must be active or revoked"
	if [[ "$(field "$y" kind)" == agent ]]; then
		local op
		op="$(field "$y" operator)"
		if [[ -z "$op" ]]; then
			err "$h: agents must name an operator"
		elif [[ ! -f "$MEMBERS/$op/member.yaml" || "$(field "$MEMBERS/$op/member.yaml" kind)" != human ]]; then
			warn "$h: operator '$op' is not a registered human member"
		fi
	fi
	local rec
	rec="$(cat "$d/age.pub")"
	[[ "$rec" =~ ^age1[0-9a-z]{58}$ ]] || err "$h: age.pub must hold exactly one classic age1 recipient"
	[[ "$(field "$y" age_recipient)" == "$rec" ]] || err "$h: age_recipient does not match age.pub"
	[[ "$(awk '{print $1}' "$d/ssh.pub")" == ssh-ed25519 ]] || err "$h: ssh.pub must be ssh-ed25519"
	local fp
	fp="$(ssh_fp "$d/ssh.pub" 2>/dev/null)" || { err "$h: ssh.pub does not parse"; return; }
	[[ "$(field "$y" ssh_fingerprint)" == "$fp" ]] || err "$h: ssh_fingerprint does not match ssh.pub ($fp)"
	local exp
	exp="$(field "$y" expires)"
	if ! date -u -d "$exp" +%F >/dev/null 2>&1 || ! date -u -d "$(field "$y" enrolled)" +%F >/dev/null 2>&1; then
		err "$h: enrolled/expires must be YYYY-MM-DD"
	elif [[ "$exp" < "$(today)" && "$(field "$y" status)" == active ]]; then
		warn "$h: key expired on $exp (rotate or mark revoked)"
	fi
	# Proof of possession: member.yaml is signed by the key it publishes.
	local tmp
	tmp="$(mktemp)"
	printf '%s namespaces="%s" %s\n' "$h" "$NS_TEAM" "$(awk '{print $1, $2}' "$d/ssh.pub")" >"$tmp"
	local by
	if ssh-keygen -Y verify -f "$tmp" -I "$h" -n "$NS_TEAM" -s "$d/member.yaml.sig" <"$y" >/dev/null 2>&1; then
		ok "$h: $(field "$y" kind), $(field "$y" roles), $(field "$y" status), self-signed, $fp"
	elif [[ "$(field "$y" status)" == revoked ]] && by="$(maintainer_signer "$y" "$d/member.yaml.sig" "$NS_TEAM")" && [[ -n "$by" ]]; then
		ok "$h: revoked, record signed by maintainer $by"
	else
		err "$h: member.yaml.sig does not verify with ssh.pub (re-sign after any edit)"
	fi
	rm -f "$tmp"
	# Optional team-CA certificate.
	if [[ -f "$d/ssh-cert.pub" ]]; then
		if [[ ! -f "$TEAM/ca/team_ca.pub" ]]; then
			err "$h: ssh-cert.pub present but team/ca/team_ca.pub is missing"
		else
			local L cafp
			L="$(ssh-keygen -L -f "$d/ssh-cert.pub" 2>/dev/null)" || { err "$h: ssh-cert.pub does not parse"; return; }
			cafp="$(ssh_fp "$TEAM/ca/team_ca.pub")"
			grep -q "Signing CA: .* $cafp" <<<"$L" || err "$h: certificate is not signed by the team CA"
			grep -q "Public key: .* $fp\$" <<<"$L" || err "$h: certificate is for a different key"
			awk '/Principals:/{p=1;next} /Critical Options:/{p=0} p{print $1}' <<<"$L" | grep -qx "$h" || err "$h: certificate principals do not include $h"
			local until
			until="$(sed -n 's/.*Valid: from [^ ]* to \([^ ]*\).*/\1/p' <<<"$L")"
			if [[ -n "$until" && "$until" < "$(date -u +%FT%T)" ]]; then warn "$h: certificate expired $until"; fi
		fi
	fi
}

cmd_verify() {
	parse_meta "$@"
	command -v ssh-keygen >/dev/null || die "ssh-keygen is required"
	local n=0 h
	if [[ -d "$MEMBERS" ]]; then
		for d in "$MEMBERS"/*/; do
			[[ -d "$d" ]] || continue
			h="$(basename "$d")"
			verify_member "$h"
			n=$((n + 1))
		done
	fi
	((n)) || info "no members registered yet"
	# Private key material must never appear in the register or its tooling.
	local hits
	hits="$(grep -rlE "$(private_pattern)" "$TEAM" "$ROOT/scripts/team.sh" 2>/dev/null || true)"
	if [[ -n "$hits" ]]; then err "private key material found: $hits"; else ok "no private key material in team/"; fi
	local f
	for f in allowed_signers recipients.txt; do
		local want
		case "$f" in allowed_signers) want="$(gen_allowed_signers)" ;; recipients.txt) want="$(gen_recipients)" ;; esac
		if [[ "$(cat "$TEAM/$f" 2>/dev/null)" == "$want" ]]; then ok "team/$f is current"; else err "team/$f is stale (run scripts/team.sh update)"; fi
	done
	if [[ "$(cat "$TEAM/SHA256SUMS" 2>/dev/null)" == "$(gen_sums)" ]]; then
		ok "team/SHA256SUMS covers $(wc -l <"$TEAM/SHA256SUMS") files and is current"
	else
		err "team/SHA256SUMS is stale or a covered file changed (run scripts/team.sh update)"
		(cd "$ROOT" && sha256sum --quiet -c team/SHA256SUMS 2>&1 | sed 's/^/     /' >&2) || true
	fi
	if [[ -f "$TEAM/SHA256SUMS.sig" ]]; then
		local signer
		signer="$(maintainer_signer "$TEAM/SHA256SUMS" "$TEAM/SHA256SUMS.sig")"
		if [[ -n "$signer" ]]; then ok "team/SHA256SUMS signed by maintainer $signer"; else warn "team/SHA256SUMS.sig does not verify against any active maintainer (re-sign after merge)"; fi
	else
		info "team/SHA256SUMS is not signed yet (checksums alone only detect accidents)"
	fi
	echo
	if ((ERRORS)); then
		printf '%s%d error(s), %d warning(s)%s\n' "$C_ERR" "$ERRORS" "$WARNINGS" "$C_0"
		exit 1
	fi
	printf '%steam register valid%s: %d member(s), %d warning(s)\n' "$C_OK" "$C_0" "$n" "$WARNINGS"
}

maintainer_signer() {
	local file="$1" sig="$2" ns="${3:-$NS_FILE}" h
	while read -r h; do
		[[ -n "$h" ]] || continue
		[[ ",$(field "$MEMBERS/$h/member.yaml" roles)," == *,maintainer,* ]] || continue
		if ssh-keygen -Y verify -f "$TEAM/allowed_signers" -I "$h" -n "$ns" -s "$sig" <"$file" >/dev/null 2>&1; then
			echo "$h"
			return
		fi
	done < <(active_members)
}

# ---------- list ----------
cmd_list() {
	local h y
	printf '%s%-20s %-6s %-18s %-8s %-11s %s%s\n' "$C_B" HANDLE KIND ROLES STATUS EXPIRES SSH-FINGERPRINT "$C_0"
	[[ -d "$MEMBERS" ]] || return 0
	for d in "$MEMBERS"/*/; do
		[[ -f "$d/member.yaml" ]] || continue
		h="$(basename "$d")" y="$d/member.yaml"
		printf '%-20s %-6s %-18s %-8s %-11s %s\n' "$h" "$(field "$y" kind)" "$(field "$y" roles)" "$(field "$y" status)" "$(field "$y" expires)" "$(field "$y" ssh_fingerprint)"
	done
}

# ---------- signatures ----------
own_key() {
	local h="$1"
	valid_handle "$h" || die "--handle <your handle> is required"
	local k="${KEY_DIR:-$(key_dir_for "$h")}/ssh_ed25519"
	[[ -f "$k" ]] || die "missing $k"
	printf '%s' "$k"
}

cmd_sign_file() {
	parse_meta "$@"
	local file="${POSITIONAL[0]:-}" k
	[[ -f "$file" ]] || die "usage: sign-file <file> --handle H"
	k="$(own_key "$HANDLE_FLAG")"
	rm -f "$file.sig"
	ssh-keygen -q -Y sign -f "$k" -n "$NS_FILE" "$file" >/dev/null
	ok "wrote $file.sig (namespace $NS_FILE, signer $HANDLE_FLAG)"
}

cmd_verify_file() {
	parse_meta "$@"
	local file="${POSITIONAL[0]:-}"
	[[ -f "$file" && -f "$file.sig" ]] || die "usage: verify-file <file> --handle H (needs <file>.sig)"
	valid_handle "$HANDLE_FLAG" || die "--handle is required"
	ssh-keygen -Y verify -f "$TEAM/allowed_signers" -I "$HANDLE_FLAG" -n "$NS_FILE" -s "$file.sig" <"$file" ||
		die "signature does NOT verify for $HANDLE_FLAG"
}

cmd_sign_sums() {
	parse_meta "$@"
	local k
	k="$(own_key "$HANDLE_FLAG")"
	[[ ",$(field "$MEMBERS/$HANDLE_FLAG/member.yaml" roles 2>/dev/null)," == *,maintainer,* ]] || die "$HANDLE_FLAG is not a registered maintainer"
	[[ "$(cat "$TEAM/SHA256SUMS")" == "$(gen_sums)" ]] || die "team/SHA256SUMS is stale; run update and review first"
	rm -f "$TEAM/SHA256SUMS.sig"
	ssh-keygen -q -Y sign -f "$k" -n "$NS_FILE" "$TEAM/SHA256SUMS" >/dev/null
	ok "signed team/SHA256SUMS as $HANDLE_FLAG"
}

cmd_revoke() {
	parse_meta "$@"
	local h="${POSITIONAL[0]:-}" y k
	y="$MEMBERS/$h/member.yaml"
	[[ -f "$y" ]] || die "unknown member: $h"
	k="$(own_key "$HANDLE_FLAG")"
	[[ ",$(field "$MEMBERS/$HANDLE_FLAG/member.yaml" roles)," == *,maintainer,* ]] || die "$HANDLE_FLAG is not a registered maintainer"
	[[ "$(field "$MEMBERS/$HANDLE_FLAG/member.yaml" status)" == active ]] || die "$HANDLE_FLAG is not active"
	sed -i 's/^status: .*/status: revoked/' "$y"
	rm -f "$y.sig"
	ssh-keygen -q -Y sign -f "$k" -n "$NS_TEAM" "$y" >/dev/null
	ok "revoked $h (record signed by $HANDLE_FLAG); removed from allowed_signers and recipients.txt"
	cmd_update
	info "next: remove $h from recipients/maintainers.txt and .sops.yaml if enrolled, then p123 rotate --all (CONTRIBUTING.md)"
}

# ---------- certificates (team SSH CA) ----------
CA_DIR="${P123_TEAM_CA:-${XDG_CONFIG_HOME:-$HOME/.config}/password123/ca}"

cmd_ca_init() {
	parse_meta "$@"
	refuse_inside_repo "$CA_DIR"
	[[ ! -e "$CA_DIR/team_ca" ]] || die "$CA_DIR/team_ca already exists"
	[[ ! -e "$TEAM/ca/team_ca.pub" ]] || die "team/ca/team_ca.pub already exists (CA rotation is a reviewed PR)"
	umask 077
	mkdir -p "$CA_DIR" "$TEAM/ca"
	ssh-keygen -q -t ed25519 -a 100 -C "password123-team-ca" -f "$CA_DIR/team_ca"
	awk '{print $1, $2, $3}' "$CA_DIR/team_ca.pub" >"$TEAM/ca/team_ca.pub"
	ok "team CA private key in $CA_DIR/team_ca (keep it offline); public key in team/ca/team_ca.pub"
	cmd_update
}

cmd_issue_cert() {
	VALIDITY="+52w"
	parse_meta "$@"
	local h="${POSITIONAL[0]:-}" d y principals serial
	d="$MEMBERS/$h" y="$d/member.yaml"
	[[ -f "$y" ]] || die "unknown member: $h"
	[[ "$(field "$y" status)" == active ]] || die "$h is not active"
	[[ -f "$CA_DIR/team_ca" && -f "$TEAM/ca/team_ca.pub" ]] || die "team CA missing (run ca-init)"
	principals="$h"
	if [[ -n "$(field "$y" email)" ]]; then principals="$h,$(field "$y" email)"; fi
	serial="$(date -u +%Y%m%d%H%M%S)"
	cp "$d/ssh.pub" "$d/.cert-input.pub"
	ssh-keygen -q -s "$CA_DIR/team_ca" -I "$h@password123" -n "$principals" -V "$VALIDITY" -z "$serial" "$d/.cert-input.pub"
	mv "$d/.cert-input-cert.pub" "$d/ssh-cert.pub"
	rm -f "$d/.cert-input.pub"
	ok "issued team certificate for $h (principals $principals, validity $VALIDITY, serial $serial)"
	cmd_update
}

# ---------- dispatch ----------
cmd="${1:-help}"
shift || true
case "$cmd" in
keygen) cmd_keygen "$@" ;;
register) cmd_register "$@" ;;
update) cmd_update ;;
verify | check) cmd_verify "$@" ;;
list) cmd_list ;;
sign-file) cmd_sign_file "$@" ;;
verify-file) cmd_verify_file "$@" ;;
sign-sums) cmd_sign_sums "$@" ;;
revoke) cmd_revoke "$@" ;;
ca-init) cmd_ca_init "$@" ;;
issue-cert) cmd_issue_cert "$@" ;;
help | -h | --help) usage ;;
version | --version) echo "team.sh v$VERSION" ;;
*) usage >&2; die "unknown command: $cmd" ;;
esac
