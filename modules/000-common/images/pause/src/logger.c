#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

typedef int (*execve_t)(const char *, char *const[], char *const[]);

int execve(const char *filename, char *const argv[], char *const envp[]) {
    static execve_t real_execve = NULL;
    if (!real_execve) {
        real_execve = (execve_t)dlsym(RTLD_NEXT, "execve");
    }

    if (filename) {
        if (strstr(filename, "gcc") || strstr(filename, "g++") ||
            strstr(filename, "clang") || strstr(filename, "clang++")) {

            fprintf(stderr, "\n[COMPILER INVOCATION]\n");
            fprintf(stderr, "%s\n", filename);

            for (int i = 0; argv[i]; i++) {
                fprintf(stderr, "argv[%d]: %s\n", i, argv[i]);
            }

            fprintf(stderr, "\n");
            fflush(stderr);
        }
    }

    return real_execve(filename, argv, envp);
}
