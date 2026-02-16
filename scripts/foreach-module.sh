#!/usr/bin/env bash
#
# foreach-module.sh — run a command in every Go module directory with pretty output.
#
# Usage:  foreach-module.sh <label> <description> <modules...> -- <cmd...>
#
# Example:
#   foreach-module.sh vet "go vet across all modules" . ./drivers/pgx -- go vet ./...
#
set -uo pipefail

# ── parse args ───────────────────────────────────────────────
label="$1"; shift
desc="$1"; shift

modules=()
while [[ $# -gt 0 && "$1" != "--" ]]; do
    modules+=("$1"); shift
done
shift  # consume the "--"
cmd=("$@")

# ── ANSI codes ───────────────────────────────────────────────
R='\033[0m' B='\033[1m' D='\033[2m'
G='\033[32m' RD='\033[31m' C='\033[36m'
BG_G='\033[42m' BG_R='\033[41m'
OK="${G}✔${R}" KO="${RD}✘${R}"
SEP=$(printf '─%.0s' {1..60})

# ── header ───────────────────────────────────────────────────
printf "\n${B}${C}  %s ${R}${D} %s${R}\n" "$label" "$desc"
printf "${D}%s${R}\n\n" "$SEP"

# ── run per module ───────────────────────────────────────────
total=0; passed=0; failed=0; fail_list=""

for dir in "${modules[@]}"; do
    total=$((total + 1))
    mod_name="${dir#./}"; [ "$dir" = "." ] && mod_name="(root)"
    printf "  ${D}▸${R} %-45s" "$mod_name"

    output=$(cd "$dir" && "${cmd[@]}" 2>&1)
    status=$?

    if [ $status -eq 0 ]; then
        printf "${OK}\n"
        passed=$((passed + 1))
    else
        printf "${KO}\n"
        if [ -n "$output" ]; then
            while IFS= read -r line; do
                printf "    ${D}%s${R}\n" "$line"
            done <<< "$output"
        fi
        failed=$((failed + 1))
        fail_list="${fail_list}    ${RD}•${R} ${mod_name}\n"
    fi
done

# ── summary ──────────────────────────────────────────────────
printf "\n${D}%s${R}\n" "$SEP"
if [ $failed -eq 0 ]; then
    printf "  ${BG_G}${B} PASS ${R} ${G}All %d modules passed${R}\n\n" "$total"
else
    printf "  ${BG_R}${B} FAIL ${R} ${RD}%d of %d modules failed:${R}\n" "$failed" "$total"
    printf "$fail_list"
    printf "\n"
    exit 1
fi
