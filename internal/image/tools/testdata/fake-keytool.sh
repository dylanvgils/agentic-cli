#!/bin/bash
# Fake keytool: logs its args, and stdin unless given -file, to TEST_LOG; fails when TEST_KEYTOOL_FAIL is set.
[[ -z "${TEST_KEYTOOL_FAIL:-}" ]] || exit 1
echo "$*" >> "$TEST_LOG"
[[ " $* " == *" -file "* ]] || cat >> "$TEST_LOG"
