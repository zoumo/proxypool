#!/bin/bash

# ===============================================================
# This file was autogenerated. Do not edit it manually!
# ===============================================================

# Exit on error. Append "|| true" if you expect an error.
set -o errexit
# Do not allow use of undefined vars. Use ${VAR:-} to use an undefined VAR
set -o nounset
# Catch the error in pipeline.
set -o pipefail

MAKE_RULES_ROOT=$(dirname "${BASH_SOURCE}")
VERBOSE="${VERBOSE:-1}"
source "${MAKE_RULES_ROOT}/lib/init.sh"

entry::clean() {
    rm -rf ${REPO_OUTPUT_BINPATH}
}

entry::go::usage() {
    log::usage_from_stdin <<EOF
usage: $(basename $0) go <commands> [TARGETS]

Available Commands:
    build      build go package
    unittest   test all package in this project
EOF
}

entry::go::build() {
    hook::pre-build
    golang::build_binaries "$@"
    hook::post-build
}

entry::go::unittest() {
    golang::unittest "$@"
}

entry::go() {
    subcommand=${1-}
    case $subcommand in
    "" | "-h" | "--help")
        entry::go::usage
        ;;
    *)
        shift
        entry::go::${subcommand} $@
        ;;
    esac

}

entry::container::build() {
    docker::build_images "$@"
}

entry::container::push() {
    docker::push_images "$@"
}

entry::container::usage() {
    log::usage_from_stdin <<EOF
usage: $(basename $0) container <commands> [TARGETS]

Available Commands:
    build      build container image
    push       push container image to registries
EOF
}

entry::container() {
    subcommand=${1-}
    case $subcommand in
    "" | "-h" | "--help")
        entry::container::usage
        ;;
    *)
        shift
        entry::container::${subcommand} $@
        ;;
    esac
}

usage() {
    log::usage_from_stdin <<EOF
usage: caimake <commands> <subcommands>

Available Commands:
    clean      clean all build output
    go         building for golang
	container     building for container 
EOF
}

subcommand=${1-}
case $subcommand in
"" | "-h" | "--help")
    usage
    ;;
"clean")
    shift
    entry::clean $@
    ;;
"go")
    shift
    entry::go $@
    ;;
"container")
    shift
    entry::container $@
    ;;
*)
    usage
    ;;
esac
