/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

#include <stdio.h>
#include <unistd.h>
#include <errno.h>
#include <string.h>
#include "wrapper.h"

// TODO
// - Extract last path to detect systemctl action.
// - Pass extra arguments from command line for --help and other functions.

int main(int argc, char** argv) {
  char const * systemctl_action = NULL;
  if (!detect_action(argv[0], &systemctl_action)) {
    printf(USAGE);
    return 1;
  }

  int dry_run = detect_dry_run(argc, argv);

  // Prepare args for exec. First item is needed for dry run invocation.
  char * systemctl_args[] = {
    (char*)ECHO,
    (char*)SYSTEMCTL,
    (char*)systemctl_action,
    (char*)CHECK_INHIBITORS_FLAG,
    NULL,
  };

  errno = 0;

  // Exec systemctl with the detected power command and the additional flag.
  char const *exec_file = SYSTEMCTL;
  char * const * args = &systemctl_args[1]; // Ignore "echo" item.
  if (dry_run) {
    // Or use echo to print systemctl command line if dry run is requested.
    exec_file = ECHO;
    args = systemctl_args;
  }

  int ret_n = execvp(exec_file, args);
  if (ret_n != 0) {
    perror("exec: ");
    return 1;
  }

  return 0;
}

// detect_action loops through all known actions and detects
// which action is requested in argv[0].
int detect_action(char const* argv0, const char **action_name) {
  char *pos = NULL;

  // Exception for shutdown: systemctl runs poweroff action if executed with alias "shutdown".
  if (strstr(argv0, ACTION_SHUTDOWN)) {
    *action_name = ACTION_POWEROFF;
    return 1;
  }

  // Detect other actions.
  for (int i=0; i < KNOWN_ACTIONS_COUNT; i++) {
    pos = strstr(argv0, KNOWN_ACTIONS[i]);
    if (pos) {
      *action_name = KNOWN_ACTIONS[i];
      return 1;
    }
  }

  // Unknown action.
  return 0;
}

// detect_dry_run return 0 if --dry-run flag is specified.
int detect_dry_run(int argc, char ** argv) {
  if (argc <= 1) { return 0; }

  char *pos = NULL;
  pos = strstr(argv[1], DRY_RUN);
  return pos ? 1 : 0;
}

