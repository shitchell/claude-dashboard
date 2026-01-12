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

        Reads diff from stdin, outputs class names (one per line).
        Tracks context to detect method/field changes within class blocks.
    '
    local -- __line __current_class=""
    local -a __classes=()

    while IFS= read -r __line; do
        # Track which class block we're in (matches both changed and context lines)
        # Format: class "package.ClassName" or class "ClassName"
        if [[ "${__line}" =~ class[[:space:]]+\"([^\"]+)\" ]]; then
            __current_class="${BASH_REMATCH[1]}"
        fi

        # Detect end of class block (closing brace at class indentation level)
        # PlantUML class blocks end with "    }" (4 spaces + brace)
        if [[ "${__line}" =~ ^[[:space:]]{4}\}[[:space:]]*$ ]]; then
            __current_class=""
        fi

        # New class added/removed - the class definition line itself changed
        if [[ "${__line}" =~ ^[+-][[:space:]]*class[[:space:]]+\"([^\"]+)\" ]]; then
            __classes+=("${BASH_REMATCH[1]}")
        fi

        # Method or field added/removed within a class context
        # Format: +/- followed by spaces, then +/- (public/private) and identifier
        # e.g.: "+        + SpawnClaude(...)" or "+        - cleanupFuncs ..."
        if [[ -n "${__current_class}" ]]; then
            if [[ "${__line}" =~ ^[+-][[:space:]]+[\+\-][[:space:]]+[A-Za-z_] ]]; then
                __classes+=("${__current_class}")
            fi
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

function _find-gopls() {
    :  'Find gopls binary, checking common locations

        Outputs the path to gopls if found, empty string otherwise.
    '
    local -- __gopls

    # Check PATH first
    __gopls=$(command -v gopls 2>/dev/null)
    if [[ -n "${__gopls}" ]]; then
        echo "${__gopls}"
        return 0
    fi

    # Check GOPATH/bin
    local -- __gopath
    __gopath=$(go env GOPATH 2>/dev/null)
    if [[ -n "${__gopath}" ]] && [[ -x "${__gopath}/bin/gopls" ]]; then
        echo "${__gopath}/bin/gopls"
        return 0
    fi

    # Check ~/go/bin (default GOPATH)
    if [[ -x "${HOME}/go/bin/gopls" ]]; then
        echo "${HOME}/go/bin/gopls"
        return 0
    fi

    return 1
}

function _find-symbol-location() {
    :  'Find the file:line location of a Go type/struct by name

        @arg symbol_name
            The name of the type to find (e.g., "TestHarness", "ClaudeInstance")

        Outputs file:line:column if found, empty otherwise.
    '
    local -- __symbol="${1}"
    local -- __match

    # Search for type definition: "type SymbolName struct"
    __match=$(grep -rn "^type ${__symbol} struct" --include="*.go" "${REPO_ROOT}" 2>/dev/null | head -1)

    if [[ -n "${__match}" ]]; then
        # Format: file.go:123:type SymbolName struct
        local -- __file __line
        __file=$(echo "${__match}" | cut -d: -f1)
        __line=$(echo "${__match}" | cut -d: -f2)
        # Column is position of symbol name in the line
        local -- __col
        __col=$(echo "${__match}" | cut -d: -f3- | grep -bo "${__symbol}" | head -1 | cut -d: -f1)
        __col=$(( __col + 1 ))  # 1-indexed
        echo "${__file}:${__line}:${__col}"
        return 0
    fi

    return 1
}

function _get-gopls-references() {
    :  'Get references to a symbol using gopls

        @arg gopls_path
            Path to gopls binary
        @arg symbol_name
            Name of the symbol to find references for

        Outputs unique file:line references, one per line.
    '
    local -- __gopls="${1}"
    local -- __symbol="${2}"
    local -- __location

    # Find where the symbol is defined
    __location=$(_find-symbol-location "${__symbol}")
    if [[ -z "${__location}" ]]; then
        return 1
    fi

    # Get references via gopls
    # Output format: /path/to/file.go:line:col-col
    "${__gopls}" references "${__location}" 2>/dev/null | \
        cut -d: -f1-2 | \
        sort -u
}

function _extract-connected-classes-gopls() {
    :  'Extract connected class names with rich context using gopls references

        @arg class_names
            Newline-separated list of class names (stdin)

        Outputs structured connection data, one per line:
        FORMAT: CONNECTED_TYPE|SOURCE_CLASS|RELATIONSHIP|FILE:LINE|METHOD_NAME

        Example:
          TestHarness|ClaudeInstance|creates|harness.go:375|SpawnClaude
          gotmux.Pane|ClaudeInstance|field|harness.go:305|

        Falls back to simple class names if gopls unavailable (return code 1).
    '
    local -- __gopls __class_name __ref_file __ref_line
    local -a __connections=()
    local -a __changed_list=()

    # Try to find gopls
    __gopls=$(_find-gopls)

    if [[ -z "${__gopls}" ]]; then
        # No gopls available - return empty (caller should use grep fallback)
        return 1
    fi

    # Read class names from stdin into array
    while IFS= read -r __class_name; do
        [[ -z "${__class_name}" ]] && continue
        __changed_list+=("${__class_name}")
    done

    # For each changed class, find what references it AND what it references
    for __class_name in "${__changed_list[@]}"; do
        # 1. Find types that REFERENCE this class (what uses it)
        while IFS= read -r __ref; do
            [[ -z "${__ref}" ]] && continue

            __ref_file=$(echo "${__ref}" | cut -d: -f1)
            __ref_line=$(echo "${__ref}" | cut -d: -f2)

            # Get enclosing context (type, method, context_type)
            local -- __context __enc_type __enc_method __ctx_type
            __context=$(_find-enclosing-context "${__ref_file}" "${__ref_line}")
            __enc_type=$(echo "${__context}" | cut -d'|' -f1)
            __enc_method=$(echo "${__context}" | cut -d'|' -f2)
            __ctx_type=$(echo "${__context}" | cut -d'|' -f3)

            # Skip self-references and entries with no enclosing type
            [[ "${__enc_type}" == "${__class_name}" ]] && continue
            [[ -z "${__enc_type}" ]] && continue

            # Categorize the relationship
            local -- __category
            __category=$(_categorize-reference "${__ref_file}" "${__ref_line}" "${__class_name}")

            # Build location string (relative path)
            local -- __rel_file
            __rel_file="${__ref_file#${REPO_ROOT}/}"

            # Use the enclosing type as the connected name
            local -- __connected_name="${__enc_type}"

            # Output: CONNECTED|SOURCE|RELATIONSHIP|LOCATION|METHOD
            __connections+=("${__connected_name}|${__class_name}|${__category}|${__rel_file}:${__ref_line}|${__enc_method}")

        done < <(_get-gopls-references "${__gopls}" "${__class_name}")

        # 2. Find types that this class REFERENCES (field types)
        local -- __location __type_refs
        __location=$(_find-symbol-location "${__class_name}")
        if [[ -n "${__location}" ]]; then
            local -- __src_file="${__location%%:*}"
            local -- __src_line="${__location#*:}"
            __src_line="${__src_line%%:*}"
            local -- __rel_src="${__src_file#${REPO_ROOT}/}"

            __type_refs=$(_extract-type-references "${__src_file}" "${__class_name}")
            while IFS= read -r __type_ref; do
                [[ -z "${__type_ref}" ]] && continue

                # Skip if it's one of the changed classes
                local -- __is_changed=false
                for __c in "${__changed_list[@]}"; do
                    if [[ "${__type_ref}" == "${__c}" ]]; then
                        __is_changed=true
                        break
                    fi
                done

                if ! ${__is_changed}; then
                    # Output: CONNECTED|SOURCE|RELATIONSHIP|LOCATION|METHOD
                    __connections+=("${__type_ref}|${__class_name}|field|${__rel_src}:${__src_line}|")
                fi
            done <<< "${__type_refs}"
        fi
    done

    # Output all connections (caller will dedupe/format)
    printf '%s\n' "${__connections[@]}" 2>/dev/null | sort -u
}

function _extract-type-references() {
    :  'Extract type references from a struct definition

        @arg file_path
            Path to the Go file
        @arg struct_name
            Name of the struct to analyze

        Outputs referenced type names (one per line).
    '
    local -- __file="${1}"
    local -- __struct="${2}"
    local -a __types=()

    # Find the struct definition and extract field types
    # Go struct fields: "fieldName TypeName" or "fieldName *pkg.TypeName"
    # We want the type (after the field name), not the field name itself
    local -- __in_struct=false __line
    while IFS= read -r __line; do
        if [[ "${__line}" =~ ^type[[:space:]]+${__struct}[[:space:]]+struct ]]; then
            __in_struct=true
            continue
        fi
        if ${__in_struct}; then
            # End of struct
            if [[ "${__line}" =~ ^\} ]]; then
                break
            fi
            # Skip empty lines and comments
            [[ "${__line}" =~ ^[[:space:]]*$ ]] && continue
            [[ "${__line}" =~ ^[[:space:]]*/\* ]] && continue
            [[ "${__line}" =~ ^[[:space:]]*// ]] && continue

            # Extract field type - pattern: fieldName followed by type
            # Types we care about: *Pkg.Type, Pkg.Type, *Type, Type (capitalized)
            # Skip primitives: string, int, bool, etc.
            local -- __field_type

            # Match qualified types: *pkg.Type or pkg.Type
            __field_type=$(echo "${__line}" | grep -oP '\*?[a-z][a-z0-9_]*\.[A-Z][A-Za-z0-9_]*' | head -1)
            if [[ -n "${__field_type}" ]]; then
                __field_type="${__field_type#\*}"
                __types+=("${__field_type}")
                continue
            fi

            # Match unqualified pointer types: *TypeName (must be after field name)
            # Pattern: whitespace, fieldname, whitespace, *TypeName
            if [[ "${__line}" =~ [[:space:]][a-z][a-zA-Z0-9_]*[[:space:]]+\*([A-Z][A-Za-z0-9_]*) ]]; then
                __types+=("${BASH_REMATCH[1]}")
                continue
            fi

            # Match unqualified types that aren't primitives
            # Check for: fieldName TypeName where TypeName is capitalized
            if [[ "${__line}" =~ [[:space:]][a-z][a-zA-Z0-9_]*[[:space:]]+([A-Z][A-Za-z0-9_]*)[[:space:]]*$ ]]; then
                local -- __potential="${BASH_REMATCH[1]}"
                # Skip common non-type patterns
                case "${__potential}" in
                    String|Int|Bool|Float|Byte|Rune|Error) ;;  # Skip built-in type aliases
                    *) __types+=("${__potential}") ;;
                esac
            fi
        fi
    done < "${__file}"

    printf '%s\n' "${__types[@]}" 2>/dev/null | sort -u
}

function _find-enclosing-context() {
    :  'Find the enclosing type and method for a given line in a Go file

        @arg file_path
            Path to the Go file
        @arg line_number
            Line number to check

        Outputs: TYPE|METHOD|CONTEXT_TYPE
        Where CONTEXT_TYPE is: method, struct_field, function, or unknown
        Example: "TestHarness|SpawnClaude|method" or "ClaudeInstance||struct_field"
    '
    local -- __file="${1}"
    local -- __line="${2}"
    local -- __content __type_name __method_name __context_type

    # Read lines from start of file up to the target line, in reverse
    __content=$(head -n "${__line}" "${__file}" 2>/dev/null | tac)

    # First check for method receiver: func (x *TypeName) MethodName(...)
    local -- __method_line
    __method_line=$(echo "${__content}" | grep -m1 -P '^\s*func\s+\([^)]+\)\s+[A-Za-z_]')
    if [[ -n "${__method_line}" ]]; then
        # Extract type from receiver
        __type_name=$(echo "${__method_line}" | grep -oP '\(\s*\w+\s+\*?([A-Z][A-Za-z0-9_]*)' | grep -oP '[A-Z][A-Za-z0-9_]*$')
        # Extract method name
        __method_name=$(echo "${__method_line}" | grep -oP '\)\s+([A-Za-z_][A-Za-z0-9_]*)' | grep -oP '[A-Za-z_][A-Za-z0-9_]*$')
        echo "${__type_name}|${__method_name}|method"
        return 0
    fi

    # Check for type definition: type TypeName struct (we're inside a struct)
    local -- __struct_line
    __struct_line=$(echo "${__content}" | grep -m1 -P '^type\s+[A-Z][A-Za-z0-9_]*\s+struct')
    if [[ -n "${__struct_line}" ]]; then
        __type_name=$(echo "${__struct_line}" | awk '{print $2}')
        echo "${__type_name}||struct_field"
        return 0
    fi

    # Check for standalone function: func FunctionName(...)
    local -- __func_line
    __func_line=$(echo "${__content}" | grep -m1 -P '^\s*func\s+[A-Z][A-Za-z0-9_]*\s*\(')
    if [[ -n "${__func_line}" ]]; then
        __method_name=$(echo "${__func_line}" | grep -oP 'func\s+([A-Z][A-Za-z0-9_]*)' | awk '{print $2}')
        echo "|${__method_name}|function"
        return 0
    fi

    echo "||unknown"
    return 1
}

function _categorize-reference() {
    :  'Categorize how a type is referenced on a given line

        @arg file_path
            Path to the Go file
        @arg line_number
            Line number to check
        @arg type_name
            The type being referenced

        Outputs: CATEGORY
        Categories: creates, param, returns, field, uses
    '
    local -- __file="${1}"
    local -- __line_num="${2}"
    local -- __type_name="${3}"
    local -- __line_content

    __line_content=$(sed -n "${__line_num}p" "${__file}" 2>/dev/null)

    # Check for instantiation: &TypeName{ or TypeName{
    if [[ "${__line_content}" =~ \&?${__type_name}\{ ]]; then
        echo "creates"
        return 0
    fi

    # Check for return type in function signature: ) *TypeName or ) (... *TypeName
    if [[ "${__line_content}" =~ \)[[:space:]]*\*?${__type_name}[[:space:]]*\{ ]] || \
       [[ "${__line_content}" =~ \)[[:space:]]*\(.*\*?${__type_name} ]] || \
       [[ "${__line_content}" =~ return.*${__type_name} ]]; then
        echo "returns"
        return 0
    fi

    # Check for parameter in function signature
    if [[ "${__line_content}" =~ func.*\(.*\*?${__type_name} ]]; then
        echo "param"
        return 0
    fi

    # Check for field definition (inside struct)
    if [[ "${__line_content}" =~ ^[[:space:]]+[A-Za-z_][A-Za-z0-9_]*[[:space:]]+\*?${__type_name} ]] || \
       [[ "${__line_content}" =~ ^[[:space:]]+[A-Za-z_][A-Za-z0-9_]*[[:space:]]+\*?[a-z]+\.${__type_name} ]]; then
        echo "field"
        return 0
    fi

    # Default: general usage
    echo "uses"
    return 0
}

function _find-enclosing-type() {
    :  'Find the type/struct that encloses a given line (compatibility wrapper)

        @arg file_path
            Path to the Go file
        @arg line_number
            Line number to check

        Outputs the enclosing type name, or empty if not in a type context.
    '
    local -- __result
    __result=$(_find-enclosing-context "${1}" "${2}")
    echo "${__result}" | cut -d'|' -f1
}

function _format-rich-connections() {
    :  'Format rich connection data for display

        @arg connections
            Newline-separated connection data in format:
            CONNECTED_TYPE|SOURCE_CLASS|RELATIONSHIP|FILE:LINE|METHOD_NAME

        Outputs formatted, grouped connection information.
    '
    local -- __connections="${1}"
    local -- __current_class="" __line
    declare -A __seen_connections

    # Primitives and noise to filter out
    local -- __primitives="string|int|bool|error|byte|rune|float32|float64|uint|int32|int64|uint32|uint64|uintptr|method|struct_field|function|unknown"

    # Group by connected class and format
    while IFS='|' read -r __connected __source __rel __location __method; do
        [[ -z "${__connected}" ]] && continue
        [[ -z "${__rel}" ]] && continue

        # Skip primitives and context types
        if [[ "${__connected}" =~ ^(${__primitives})$ ]]; then
            continue
        fi

        # Skip if connected is one of the changed classes being referenced from itself
        [[ "${__connected}" == "${__source}" ]] && continue

        # Skip if connected looks like a method name (lowercase first char or camelCase without dot)
        # Valid connected types: TestHarness, gotmux.Pane, etc.
        # Invalid: SendCommand, WaitForNewSession (these are methods, not types)
        if [[ "${__connected}" =~ ^[a-z] ]] && [[ ! "${__connected}" =~ \. ]]; then
            continue
        fi

        # Create a unique key for deduplication
        local -- __key="${__connected}|${__source}|${__rel}|${__method}"
        if [[ -n "${__seen_connections[${__key}]+x}" ]]; then
            continue
        fi
        __seen_connections["${__key}"]=1

        # Print class header if new class
        if [[ "${__connected}" != "${__current_class}" ]]; then
            [[ -n "${__current_class}" ]] && echo ""  # Blank line between classes
            echo "  ${C_BOLD}${__connected}${C_RESET}"
            __current_class="${__connected}"
        fi

        # Format the relationship line
        local -- __rel_text
        case "${__rel}" in
            creates)  __rel_text="creates" ;;
            param)    __rel_text="takes param" ;;
            returns)  __rel_text="returns" ;;
            field)    __rel_text="used as field type in" ;;
            uses)     __rel_text="uses" ;;
            *)        __rel_text="${__rel}" ;;
        esac

        local -- __method_text=""
        if [[ -n "${__method}" ]]; then
            __method_text=" in ${__method}()"
        fi

        # Build location display (just filename:line)
        local -- __loc_display=""
        if [[ -n "${__location}" ]]; then
            __loc_display="${C_DIM}(${__location})${C_RESET}"
        fi

        # Print the relationship
        echo "    → ${__rel_text} ${__source}${__method_text} ${__loc_display}"

    done <<< "${__connections}"
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

    # Get connected classes - try gopls first, fall back to grep-based
    local -- __connected_classes="" __connection_method=""
    if [[ -n "${__changed_classes}" ]]; then
        __connected_classes=$(echo "${__changed_classes}" | _extract-connected-classes-gopls 2>/dev/null)
        if [[ -n "${__connected_classes}" ]]; then
            __connection_method="gopls"
        else
            # Fallback to grep-based extraction from PlantUML
            __connected_context=$(echo "${__changed_classes}" | _extract-connected-context "${__new_file}")
            __connection_method="grep"
        fi
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

    if [[ -n "${__connected_classes}" ]] && [[ "${__connection_method}" == "gopls" ]]; then
        echo "${C_INFO}Connected classes (via ${__connection_method}):${C_RESET}"
        _format-rich-connections "${__connected_classes}"
        echo ""
    elif [[ -n "${__connected_context}" ]]; then
        echo "${C_INFO}Connected relationships (via grep, may be incomplete):${C_RESET}"
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
