#!/bin/bash

hook::pre-build() {
    target="${1:-${MAKE_RULES_ROOT}/hooks}"
    hook::__run pre-build ${target}
}

hook::post-build() {
    target="${1:-${MAKE_RULES_ROOT}/hooks}"
    hook::__run post-build ${target}
}

hook::__run() {
    phase=${1}
    target=${2}

    if [[ -f ${target}/${phase} ]]; then
        log::status "Invoke ${phase} hook - ${target}/${phase}"
        source ${target}/${phase}
    fi
}
