#!/bin/bash
# Fake copilot: prints TEST_LIST for "plugin marketplace list" and logs any other call's args to TEST_LOG.
if [[ "$*" == "plugin marketplace list" ]]; then
  printf '%s\n' "$TEST_LIST"
  exit
fi
echo "$*" >> "$TEST_LOG"
