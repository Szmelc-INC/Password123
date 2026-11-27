#!/usr/bin/env bash
# End-to-end test for scripts/team.sh with throwaway keys.
# Runs in a temporary copy, so it never touches the real register or your keys.
set -uo pipefail

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
T="$(mktemp -d)"
trap 'rm -rf "$T"' EXIT
mkdir -p "$T/root/scripts"
cp "$SRC/scripts/team.sh" "$T/root/scripts/"
cp "$SRC/go.mod" "$T/root/"
[[ -f "$SRC/go.sum" ]] && cp "$SRC/go.sum" "$T/root/"
cd "$T/root" || exit 1
export P123_TEAM_KEYS="$T/keys" P123_TEAM_CA="$T/ca" NO_COLOR=1

FAIL=0 PASS=0
S() { bash scripts/team.sh "$@"; }
expect_ok() {
	local name="$1"
	shift
	if "$@" >"$T/out" 2>&1; then PASS=$((PASS + 1)); echo "pass  $name"; else FAIL=$((FAIL + 1)); echo "FAIL  $name"; sed 's/^/      /' "$T/out"; fi
}
expect_fail() {
	local name="$1"
	shift
	if "$@" >"$T/out" 2>&1; then FAIL=$((FAIL + 1)); echo "FAIL  $name (expected failure)"; sed 's/^/      /' "$T/out"; else PASS=$((PASS + 1)); echo "pass  $name"; fi
}
expect_grep() {
	local name="$1" pat="$2"
	shift 2
	"$@" >"$T/out" 2>&1
	if grep -qE "$pat" "$T/out"; then PASS=$((PASS + 1)); echo "pass  $name"; else FAIL=$((FAIL + 1)); echo "FAIL  $name (no match for /$pat/)"; sed 's/^/      /' "$T/out"; fi
}
resign() { rm -f "$2.sig"; ssh-keygen -q -Y sign -f "$T/keys/$1/ssh_ed25519" -n "$3" "$2" >/dev/null 2>&1; }

Y=team/members/bot-one/member.yaml

expect_fail "missing generated files are reported" S verify
S update >/dev/null
expect_ok "empty register verifies" S verify
expect_ok "keygen human maintainer" S keygen alice --email alice@example.org --roles member,maintainer --no-passphrase
expect_ok "keygen agent" S keygen bot-one --kind agent --operator alice --no-passphrase
expect_fail "agent without operator is refused" S keygen bot-two --kind agent --no-passphrase
expect_fail "wildcard email is refused at keygen" S keygen carol --email '*@*' --no-passphrase
expect_fail "existing keys are not overwritten" S keygen alice --no-passphrase
expect_fail "key dir inside the repository is refused" env P123_TEAM_KEYS="$T/root/keys" bash scripts/team.sh keygen dave --no-passphrase
expect_ok "two members verify" S verify --strict
expect_grep "private keys are mode 600" '^600$' stat -c %a "$T/keys/alice/age.key"
expect_grep "allowed_signers lists both" '^bot-one namespaces=' cat team/allowed_signers
expect_ok "sha256sum -c passes" sha256sum -c --quiet team/SHA256SUMS

printf '\n\n' | S ca-init >/dev/null 2>&1
expect_ok "team CA created" test -f team/ca/team_ca.pub
expect_ok "certificate issued" S issue-cert alice --validity +4w
expect_grep "certificate verified" 'alice: human' S verify --strict

expect_ok "maintainer signs SHA256SUMS" S sign-sums --handle alice
expect_grep "SHA256SUMS signature attributed" 'signed by maintainer alice' S verify --strict
expect_fail "agent cannot sign-sums" S sign-sums --handle bot-one

echo hello >"$T/f.txt"
expect_ok "sign-file" S sign-file "$T/f.txt" --handle bot-one
expect_ok "verify-file" S verify-file "$T/f.txt" --handle bot-one
expect_fail "verify-file with the wrong signer" S verify-file "$T/f.txt" --handle alice

# Principal injection: bot-one claims alice's principal and re-signs its own record.
cp "$Y" "$T/y.bak"
sed -i 's/^email: .*/email: "x@example.org,alice"/' "$Y"
resign bot-one "$Y" password123-team
S update >/dev/null
resign bot-one team/SHA256SUMS password123-file
expect_fail "injected principal is rejected" S verify
expect_grep "bot signature is not attributed to alice" 'does not verify against any active maintainer' S verify
cp "$T/y.bak" "$Y"
resign bot-one "$Y" password123-team
S update >/dev/null
S sign-sums --handle alice >/dev/null

# Tampering.
cp "$Y" "$T/y.bak"
sed -i 's/^roles: member$/roles: member,maintainer/' "$Y"
expect_fail "edited member.yaml is rejected" S verify
cp "$T/y.bak" "$Y"
cp team/members/alice/age.pub "$T/a.bak"
echo extra >>team/members/alice/age.pub
expect_fail "edited age.pub is rejected" S verify
cp "$T/a.bak" team/members/alice/age.pub
cp team/members/alice/member.yaml "$T/alice.bak"
sed -i 's/^expires: .*/expires: now/' team/members/alice/member.yaml
resign alice team/members/alice/member.yaml password123-team
expect_fail "free-form date is rejected" S verify
cp "$T/alice.bak" team/members/alice/member.yaml
resign alice team/members/alice/member.yaml password123-team
S update >/dev/null
expect_ok "register is clean again" S verify

# Planted secret (pattern assembled at runtime).
mkdir -p team/members/zz-leak
printf '%s%s\n' "AGE-SECRET-KEY-" "1QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ" >team/members/zz-leak/leak
expect_grep "planted secret is detected" 'private key material found' S verify
rm -rf team/members/zz-leak

# Revocation by a maintainer.
expect_ok "maintainer revokes agent" S revoke bot-one --handle alice
expect_grep "revoked record verifies" 'bot-one: revoked, record signed by maintainer alice' S verify
expect_fail "revoked member left allowed_signers" grep -q '^bot-one' team/allowed_signers
expect_fail "revoked member left recipients.txt" grep -q 'bot-one' team/recipients.txt

echo
echo "$PASS passed, $FAIL failed"
((FAIL == 0))
