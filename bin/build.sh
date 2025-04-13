#!/bin/bash
# set -e

stty_settings=$(stty -g)  # Save term settings
disable_input() {
  stty -echo -icanon
}
restore_input() {
  stty "$stty_settings"
}
trap restore_input EXIT  # Restore on script exit

#------------------------------------------------------------------------------
# Color palette
#------------------------------------------------------------------------------
RESET="\033[0m"
BOLD="\033[1m"
BLUE="\033[34m"
GRAY="\033[90m"
BLACK="\033[30m"
WHITE="\033[97m"
RED="\033[31m"
GREEN="\033[32m"
AMBER="\033[33m"
CYAN="\033[96m"
MAGENTA="\033[95m"
BG_WHITE="\033[107m"
BG_BLUE="\033[44m"
BG_GRAY="\033[100m"
BG_RED="\033[41m"
BG_AMBER="\033[43m"
BG_GREEN="\033[42m"

#------------------------------------------------------------------------------
# Utility functions
#------------------------------------------------------------------------------
create_header() {
  local TITLE="$1"
  local COLOR="$2"

  printf "\n${COLOR}  ${TITLE}  ${RESET}\n"
  printf "${COLOR}┌───────────────────────────────────────────────────────────┐${RESET}\n"
}

create_footer() {
  local COLOR="$1"
  printf "${COLOR}└───────────────────────────────────────────────────────────┘${RESET}\n\n"
}

show_progress() {
  local COLOR="$1"
  local MSG="$2"
  local DELAY="${3:-0.01}"
  local BAR_SIZE=59

  # Print message on one line
  printf "  ${MSG}\n"

  # Progress bar animation on the line below
  for ((i=0; i<=BAR_SIZE; i++)); do
    # Calculate percentage
    percent=$((i*100/BAR_SIZE))

    # Create the bar with ─ for filled and spaces for empty
    bar="├"
    for ((j=0; j<i; j++)); do bar+="─"; done
    for ((j=i; j<BAR_SIZE; j++)); do bar+=" "; done
    bar+="┤"

    # Print the progress bar and percentage
    printf "\r${COLOR}${bar}${RESET}"
    sleep $DELAY
  done
  printf "\n"
}

#------------------------------------------------------------------------------
# Setup environment variables
#------------------------------------------------------------------------------
setup_environment() {
  # Get the root directory of the project
  PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

  # Load environment variables from .env file if it exists
  ENV_FILE="${PROJECT_ROOT}/.env"
  if [ -f "$ENV_FILE" ]; then
    source "$ENV_FILE"
  fi

  # Set environment variables with fallbacks to defaults
  export ENV_NAME="${ENV_NAME:-AUDIO ENGINE}"
  export ENV_SHORT_NAME="${ENV_SHORT_NAME:-app}"
  export ENV_BUILD_DIR="${ENV_BUILD_DIR:-${PROJECT_ROOT}/build}"

  # Set the build metadata
  BUILD_NAME="${ENV_SHORT_NAME}"
  BUILD_VERSION="${BUILD_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'v0.0.0')}"
  BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  BUILD_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

  # Create build directory if it doesn't exist
  mkdir -p "${ENV_BUILD_DIR}"
}

#------------------------------------------------------------------------------
# Help display function
#------------------------------------------------------------------------------
display_help() {
  create_header "Usage" "${GRAY}"
  printf "  ./bin/build.sh [--help]\n"
  create_footer "${GRAY}"

  create_header "ENVIRONMENT VARIABLES" "${GRAY}"
  printf "  Vars can be set in a .env file or in the environment\n\n"

  printf "  ${GRAY}ENV_NAME${RESET}\n"
  printf "    Name of the application used in displays.\n"
  printf "    Default: AUDIO ENGINE\n\n"

  printf "  ${GRAY}ENV_CGO_PATH${RESET}\n"
  printf "    Path to the C files used by CGO.\n"
  printf "    Default: PROJECT_ROOT/cmem\n\n"

  printf "  ${GRAY}ENV_BUILD_DIR${RESET}\n"
  printf "    Directory where build artifacts will be placed.\n"
  printf "    Default: PROJECT_ROOT/build\n\n"

  printf "  ${GRAY}ENV_SHORT_NAME${RESET}\n"
  printf "    Name of the compiled binary.\n"
  printf "    Default: aeng\n"
  create_footer "${GRAY}"

  exit 0
}

#------------------------------------------------------------------------------
# Display configuration
#------------------------------------------------------------------------------
show_configuration() {
  create_header "Configuration" "${CYAN}"
  printf "  %-10s ${CYAN}│${RESET} %s\n" "Name" "${ENV_NAME}"
  printf "  %-10s ${CYAN}│${RESET} %s\n" "Cmd" "${ENV_SHORT_NAME}"
  printf "  %-10s ${CYAN}│${RESET} %s\n" "Root" "${PROJECT_ROOT}"
  printf "  %-10s ${CYAN}│${RESET} %s\n" "Output" "${ENV_BUILD_DIR}"
  create_footer "${CYAN}"
}

#------------------------------------------------------------------------------
# Build the application
#------------------------------------------------------------------------------
build_app() {
  create_header "Building" "${RED}"
  show_progress "${RED}" "Compiling"

  # Build command (silent)
  go build -o "${ENV_BUILD_DIR}/${ENV_SHORT_NAME}" \
    -ldflags "\
      -X audio/internal/build.buildName=${BUILD_NAME} \
      -X audio/internal/build.buildTime=${BUILD_TIME} \
      -X audio/internal/build.buildCommit=${BUILD_COMMIT} \
      -X audio/internal/build.buildVersion=${BUILD_VERSION}" \
    main.go > /dev/null 2>&1

  # Completion and binary size
  SIZE=$(du -h "${ENV_BUILD_DIR}/${ENV_SHORT_NAME}" | cut -f1)
  printf "  Binary size: ${BOLD}${SIZE}${RESET}\n"
  printf "\n  ${RED}✓ Build completed successfully${RESET}\n"
  create_footer "${RED}"
}

#------------------------------------------------------------------------------
# Run tests
#------------------------------------------------------------------------------
run_tests() {
  create_header "Testing" "${AMBER}"

  show_progress "${AMBER}" "Unit Tests"
  TEST_OUTPUT=$(go test -v ./... 2>&1)
  TEST_RESULT=$?

  while IFS= read -r line; do
    if [[ "$line" == *"PASS"* ]]; then
      printf "  ${AMBER}✓${RESET} %s\n" "$line"
    elif [[ "$line" == *"FAIL"* ]]; then
      printf "  ${RED}✗${RESET} %s\n" "$line"
    else
      printf "  %s\n" "$line"
    fi
  done <<< "$TEST_OUTPUT"

  if [ $TEST_RESULT -eq 0 ]; then
    printf "\n  ${AMBER}✓ All tests passed${RESET}\n"
  else
    printf "\n  ${RED}✗ Some tests failed${RESET}\n"
  fi

  create_footer "${AMBER}"

  return $TEST_RESULT
}

#------------------------------------------------------------------------------
# Show summary
#------------------------------------------------------------------------------
show_summary() {
  local TEST_RESULT=$1

  create_header "Summary" "${GREEN}"
  printf "  ${GREEN}✓ Build completed ${RESET}\n"

  if [ $TEST_RESULT -eq 0 ]; then
    printf "  ${GREEN}✓ All tests passed${RESET}\n"
  else
    printf "  ${RED}✗ Some tests failed${RESET}\n"
  fi

  printf "\n  Run the app:  ${GREEN}./build/${ENV_SHORT_NAME}${RESET}\n"
  create_footer "${GREEN}"
}

#------------------------------------------------------------------------------
# Main function
#------------------------------------------------------------------------------
main() {
  disable_input
  setup_environment

  # Help?
  if [[ "$1" == "--help" ]]; then
    clear
    display_help
  fi

  cd "${PROJECT_ROOT}"
  clear
  show_configuration

  build_app

  run_tests
  TEST_RESULT=$?
  show_summary $TEST_RESULT

  restore_input
}

# Execute main with all args
main "$@"
