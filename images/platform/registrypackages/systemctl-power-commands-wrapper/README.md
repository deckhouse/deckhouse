# Wrapper to run systemctl power commands

## Why?

This is a complementary package for component that implements
the inhibit shutdown for stateful Pods feature.

We need reboot/poweroff/shutdown commands to use --check-inhibitors=yes flag for systemctl
to not ignore acquired inhibitor locks.

## Implementation

The are 2 parts:

1. Wrapper. It is a binary that checks argv[0] for command and then exec systemctl with the detected command and the --check-inhibitors=yes flag. 

2. Overrides for poweroff/shutdown/reboot commands. Override is made with symlinks to the wrapper in /usr/local/sbin directory. This path has higher priority than
/usr/sbin, so original commands from the distribution will not be used.

## Development

Makefile in `src` directory contains some useful commands:

Run `make` to build static binary with musl-libc.

Run `make dev-container` to start development container with gcc, musl libc. Run `cd /app` to see source files, run make to build, run ./wrapper for debugging.

Run `make clean` to remove binary and a dev-container.

