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

#include <getopt.h>
#include <errno.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

#include "wrapper.h"

static const char USAGE[] =
  "Wrapper for legacy power commands to invoke shutdown via logind\n"
  "to send shutdown signal to all processes that hold inhibitor locks.\n"
  "It translates legacy commands into:\n"
  "systemctl halt|poweroff|reboot -i.\n"
  "\n"
  "Create symlink with alias to invoke systemctl:\n"
  "\n"
  "reboot                   Shut down and reboot the system\n"
  "poweroff                 Shut down and power-off the system\n"
  "shutdown                 Shut down and power-off the system\n"
  "halt                     Shut down and halt the system\n"
  "\n"
  "Options:\n"
  "          --dry-run      Print systemctl command line, not run it.\n"
  "   -r     --reboot       shutdown command compatibility: reboot.\n"
  "   -P, -p --poweroff     halt command compatibility: poweroff.\n"
  "   -H, -h --halt         poweroff command compatibility: halt.\n"
  "\n"
  "Other legacy options are silently ignored\n";

// Strings to build systemctl command.
static const char SYSTEMCTL[] = "systemctl";

static const char CMD_HALT[] = "halt";
static const char CMD_REBOOT[] = "reboot";
static const char CMD_POWEROFF[] = "poweroff";

static const char IGNORE_INHIBITORS_FLAG[] = "-i";


int main(int argc, char* argv[]) {
  int r = detect_action(argc, argv);
  if (r < 0 || arg_help == 1) {
    printf(USAGE);
    return 1;
  }

  char const * systemctl_action = NULL;
  switch (arg_action) {
  case ACTION_HALT:
    systemctl_action = CMD_HALT;
    break;
  case ACTION_POWEROFF:
    systemctl_action = CMD_POWEROFF;
    break;
  case ACTION_REBOOT:
    systemctl_action = CMD_REBOOT;
    break;
  default:
    printf(USAGE);
    return 2;
  }

  // Prepare args for exec. First item "echo" is needed for dry run invocation.
  char *const systemctl_args[] = {
    (char*)SYSTEMCTL,
    (char*)systemctl_action,
    (char*)IGNORE_INHIBITORS_FLAG,
    NULL,
  };

  errno = 0;

  // Print systemctl command if dry run is requested.
  if (arg_dry_run) {
    printf("Cmd:");
    for (int i=0; systemctl_args[i]; i++) {
      printf(" %s", systemctl_args[i]);
    }
    printf("\n");
    return 0;
  }

  // Exec systemctl with the detected power command and the additional flag.
  int ret_n = execvp(SYSTEMCTL, systemctl_args);
  if (ret_n != 0) {
    perror("exec systemctl");
    return 1;
  }

  return 0;
}

enum action arg_action;
int arg_dry_run;
int arg_help;

int detect_action(int argc, char *argv[]) {
  if (run_with_alias(argv, "halt")) {
    arg_action = ACTION_HALT;
    return parse_argv(argc, argv);
  } else if (run_with_alias(argv, "poweroff")) {
    arg_action = ACTION_POWEROFF;
    return parse_argv(argc, argv);
  } else if (run_with_alias(argv, "reboot")) {
    arg_action = ACTION_REBOOT;
    return parse_argv(argc, argv);
  } else if (run_with_alias(argv, "shutdown")) {
    arg_action = ACTION_POWEROFF;
    return parse_argv(argc, argv);
  }
  arg_help = 1;
  return 0;
}

int parse_argv(int argc, char *argv[]) {
  enum {
    ARG_HELP = 0x100,
    ARG_HALT,
    ARG_REBOOT,
    ARG_POWEROFF,
    ARG_NO_WALL,
    ARG_DRY_RUN,
  };

  static const struct option options[] = {
    { "help",      no_argument,       NULL, ARG_HELP     },
    { "halt",      no_argument,       NULL, ARG_HALT     },
    { "poweroff",  no_argument,       NULL, ARG_POWEROFF },
    { "reboot",    no_argument,       NULL, ARG_REBOOT   },
    { "force",     no_argument,       NULL, 'f'   },
    { "wtmp-only", no_argument,       NULL, 'w'   },
    { "no-wtmp",   no_argument,       NULL, 'd'   },
    { "no-sync",   no_argument,       NULL, 'n'   },
    { "no-wall",   no_argument,       NULL, ARG_NO_WALL },
    { "dry-run",   no_argument,       NULL, ARG_DRY_RUN },
    {}
  };

  int c;

  while ((c = getopt_long(argc, argv, "HhPprdinwacFfKkt:", options, NULL)) >= 0)
    switch(c) {
    case ARG_HELP:
      arg_help = 1;
      return 0;

    case ARG_DRY_RUN:
      arg_dry_run = 1;
      break;

    case ARG_HALT:
    case 'H':
      arg_action = ACTION_HALT;
      break;

    case 'h':
      if (arg_action != ACTION_HALT)
        arg_action = ACTION_POWEROFF;
      break;

    case ARG_POWEROFF:
    case 'P':
      arg_action = ACTION_POWEROFF;
      break;

    case 'p':
      if (arg_action != ACTION_REBOOT)
        arg_action = ACTION_POWEROFF;
      break;

    case ARG_REBOOT:
    case 'r':
      arg_action = ACTION_REBOOT;
      break;

    // Ignore poweroff/reboot/halt specific flags.
    case 'd':
    case 'i':
    case 'n':
    case 'w':
    // Ignore shutdown specific flags.
    case 'a':
    case 'c':
    case 'F':
    case 'K':
    case 'k':
    case 't': /* Note that we also ignore any passed argument to -t, not just the -t itself */
    // Ignore common flags.
    case 'f':
    case ARG_NO_WALL:
      /* Compatibility nops */
      break;

    case '?':
      return -EINVAL;

    default:
      printf("Unknown option %o\n", c);
    }

  return 0;
}

bool run_with_alias(char *argv[], const char *alias) {
  if (!argv || !alias) {
    return false;
  }

  // Find / from the end and increase pointer to get string after /
  // Use full argv[0] if there is no /.
  char *last_path = strrchr(argv[0], '/');
  if (last_path) {
    last_path++;
  } else {
    last_path = argv[0];
  }

  int cmp = strcmp(last_path, alias);

  return cmp == 0 ? true : false;
}
