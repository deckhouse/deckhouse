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
