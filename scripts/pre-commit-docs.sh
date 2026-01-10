#!/usr/bin/env bash
#
# Pre-commit hook: Regenerate documentation diagrams when Go files change
#
# This hook checks if any .go files are staged for commit. If so, it
# regenerates the AST-based diagrams and stages them automatically.
#
# In strict review mode (git config docs.strictReview true), it also:
# - Shows meaningful diffs of structural changes
# - Extracts connected classes to help understand impact
# - Requires architecture.md to be staged if diagrams changed
#

set -o pipefail


## setup #######################################################################
################################################################################

REPO_ROOT="$(git rev-parse --show-toplevel)"
GENERATE_SCRIPT="${REPO_ROOT}/scripts/generate-docs.sh"
DIAGRAMS_DIR="${REPO_ROOT}/docs/diagrams/generated"
TRACKING_FILE="${DIAGRAMS_DIR}/.tracking"
ARCHITECTURE_FILE="${REPO_ROOT}/docs/architecture.md"

# Colors (only if terminal)
if [[ -t 1 ]]; then
    C_INFO=$'\033[0;34m'
    C_SUCCESS=$'\033[0;32m'
    C_WARN=$'\033[0;33m'
    C_ERROR=$'\033[0;31m'
    C_BOLD=$'\033[1m'
    C_DIM=$'\033[2m'
    C_RESET=$'\033[0m'
else
    C_INFO="" C_SUCCESS="" C_WARN="" C_ERROR="" C_BOLD="" C_DIM="" C_RESET=""
fi


## helpful functions ###########################################################
################################################################################

function info() {
    echo -e "${C_INFO}[docs]${C_RESET} ${1}"
}

function success() {
    echo -e "${C_SUCCESS}[docs]${C_RESET} ${1}"
}

function warn() {
    echo -e "${C_WARN}[docs]${C_RESET} ${1}"
}

function error() {
    echo -e "${C_ERROR}[docs]${C_RESET} ${1}" >&2
}


## fingerprinting functions ####################################################
################################################################################

function _sha256() {
    :  'Compute SHA256 hash of a file'
    sha256sum "${1}" 2>/dev/null | cut -d' ' -f1
}

function _byte-freq-hash() {
    :  'Compute byte frequency fingerprint of a file

        Returns a hash of the byte frequency distribution.
        Useful for files with non-deterministic ordering.
    '
    od -An -tu1 -w1 "${1}" 2>/dev/null | sort -n | uniq -c | sha256sum | cut -d' ' -f1
}

function _compute-fingerprints() {
    :  'Compute fingerprints for current source files

        Sets global variables:
        - CURRENT_PUML_SHA256
        - CURRENT_GV_FINGERPRINT
    '
    if [[ -f "${DIAGRAMS_DIR}/classes.puml" ]]; then
        CURRENT_PUML_SHA256=$(_sha256 "${DIAGRAMS_DIR}/classes.puml")
    else
        CURRENT_PUML_SHA256=""
    fi

    if [[ -f "${DIAGRAMS_DIR}/callgraph.gv" ]]; then
        CURRENT_GV_FINGERPRINT=$(_byte-freq-hash "${DIAGRAMS_DIR}/callgraph.gv")
    else
        CURRENT_GV_FINGERPRINT=""
    fi
}


## tracking file functions #####################################################
################################################################################

function _read-tracking() {
    :  'Read tracking file into global variables

        Sets:
        - TRACKED_PUML_SHA256
        - TRACKED_GV_FINGERPRINT
        - TRACKED_VERIFIED_AT
    '
    TRACKED_PUML_SHA256=""
    TRACKED_GV_FINGERPRINT=""
    TRACKED_VERIFIED_AT=""

    if [[ -f "${TRACKING_FILE}" ]]; then
        # shellcheck source=/dev/null
        source "${TRACKING_FILE}"
        TRACKED_PUML_SHA256="${CLASSES_PUML_SHA256:-}"
        TRACKED_GV_FINGERPRINT="${CALLGRAPH_GV_FINGERPRINT:-}"
        TRACKED_VERIFIED_AT="${VERIFIED_AT:-}"
    fi
}

function _write-tracking() {
    :  'Write current fingerprints to tracking file'
    cat > "${TRACKING_FILE}" << EOF
# Auto-generated tracking file for documentation diagrams
# Maps source fingerprints to verified architecture state
CLASSES_PUML_SHA256="${CURRENT_PUML_SHA256}"
CALLGRAPH_GV_FINGERPRINT="${CURRENT_GV_FINGERPRINT}"
VERIFIED_AT="$(date -Iseconds)"
EOF
}

function _sources-match-tracking() {
    :  'Check if current sources match tracked fingerprints

        @return
            0 if match (architecture is up-to-date), 1 if mismatch
    '
    _read-tracking
    _compute-fingerprints

    [[ "${CURRENT_PUML_SHA256}" == "${TRACKED_PUML_SHA256}" ]] && \
    [[ "${CURRENT_GV_FINGERPRINT}" == "${TRACKED_GV_FINGERPRINT}" ]]
}


## diff and context extraction #################################################
################################################################################

function _extract-changed-classes() {
    :  'Extract class names that changed from a unified diff

        Reads diff from stdin, outputs class names (one per line)
    '
    local -- __line
    local -a __classes=()

    while IFS= read -r __line; do
        # Look for added/removed lines with class definitions
        # Format: class "package.ClassName" or just class "ClassName"
        if [[ "${__line}" =~ ^[+-][[:space:]]*class[[:space:]]+\"([^\"]+)\" ]]; then
            __classes+=("${BASH_REMATCH[1]}")
        fi
        # Also catch method additions/removals within a class context
        # Format: + MethodName(...) or - MethodName(...)
        if [[ "${__line}" =~ ^[+-][[:space:]]+\+[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)\( ]]; then
            # This is a method change - we'd need context to know which class
            # For now, we'll rely on class-level detection
            :
        fi
    done

    # Deduplicate and output
    printf '%s\n' "${__classes[@]}" | sort -u
}

function _extract-connected-context() {
    :  'Extract class definitions and relationships for given class names

        @arg puml_file
            Path to the PlantUML file
        @arg class_names
            Newline-separated list of class names (stdin or arg)

        Outputs the class definitions and all relationships involving them.
    '
    local -- __puml_file="${1}"
    local -- __class_name
    local -a __patterns=()

    # Read class names from stdin or remaining args
    if [[ -t 0 ]]; then
        shift
        for __class_name in "${@}"; do
            __patterns+=("${__class_name}")
        done
    else
        while IFS= read -r __class_name; do
            [[ -n "${__class_name}" ]] && __patterns+=("${__class_name}")
        done
    fi

    if [[ ${#__patterns[@]} -eq 0 ]]; then
        return 0
    fi

    # Build grep pattern
    local -- __grep_pattern
    __grep_pattern=$(printf '%s\|' "${__patterns[@]}")
    __grep_pattern="${__grep_pattern%\\|}"  # Remove trailing \|

    # Extract:
    # 1. Class definitions containing these names
    # 2. Relationship lines containing these names
    grep -E "(class \"[^\"]*($__grep_pattern)[^\"]*\"|\"[^\"]*($__grep_pattern)[^\"]*\"[[:space:]]*(#\.\.|<\|--|\*--|-->|\.\.>))" "${__puml_file}" 2>/dev/null || true
}

function _generate-review-context() {
    :  'Generate context for architecture review

        Shows:
        1. The diff of changed classes
        2. Connected classes and relationships
        3. Instructions for the reviewer

        @arg old_file
            Path to old .puml file
        @arg new_file
            Path to new .puml file
    '
    local -- __old_file="${1}"
    local -- __new_file="${2}"
    local -- __diff
    local -- __changed_classes
    local -- __connected_context

    # Generate diff
    __diff=$(diff -u "${__old_file}" "${__new_file}" 2>/dev/null || true)

    if [[ -z "${__diff}" ]]; then
        echo "No structural changes detected in class diagram."
        return 0
    fi

    # Extract changed class names
    __changed_classes=$(echo "${__diff}" | _extract-changed-classes)

    # Get connected context
    if [[ -n "${__changed_classes}" ]]; then
        __connected_context=$(echo "${__changed_classes}" | _extract-connected-context "${__new_file}")
    fi

    # Output review context
    echo ""
    echo "${C_BOLD}═══════════════════════════════════════════════════════════════════${C_RESET}"
    echo "${C_BOLD}                    ARCHITECTURE REVIEW REQUIRED${C_RESET}"
    echo "${C_BOLD}═══════════════════════════════════════════════════════════════════${C_RESET}"
    echo ""
    echo "${C_INFO}The following structural changes were detected:${C_RESET}"
    echo ""
    echo "${C_DIM}--- classes.puml (before)${C_RESET}"
    echo "${C_DIM}+++ classes.puml (after)${C_RESET}"
    echo "${__diff}" | head -100
    echo ""

    if [[ -n "${__changed_classes}" ]]; then
        echo "${C_INFO}Changed classes:${C_RESET}"
        echo "${__changed_classes}" | sed 's/^/  - /'
        echo ""
    fi

    if [[ -n "${__connected_context}" ]]; then
        echo "${C_INFO}Connected relationships (may be impacted):${C_RESET}"
        echo "${__connected_context}" | sed 's/^/  /'
        echo ""
    fi

    echo "${C_BOLD}Please review:${C_RESET}"
    echo "  1. Will these changes negatively impact connected logic?"
    echo "  2. Do they fit the overall architecture?"
    echo "  3. Update ${C_INFO}docs/architecture.md${C_RESET} to reflect these changes"
    echo ""
    echo "${C_WARN}Stage architecture.md and retry the commit.${C_RESET}"
    echo ""
    echo "${C_BOLD}═══════════════════════════════════════════════════════════════════${C_RESET}"
    echo ""
}


## main logic ##################################################################
################################################################################

function _is-strict-mode() {
    :  'Check if strict review mode is enabled'
    local -- __value
    __value=$(git config --get docs.strictReview 2>/dev/null || echo "false")
    [[ "${__value}" == "true" ]]
}

function _is-architecture-staged() {
    :  'Check if architecture.md is staged for commit'
    git diff --cached --name-only | grep -q "^docs/architecture.md$"
}

function _diagrams-actually-changed() {
    :  'Check if diagram sources actually changed by comparing .new files

        This is called BEFORE the generate script commits/cleans up temp files.
        Returns 0 if sources changed, 1 if identical.
    '
    # If no .new files exist, no changes were generated
    if [[ ! -f "${DIAGRAMS_DIR}/classes.puml.new" ]] && [[ ! -f "${DIAGRAMS_DIR}/callgraph.gv.new" ]]; then
        return 1  # No changes
    fi

    # Check if .new files differ from current files
    local -- __puml_changed=false
    local -- __gv_changed=false

    if [[ -f "${DIAGRAMS_DIR}/classes.puml.new" ]]; then
        if [[ ! -f "${DIAGRAMS_DIR}/classes.puml" ]]; then
            __puml_changed=true
        elif ! diff -q "${DIAGRAMS_DIR}/classes.puml" "${DIAGRAMS_DIR}/classes.puml.new" &>/dev/null; then
            __puml_changed=true
        fi
    fi

    if [[ -f "${DIAGRAMS_DIR}/callgraph.gv.new" ]]; then
        if [[ ! -f "${DIAGRAMS_DIR}/callgraph.gv" ]]; then
            __gv_changed=true
        elif ! _byte-freq-match "${DIAGRAMS_DIR}/callgraph.gv" "${DIAGRAMS_DIR}/callgraph.gv.new"; then
            __gv_changed=true
        fi
    fi

    ${__puml_changed} || ${__gv_changed}
}

function _byte-freq-match() {
    :  'Compare two files by byte frequency (for non-deterministic files)'
    local -- __f1="${1}" __f2="${2}"
    local -- __h1 __h2
    __h1=$(od -An -tu1 -w1 "${__f1}" | sort -n | uniq -c)
    __h2=$(od -An -tu1 -w1 "${__f2}" | sort -n | uniq -c)
    [[ "${__h1}" == "${__h2}" ]]
}

function _handle-strict-review() {
    :  'Handle strict review mode checks

        @return
            0 if review passed, 1 if commit should be rejected
    '
    # First check: do we have a tracking file?
    if [[ ! -f "${TRACKING_FILE}" ]]; then
        # No tracking file - initialize it if diagrams exist
        if [[ -f "${DIAGRAMS_DIR}/classes.puml" ]]; then
            _compute-fingerprints
            _write-tracking
            info "Initialized tracking file (first run)"
            return 0
        fi
    fi

    # Check if sources match tracking (meaning no review needed)
    if _sources-match-tracking; then
        success "Diagram sources match verified state"
        return 0
    fi

    # Sources changed - check if architecture.md is staged
    if _is-architecture-staged; then
        info "Architecture changes detected and architecture.md is staged"
        _compute-fingerprints
        _write-tracking
        success "Tracking file updated"
        return 0
    fi

    # Generate review context for the agent
    # Note: At this point, the generate script has already run and committed the changes
    # So we compare the current file against what git has (before staging)
    local -- __old_puml
    __old_puml=$(mktemp)
    if git show HEAD:"docs/diagrams/generated/classes.puml" > "${__old_puml}" 2>/dev/null; then
        _generate-review-context "${__old_puml}" "${DIAGRAMS_DIR}/classes.puml"
    else
        echo ""
        warn "New class diagram detected. Please review and update architecture.md."
        echo ""
    fi
    rm -f "${__old_puml}"

    error "Commit rejected: architecture.md must be staged when diagrams change"
    echo ""
    echo "To bypass strict review mode for this commit:"
    echo "  git commit --no-verify"
    echo ""
    echo "To disable strict review mode:"
    echo "  git config docs.strictReview false"
    echo ""

    return 1
}


## main ########################################################################
################################################################################

function main() {
    local -- __staged_go_files
    local -- __strict_mode

    # Check if any .go files are staged
    __staged_go_files=$(git diff --cached --name-only --diff-filter=ACMR | grep '\.go$' || true)

    if [[ -z "${__staged_go_files}" ]]; then
        # No Go files staged, skip diagram generation
        return 0
    fi

    __strict_mode=$(_is-strict-mode && echo "true" || echo "false")

    info "Go files changed, regenerating diagrams..."

    # Regenerate diagrams (script is smart - skips SVG if sources unchanged)
    if ! "${GENERATE_SCRIPT}"; then
        warn "Failed to regenerate diagrams (continuing anyway)"
        return 0
    fi

    # Handle strict review mode
    if [[ "${__strict_mode}" == "true" ]]; then
        if ! _handle-strict-review; then
            # Clean up temp files before rejecting
            rm -f "${DIAGRAMS_DIR}"/*.new 2>/dev/null || true
            return 1
        fi
    fi

    # Stage the regenerated/updated diagrams
    if [[ -d "${DIAGRAMS_DIR}" ]]; then
        git add "${DIAGRAMS_DIR}"/*.svg "${DIAGRAMS_DIR}"/*.puml "${DIAGRAMS_DIR}"/*.gv 2>/dev/null || true
        # Also stage tracking file if it exists
        [[ -f "${TRACKING_FILE}" ]] && git add "${TRACKING_FILE}" 2>/dev/null || true
        success "Diagrams staged"
    fi

    return 0
}


## run #########################################################################
################################################################################

main "${@}"
