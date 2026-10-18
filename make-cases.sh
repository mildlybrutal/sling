#!/usr/bin/env bash
# Creates the test commits that Sling's run-cases.sh deploys.
#
# Run it INSIDE your demo-app repo (sling), on the branch where main.go,
# Dockerfile and .dockerignore are already committed in their working state.
# That commit is tagged good-v1. The deliberately broken commits go on a
# separate branch, test-cases, so main stays clean:
#
#   good-v1     working                       (tag on your current commit)
#   crash       exits immediately on boot     }
#   bad-health  /health returns 500           } on branch test-cases
#   slow-start  10s before it starts listening}
#   good-v2     working, different message    }
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

die() { echo "error: $*" >&2; exit 1; }

[ -z "$(git status --porcelain)" ] || die "working tree not clean: commit or stash first"
[ -f Dockerfile ] || die "no Dockerfile in repo root"
grep -q 'behavior under test' main.go || die "main.go is missing the 'behavior under test' const block"
grep -qE 'crashOnBoot +\= false' main.go || die "main.go must start in the working state (crashOnBoot = false)"
for t in good-v1 crash bad-health slow-start good-v2; do
  git rev-parse -q --verify "refs/tags/$t" >/dev/null && die "tag $t already exists (delete it with: git tag -d $t)"
done

start="$(git rev-parse --abbrev-ref HEAD)"
git tag good-v1

git checkout -q -b test-cases

# sed -i.bak works on both GNU (Linux) and BSD (macOS) sed.
edit()   { sed -i.bak -E "$1" main.go && rm -f main.go.bak; }
commit() { gofmt -w main.go; git add -A; git commit -q -m "$1"; git tag "$2"; }

edit 's/crashOnBoot +\= false/crashOnBoot = true/'
commit "test: crash on boot" crash

edit 's/crashOnBoot +\= true/crashOnBoot = false/; s/healthStatus +\= http.StatusOK/healthStatus = http.StatusInternalServerError/'
commit "test: /health returns 500" bad-health

edit 's/healthStatus +\= http.StatusInternalServerError/healthStatus = http.StatusOK/; s/startupDelay +\= 0 \* time.Second/startupDelay = 10 * time.Second/'
commit "test: slow start (10s)" slow-start

edit 's/startupDelay +\= 10 \* time.Second/startupDelay = 0 * time.Second/; s/hello from v1/hello from v2/'
commit "test: working v2" good-v2

git checkout -q "$start"

echo
git log --oneline --decorate --all | head -6
echo
for t in good-v1 crash bad-health slow-start good-v2; do printf '%-11s %s\n' "$t" "$(git rev-parse "$t")"; done
echo
echo "Back on '$start'. Push the branch and tags so Sling can clone them:"
echo "  git push origin test-cases --tags"
