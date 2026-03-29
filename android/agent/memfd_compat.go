package main

// memfd_compat.go — Intercept libc memfd_create to prevent SIGSYS on older Android.
//
// Android 8.x and below block memfd_create (syscall 422 on ARM) via seccomp.
// Go's runtime uses memfd_create for memory allocation and checks for ENOSYS
// to fall back to mmap. The --wrap linker flag redirects all libc-level calls
// to memfd_create through our wrapper which returns ENOSYS immediately.
//
// This works together with the SIGSYS signal handler in main.go which catches
// any raw syscall invocations that bypass libc.

// #cgo LDFLAGS: -Wl,--wrap=memfd_create
//
// #include <errno.h>
// #include <sys/types.h>
//
// // Called instead of the real memfd_create (via --wrap linker flag)
// int __wrap_memfd_create(const char *name, unsigned int flags) {
//     errno = ENOSYS;
//     return -1;
// }
import "C"
