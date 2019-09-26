#!/bin/bash

# Exit on error. Append "|| true" if you expect an error.
set -o errexit
# Do not allow use of undefined vars. Use ${VAR:-} to use an undefined VAR
set -o nounset
# Catch the error in pipeline.
set -o pipefail

# The root of the build/dist directory
# make-rules should be palced in project/hack/
# so the project root is project/hack/make-rules/lib/../../../
MAKE_RULES_ROOT="$(cd "$(dirname "${BASH_SOURCE}")/.." && pwd -P)"
REPO_ROOT="$(cd "${MAKE_RULES_ROOT}/../.." && pwd -P)"
# REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE}")/../../.." && pwd -P)"
REPO_CMDPATH="${REPO_ROOT}/cmd"
REPO_OUTPUT_BINPATH="${REPO_ROOT}/bin"

source "${MAKE_RULES_ROOT}/lib/util.sh"
source "${MAKE_RULES_ROOT}/lib/logging.sh"

log::install_errexit

source "${MAKE_RULES_ROOT}/lib/hook.sh"
source "${MAKE_RULES_ROOT}/lib/version.sh"
source "${MAKE_RULES_ROOT}/lib/lang/golang.sh"
source "${MAKE_RULES_ROOT}/lib/docker.sh"
