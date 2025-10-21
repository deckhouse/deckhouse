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
package main

/*
#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <fcntl.h>
#include <sched.h>
#include <errno.h>
#include <string.h>
#include <limits.h>

__attribute__((constructor)) void enter_namespace(void) {
    const char *target_pid = getenv("TARGET_PID");
    if (!target_pid) {
        fprintf(stderr, "TARGET_PID not set in environment\n");
        exit(1);
    }

    char ns_path[PATH_MAX];
    snprintf(ns_path, sizeof(ns_path), "/proc/%s/ns/mnt", target_pid);
    
    int fd = open(ns_path, O_RDONLY);
    if (fd == -1) {
        fprintf(stderr, "Failed to open %s: %s\n", ns_path, strerror(errno));
        exit(1);
    }

    if (setns(fd, CLONE_NEWNS) == -1) {
        fprintf(stderr, "setns failed: %s\n", strerror(errno));
        close(fd);
        exit(1);
    }

    close(fd);
}
*/
import "C"

import (
	"fmt"
	"log"
	"os"
	"golang.org/x/sys/unix"
)

func main() {
    if len(os.Args) != 2 {
        log.Fatalf("Usage: %s <mount path>", os.Args[0])
    }

    targetPid := os.Getenv("TARGET_PID")
    mountPath := os.Args[1]
    if err := unix.Unmount(mountPath, 0); err != nil {
        log.Fatalf("Failed to unmount %s: %v", mountPath, err)
    }

    fmt.Printf("Successfully unmounted %s in namespace of PID %s\n", mountPath, targetPid)
}
