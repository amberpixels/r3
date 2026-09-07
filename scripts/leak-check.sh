#!/usr/bin/env bash
# Asserts that importing an r3 driver does not drag a backend the consumer never
# compiles into its go.sum.
#
# The failure this guards against is subtle and silent: `go mod tidy` records
# checksums for the *tests* of packages you import, so one test file importing
# testcontainers or gorm puts that whole stack into every consumer's go.sum. It
# cost r3 the moby/docker tree until the container suites moved to e2e/, and then
# cost it gorm+sqlite again through a single test in features/history.
#
# The check is empirical on purpose: it tidies a throwaway consumer against this
# checkout and reads the go.sum that falls out, rather than trusting a rule about
# which imports are safe.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# Each case: <name> <import path under r3> <backends that must NOT appear>
run_case() {
	local name="$1" imports="$2" forbidden="$3"
	local dir="$work/$name"
	mkdir -p "$dir"

	cat >"$dir/go.mod" <<EOF
module leakcheck$name

go 1.26.0

require github.com/amberpixels/r3 v0.0.0

replace github.com/amberpixels/r3 => $root
EOF

	{
		echo "package main"
		echo
		echo "import ("
		for p in $imports; do echo "	_ \"$p\""; done
		echo ")"
		echo
		echo "func main() {}"
	} >"$dir/main.go"

	(cd "$dir" && GOFLAGS=-mod=mod go mod tidy >/dev/null 2>&1)

	local found=""
	for pat in $forbidden; do
		if grep -q "$pat" "$dir/go.sum" 2>/dev/null; then
			found="$found $pat"
		fi
	done

	if [ -n "$found" ]; then
		echo "FAIL  $name leaks:$found"
		echo "      imports: $imports"
		echo "      Something these packages' TESTS import reaches those modules."
		echo "      Move the offending test into the e2e module."
		return 1
	fi
	echo "ok    $name  ($(wc -l <"$dir/go.sum" | tr -d ' ') go.sum lines)"
}

container_stack="testcontainers moby/ docker/"
sql_stack="gorm.io mattn/go-sqlite3 go-sql-driver/mysql lib/pq jackc/pgx uptrace/bun go-pg/pg"

failed=0

# The core and every backend-agnostic decorator must pull in no backend at all.
run_case core \
	"github.com/amberpixels/r3 github.com/amberpixels/r3/features/history github.com/amberpixels/r3/features/permissions github.com/amberpixels/r3/features/metrics github.com/amberpixels/r3/features/validation github.com/amberpixels/r3/features/softdelete" \
	"$container_stack $sql_stack" || failed=1

# A driver pulls in its own backend and nothing else: no container stack, and no
# sibling backend it has no business knowing about.
run_case mongo "github.com/amberpixels/r3/drivers/mongo" "$container_stack $sql_stack" || failed=1
run_case pq "github.com/amberpixels/r3/drivers/pq" "$container_stack gorm.io go-pg/pg uptrace/bun" || failed=1
run_case gorm "github.com/amberpixels/r3/drivers/gorm" "$container_stack go-pg/pg uptrace/bun lib/pq" || failed=1

exit $failed
