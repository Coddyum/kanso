#!/bin/bash
# The GitHub settings of Kanso's repository, for its maintainer. Safe to run
# again: each call sets a value, none adds one twice.
#
#   ./scripts/github.sh           what also works on a private repository
#   ./scripts/github.sh --public  checks the code, makes the repository public,
#                                 then sets everything below
#
#   main      changed only through a pull request whose check "test" is green,
#             squash merge only; nobody pushes to it, the maintainer included;
#             no force push, no deletion. Merging needs write access: only the
#             owner has it, and the script says so if anyone else does.
#   tags      only the owner creates, moves or deletes one.
#   Actions   GitHub's own actions only, a read-only token, and a workflow of
#             an outside contributor waits for the owner's approval.
#   security  Dependabot alerts and fixes, secret scanning with push protection,
#             CodeQL, private vulnerability reporting.
set -euo pipefail
cd "$(dirname "$0")/.."

REPO="${KANSO_REPO:-Coddyum/kanso}"
OWNER="${REPO%%/*}"
failed=0

ok() { printf '  ✓ %s\n' "$1"; }
fail() {
	printf '  ✗ %s\n' "$1"
	failed=1
}

# call METHOD PATH LABEL — sends the JSON read on stdin to the repository's PATH.
call() {
	local out
	if out=$(gh api -X "$1" "repos/$REPO$2" --input - 2>&1 >/dev/null); then ok "$3"; else fail "$3: $out"; fi
}

# enable PATH LABEL — turns on a feature that takes no body.
enable() {
	local out
	if out=$(gh api -X PUT "repos/$REPO$1" 2>&1 >/dev/null); then ok "$2"; else fail "$2: $out"; fi
}

# ruleset NAME LABEL — creates the ruleset read on stdin, or replaces the one of that name.
ruleset() {
	local body id
	body=$(cat)
	id=$(gh api "repos/$REPO/rulesets" --jq ".[] | select(.name == \"$1\") | .id") || id=""
	if [ -n "$id" ]; then
		printf '%s' "$body" | call PUT "/rulesets/$id" "$2"
	else
		printf '%s' "$body" | call POST "/rulesets" "$2"
	fi
}

publish() {
	echo "Before publishing"
	if [ "$(git branch --show-current)" != main ] || ! git diff --quiet HEAD; then
		echo "  Switch to main and commit or set aside your changes first: nothing was changed."
		exit 1
	fi
	git fetch -q origin main
	if [ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]; then
		echo "  Your main is not the one on GitHub: push it, or pull it, first. Nothing was changed."
		exit 1
	fi
	if ! go run ./tools/privacy -all; then
		echo "  The privacy guard refuses: nothing was changed."
		exit 1
	fi
	ok "main is the one on GitHub, and the privacy guard is green"
	read -r -p "  Make $REPO public? Everyone will see its code and its history. (yes/no) " answer
	if [ "$answer" != yes ]; then
		echo "  Nothing was changed."
		exit 0
	fi
	gh repo edit "$REPO" --visibility public --accept-visibility-change-consequences
	ok "$REPO is public"
}

[ "${1-}" = --public ] && publish
[ "$(gh api "repos/$REPO" --jq .private)" = false ] && public=1 || public=0

echo "Repository"
call PATCH "" "merge by squash only, branches deleted once merged, no wiki" <<'EOF'
{
  "description": "Your micro-business's subscriptions, invoices and paperwork, together on your Mac. 100% local.",
  "has_issues": true,
  "has_wiki": false,
  "has_projects": false,
  "has_discussions": false,
  "allow_squash_merge": true,
  "allow_merge_commit": false,
  "allow_rebase_merge": false,
  "squash_merge_commit_title": "PR_TITLE",
  "squash_merge_commit_message": "PR_BODY",
  "delete_branch_on_merge": true,
  "allow_auto_merge": false,
  "allow_update_branch": true
}
EOF
call PUT "/topics" "topics" <<'EOF'
{"names": ["macos", "local-first", "privacy", "invoices", "expense-tracker", "micro-entreprise", "go", "swift"]}
EOF
if out=$(gh label create translation --repo "$REPO" --color 7178f0 \
	--description "A text of the app in one of its languages" --force 2>&1 >/dev/null); then
	ok "label translation"
else
	fail "label translation: $out"
fi
if ! others=$(gh api "repos/$REPO/collaborators" --jq "[.[] | select(.login != \"$OWNER\") | .login] | join(\", \")"); then
	fail "could not list who has access (Settings › Collaborators)"
elif [ -z "$others" ]; then
	ok "nobody but $OWNER can push or merge"
else
	fail "these accounts can also merge: $others (Settings › Collaborators)"
fi

echo "Actions"
call PUT "/actions/permissions" "Actions on, GitHub's own actions only" <<'EOF'
{"enabled": true, "allowed_actions": "selected"}
EOF
call PUT "/actions/permissions/selected-actions" "no third-party action" <<'EOF'
{"github_owned_allowed": true, "verified_allowed": false, "patterns_allowed": []}
EOF
call PUT "/actions/permissions/workflow" "read-only token, never approves a pull request" <<'EOF'
{"default_workflow_permissions": "read", "can_approve_pull_request_reviews": false}
EOF

echo "Security"
enable "/vulnerability-alerts" "Dependabot alerts"
enable "/automated-security-fixes" "Dependabot security fixes"

if [ "$public" = 0 ]; then
	echo
	echo "The repository is private: the rules of main and tags, secret scanning, CodeQL and"
	echo "private reports need it public. Run ./scripts/github.sh --public to publish it."
	exit "$failed"
fi

call PUT "/actions/permissions/fork-pr-contributor-approval" "an outside contributor's workflow waits for approval" <<'EOF'
{"approval_policy": "all_external_contributors"}
EOF
call PATCH "" "secret scanning, with push protection" <<'EOF'
{"security_and_analysis": {"secret_scanning": {"status": "enabled"}, "secret_scanning_push_protection": {"status": "enabled"}}}
EOF
enable "/private-vulnerability-reporting" "private vulnerability reporting"
# Swift is left out: CodeQL builds it only from an Xcode project or a Swift
# package, and Kanso compiles its two Swift files with swiftc.
call PATCH "/code-scanning/default-setup" "CodeQL on Go, JavaScript and the workflows" <<'EOF'
{"state": "configured", "query_suite": "default", "languages": ["actions", "go", "javascript-typescript"]}
EOF

echo "Rules"
ruleset main "main: pull request, green tests, squash, for everyone" <<'EOF'
{
  "name": "main",
  "target": "branch",
  "enforcement": "active",
  "conditions": {"ref_name": {"include": ["~DEFAULT_BRANCH"], "exclude": []}},
  "bypass_actors": [],
  "rules": [
    {"type": "deletion"},
    {"type": "non_fast_forward"},
    {"type": "required_linear_history"},
    {"type": "pull_request", "parameters": {
      "required_approving_review_count": 0,
      "dismiss_stale_reviews_on_push": true,
      "require_code_owner_review": false,
      "require_last_push_approval": false,
      "required_review_thread_resolution": true,
      "allowed_merge_methods": ["squash"]
    }},
    {"type": "required_status_checks", "parameters": {
      "strict_required_status_checks_policy": true,
      "required_status_checks": [{"context": "test", "integration_id": 15368}]
    }}
  ]
}
EOF
ruleset tags "tags: only the owner creates, moves or deletes one" <<'EOF'
{
  "name": "tags",
  "target": "tag",
  "enforcement": "active",
  "conditions": {"ref_name": {"include": ["~ALL"], "exclude": []}},
  "bypass_actors": [{"actor_id": 5, "actor_type": "RepositoryRole", "bypass_mode": "always"}],
  "rules": [{"type": "creation"}, {"type": "update"}, {"type": "deletion"}]
}
EOF

exit "$failed"
