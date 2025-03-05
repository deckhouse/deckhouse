#include <stdio.h>
#include <unistd.h>
#include <errno.h>
#include <string.h>
#include "wrapper.h"

int main(int argc, char** argv) {
  char const * systemctl_command = NULL;
  if (!detect_command(argv[0], &systemctl_command)) {
    printf(USAGE);
    return 1;
  }

  int dry_run = detect_dry_run(argc, argv);

  // Prepare args for exec. First item is needed for dry run invocation.
  char * systemctl_args[] = {
    (char*)ECHO,
    (char*)SYSTEMCTL,
    (char*)systemctl_command,
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

// detect_command loops through all known commands and detects
// which command is requested in argv[0].
int detect_command(char const* argv0, const char * *command_name) {
  char *pos = NULL;

  for (int i=0; i < KNOWN_COMMANDS_COUNT; i++) {
    pos = strstr(argv0, KNOWN_COMMANDS[i]);
    if (pos) {
      *command_name = KNOWN_COMMANDS[i];
      return 1;
    }
  }

  // Unknown command.
  return 0;
}

// detect_dry_run return 0 if --dry-run flag is specified.
int detect_dry_run(int argc, char ** argv) {
  if (argc <= 1) { return 0; }

  char *pos = NULL;
  pos = strstr(argv[1], DRY_RUN);
  return pos ? 1 : 0;
}

