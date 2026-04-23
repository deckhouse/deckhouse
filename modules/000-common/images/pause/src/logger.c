#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <spawn.h>

extern char **environ;

static int is_compiler(const char *filename) {
    if (!filename) return 0;
    return strstr(filename, "gcc") ||
           strstr(filename, "g++") ||
           strstr(filename, "clang") ||
           strstr(filename, "clang++");
}

static void log_argv(const char *tag, const char *file, char *const argv[]) {
    if (!is_compiler(file)) return;

    fprintf(stderr, "\n[%s][PID %d] %s\n", tag, getpid(), file);
    for (int i = 0; argv && argv[i]; i++) {
        fprintf(stderr, "  argv[%d]: %s\n", i, argv[i]);
    }
    fprintf(stderr, "\n");
    fflush(stderr);
}

/* ================= execve ================= */

typedef int (*execve_t)(const char *, char *const[], char *const[]);

int execve(const char *filename, char *const argv[], char *const envp[]) {
    static execve_t real = NULL;
    if (!real) real = dlsym(RTLD_NEXT, "execve");

    log_argv("execve", filename, argv);
    return real(filename, argv, envp);
}

/* ================= execvp ================= */

typedef int (*execvp_t)(const char *, char *const[]);

int execvp(const char *file, char *const argv[]) {
    static execvp_t real = NULL;
    if (!real) real = dlsym(RTLD_NEXT, "execvp");

    log_argv("execvp", file, argv);
    return real(file, argv);
}

/* ================= execvpe (glibc) ================= */

typedef int (*execvpe_t)(const char *, char *const[], char *const[]);

int execvpe(const char *file, char *const argv[], char *const envp[]) {
    static execvpe_t real = NULL;
    if (!real) real = dlsym(RTLD_NEXT, "execvpe");

    log_argv("execvpe", file, argv);
    return real(file, argv, envp);
}

/* ================= posix_spawn ================= */

typedef int (*posix_spawn_t)(pid_t *, const char *,
                            const posix_spawn_file_actions_t *,
                            const posix_spawnattr_t *,
                            char *const[], char *const[]);

int posix_spawn(pid_t *pid, const char *path,
                const posix_spawn_file_actions_t *file_actions,
                const posix_spawnattr_t *attrp,
                char *const argv[], char *const envp[]) {

    static posix_spawn_t real = NULL;
    if (!real) real = dlsym(RTLD_NEXT, "posix_spawn");

    log_argv("spawn", path, argv);
    return real(pid, path, file_actions, attrp, argv, envp);
}

/* ================= posix_spawnp ================= */

typedef int (*posix_spawnp_t)(pid_t *, const char *,
                             const posix_spawn_file_actions_t *,
                             const posix_spawnattr_t *,
                             char *const[], char *const[]);

int posix_spawnp(pid_t *pid, const char *file,
                 const posix_spawn_file_actions_t *file_actions,
                 const posix_spawnattr_t *attrp,
                 char *const argv[], char *const envp[]) {

    static posix_spawnp_t real = NULL;
    if (!real) real = dlsym(RTLD_NEXT, "posix_spawnp");

    log_argv("spawnp", file, argv);
    return real(pid, file, file_actions, attrp, argv, envp);
}
