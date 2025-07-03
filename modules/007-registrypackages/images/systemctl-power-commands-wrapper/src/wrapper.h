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

#ifndef WRAPPER_H
#define WRAPPER_H

#include <stdbool.h>

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

static const char SYSTEMCTL[] = "systemctl";

static const char CMD_HALT[] = "halt";
static const char CMD_REBOOT[] = "reboot";
static const char CMD_POWEROFF[] = "poweroff";

static const char IGNORE_INHIBITORS_FLAG[] = "-i";

enum action {
  ACTION_HALT,
  ACTION_POWEROFF,
  ACTION_REBOOT,
};

extern enum action arg_action;
extern int arg_dry_run;
extern int arg_help;

int parse_argv(int argc, char *argv[]);
int detect_action(int argc, char *argv[]);
bool run_with_alias(char *argv[], const char *alias);

#endif
