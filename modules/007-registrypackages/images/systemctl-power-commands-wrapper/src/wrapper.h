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

static const char USAGE[] =
  "Wrapper to run systemctl with additional flag --check-inhibitors=yes.\n"
  "\n"
  "Direct invocation just prints this help.\n"
  "Create symlink with alias to invoke systemctl power command:\n"
  "\n"
  "reboot                   Shut down and reboot the system\n"
  "poweroff                 Shut down and power-off the system\n"
  "shutdown                 Shut down and power-off the system\n"
  "halt                     Shut down and halt the system\n"
  "\n"
  "Options:\n"
  "      --dry-run          Print systemctl command line, not run it.\n";


static const char ECHO[] = "echo";
static const char DRY_RUN[] = "--dry-run";

static const char SYSTEMCTL[] = "systemctl";

static const char ACTION_HALT[] = "halt";
static const char ACTION_REBOOT[] = "reboot";
static const char ACTION_POWEROFF[] = "poweroff";
static const char ACTION_SHUTDOWN[] = "shutdown";

static const char* KNOWN_ACTIONS[] = {
    ACTION_HALT,
    ACTION_REBOOT,
    ACTION_POWEROFF,
    ACTION_SHUTDOWN,
};
static const int KNOWN_ACTIONS_COUNT = (int)(sizeof(KNOWN_ACTIONS)/sizeof(KNOWN_ACTIONS[0]));

static const char CHECK_INHIBITORS_FLAG[] = "--check-inhibitors=yes";



int detect_action(char const* argv0, char const **action_name);

int detect_dry_run(int argc, char ** argv);

#endif
