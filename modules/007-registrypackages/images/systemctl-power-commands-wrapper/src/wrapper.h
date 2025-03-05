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

static const char REBOOT[] = "reboot";
static const char POWEROFF[] = "poweroff";
static const char SHUTDOWN[] = "shutdown";
static const char HALT[] = "halt";

static const char* KNOWN_COMMANDS[] = {
    REBOOT,
    POWEROFF,
    SHUTDOWN,
    HALT,
};
static const int KNOWN_COMMANDS_COUNT = (int)(sizeof(KNOWN_COMMANDS)/sizeof(KNOWN_COMMANDS[0]));

static const char CHECK_INHIBITORS_FLAG[] = "--check-inhibitors=yes";



int detect_command(char const* argv0, char const **command_name);

int detect_dry_run(int argc, char ** argv);

#endif
