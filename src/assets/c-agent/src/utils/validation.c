/*
 * NodePulse Agent - Input Validation Utilities Implementation
 */

#include "validation.h"
#include "safe_string.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>

#ifdef _WIN32
#define PATH_SEPARATOR '\\'
#define ALT_PATH_SEPARATOR '/'
#else
#define PATH_SEPARATOR '/'
#define ALT_PATH_SEPARATOR '\\'
#endif

/* Maximum reasonable path length */
#define MAX_PATH_LEN 4096

/* ============================================================================
 * Path Validation
 * ============================================================================ */

ValidationResult validate_path(const char* path, size_t max_len) {
    if (!path) {
        return VALIDATION_NULL_INPUT;
    }

    size_t len = safe_strlen(path, max_len + 1);
    if (len == 0) {
        return VALIDATION_EMPTY_INPUT;
    }

    if (len > max_len) {
        return VALIDATION_TOO_LONG;
    }

    /* Check for null bytes within string */
    for (size_t i = 0; i < len; i++) {
        if (path[i] == '\0') {
            break;
        }
    }

    /* Check for path traversal patterns */
    const char* p = path;
    while (*p) {
        /* Check for ".." followed by separator or end of string */
        if (p[0] == '.' && p[1] == '.') {
            char next = p[2];
            if (next == '\0' || next == '/' || next == '\\') {
                return VALIDATION_PATH_TRAVERSAL;
            }
        }
        p++;
    }

    /* Check for absolute paths that escape (Windows drive letters + Linux root) */
    /* This is informational - absolute paths may be allowed in some contexts */

    return VALIDATION_OK;
}

int sanitize_path(const char* path, char* output, size_t output_size) {
    if (!path || !output || output_size == 0) {
        return -1;
    }

    size_t out_pos = 0;
    const char* p = path;

    while (*p && out_pos < output_size - 1) {
        /* Skip ".." components */
        if (p[0] == '.' && p[1] == '.') {
            if (p[2] == '\0' || p[2] == '/' || p[2] == '\\') {
                /* Skip the ".." and following separator */
                p += 2;
                if (*p == '/' || *p == '\\') {
                    p++;
                }
                continue;
            }
        }

        /* Copy character */
        output[out_pos++] = *p++;
    }

    output[out_pos] = '\0';
    return 0;
}

void normalize_path_separators(char* path) {
    if (!path) return;

#ifdef _WIN32
    /* Convert forward slashes to backslashes on Windows */
    for (char* p = path; *p; p++) {
        if (*p == '/') {
            *p = '\\';
        }
    }
#else
    /* Convert backslashes to forward slashes on Unix */
    for (char* p = path; *p; p++) {
        if (*p == '\\') {
            *p = '/';
        }
    }
#endif
}

int path_is_within(const char* path, const char* base_dir) {
    if (!path || !base_dir) {
        return 0;
    }

    /* Simple prefix check - doesn't handle symlinks */
    size_t base_len = strlen(base_dir);
    if (strncmp(path, base_dir, base_len) != 0) {
        return 0;
    }

    /* Ensure the path doesn't just start with base_dir as a prefix of a longer name */
    if (path[base_len] != '\0' && path[base_len] != '/' && path[base_len] != '\\') {
        return 0;
    }

    /* Check for traversal sequences in the remaining path */
    const char* remaining = path + base_len;
    if (strstr(remaining, "..") != NULL) {
        return 0;
    }

    return 1;
}

/* ============================================================================
 * Command Validation
 * ============================================================================ */

/* List of dangerous shell metacharacters */
static const char* SHELL_METACHARACTERS = "|;&$`\"'\\<>(){}[]!#~*?";

ValidationResult validate_command(const char* cmd, size_t max_len) {
    if (!cmd) {
        return VALIDATION_NULL_INPUT;
    }

    size_t len = safe_strlen(cmd, max_len + 1);
    if (len == 0) {
        return VALIDATION_EMPTY_INPUT;
    }

    if (len > max_len) {
        return VALIDATION_TOO_LONG;
    }

    /* Check for null bytes */
    for (size_t i = 0; i < len; i++) {
        if (cmd[i] == '\0') {
            break;
        }
        /* Check for control characters (except common whitespace) */
        if (cmd[i] < 0x20 && cmd[i] != '\t' && cmd[i] != '\n' && cmd[i] != '\r') {
            return VALIDATION_INVALID_CHARS;
        }
    }

    return VALIDATION_OK;
}

int contains_shell_metacharacters(const char* cmd) {
    if (!cmd) {
        return 0;
    }

    for (const char* p = cmd; *p; p++) {
        if (strchr(SHELL_METACHARACTERS, *p) != NULL) {
            return 1;
        }
    }

    return 0;
}

int escape_shell_arg(const char* input, char* output, size_t output_size) {
    if (!input || !output || output_size == 0) {
        return -1;
    }

    size_t out_pos = 0;

#ifdef _WIN32
    /* Windows: wrap in double quotes, escape existing quotes */
    if (out_pos < output_size - 1) output[out_pos++] = '"';

    for (const char* p = input; *p && out_pos < output_size - 2; p++) {
        if (*p == '"') {
            if (out_pos < output_size - 2) {
                output[out_pos++] = '\\';
            }
        }
        output[out_pos++] = *p;
    }

    if (out_pos < output_size - 1) output[out_pos++] = '"';
#else
    /* Unix: wrap in single quotes, handle existing single quotes */
    if (out_pos < output_size - 1) output[out_pos++] = '\'';

    for (const char* p = input; *p && out_pos < output_size - 5; p++) {
        if (*p == '\'') {
            /* End quote, escaped quote, start quote: '\'' */
            output[out_pos++] = '\'';
            output[out_pos++] = '\\';
            output[out_pos++] = '\'';
            output[out_pos++] = '\'';
        } else {
            output[out_pos++] = *p;
        }
    }

    if (out_pos < output_size - 1) output[out_pos++] = '\'';
#endif

    output[out_pos] = '\0';
    return 0;
}

/* ============================================================================
 * Network Validation
 * ============================================================================ */

ValidationResult validate_ip_address(const char* ip) {
    if (!ip) {
        return VALIDATION_NULL_INPUT;
    }

    size_t len = strlen(ip);
    if (len == 0) {
        return VALIDATION_EMPTY_INPUT;
    }

    if (len > 45) {  /* Max IPv6 length */
        return VALIDATION_TOO_LONG;
    }

    /* Check for IPv4 */
    int octets = 0;
    int value = 0;
    int dots = 0;
    int has_digit = 0;

    for (const char* p = ip; *p; p++) {
        if (*p == '.') {
            if (!has_digit) {
                return VALIDATION_MALFORMED;
            }
            if (value > 255) {
                return VALIDATION_MALFORMED;
            }
            dots++;
            octets++;
            value = 0;
            has_digit = 0;
        } else if (*p >= '0' && *p <= '9') {
            value = value * 10 + (*p - '0');
            has_digit = 1;
        } else if (*p == ':') {
            /* Could be IPv6 - do simpler validation */
            break;
        } else {
            return VALIDATION_INVALID_CHARS;
        }
    }

    /* Check final octet for IPv4 */
    if (dots == 3 && has_digit && value <= 255) {
        return VALIDATION_OK;
    }

    /* Simple IPv6 validation - just check allowed characters */
    if (strchr(ip, ':') != NULL) {
        for (const char* p = ip; *p; p++) {
            if (!isxdigit((unsigned char)*p) && *p != ':' && *p != '.') {
                return VALIDATION_INVALID_CHARS;
            }
        }
        return VALIDATION_OK;
    }

    return VALIDATION_MALFORMED;
}

ValidationResult validate_port(int port) {
    if (port < 1 || port > 65535) {
        return VALIDATION_MALFORMED;
    }
    return VALIDATION_OK;
}

ValidationResult validate_subnet(const char* subnet) {
    if (!subnet) {
        return VALIDATION_NULL_INPUT;
    }

    size_t len = strlen(subnet);
    if (len == 0) {
        return VALIDATION_EMPTY_INPUT;
    }

    /* Find the slash */
    const char* slash = strchr(subnet, '/');
    if (!slash) {
        return VALIDATION_MALFORMED;
    }

    /* Validate IP part */
    size_t ip_len = (size_t)(slash - subnet);
    char ip[64];
    if (ip_len >= sizeof(ip)) {
        return VALIDATION_TOO_LONG;
    }

    safe_strcpy(ip, subnet, ip_len + 1);
    ip[ip_len] = '\0';

    ValidationResult ip_result = validate_ip_address(ip);
    if (ip_result != VALIDATION_OK) {
        return ip_result;
    }

    /* Validate CIDR prefix */
    int prefix = atoi(slash + 1);
    if (prefix < 0 || prefix > 128) {
        return VALIDATION_MALFORMED;
    }

    /* IPv4 max prefix is 32 */
    if (strchr(ip, '.') != NULL && prefix > 32) {
        return VALIDATION_MALFORMED;
    }

    return VALIDATION_OK;
}

ValidationResult validate_url(const char* url, size_t max_len) {
    if (!url) {
        return VALIDATION_NULL_INPUT;
    }

    size_t len = strlen(url);
    if (len == 0) {
        return VALIDATION_EMPTY_INPUT;
    }

    if (len > max_len) {
        return VALIDATION_TOO_LONG;
    }

    /* Check for valid scheme */
    if (strncmp(url, "http://", 7) != 0 && strncmp(url, "https://", 8) != 0) {
        return VALIDATION_MALFORMED;
    }

    /* Basic character validation */
    for (const char* p = url; *p; p++) {
        unsigned char c = (unsigned char)*p;
        /* Allow printable ASCII and high-bit characters (for internationalized URLs) */
        if (c < 0x20 || c == 0x7F) {
            return VALIDATION_INVALID_CHARS;
        }
    }

    return VALIDATION_OK;
}

/* ============================================================================
 * String Validation
 * ============================================================================ */

ValidationResult validate_printable_ascii(const char* str, size_t max_len) {
    if (!str) {
        return VALIDATION_NULL_INPUT;
    }

    for (size_t i = 0; i < max_len && str[i]; i++) {
        unsigned char c = (unsigned char)str[i];
        if (c < 0x20 || c > 0x7E) {
            /* Allow common whitespace */
            if (c != '\t' && c != '\n' && c != '\r') {
                return VALIDATION_INVALID_CHARS;
            }
        }
    }

    return VALIDATION_OK;
}

ValidationResult validate_alphanumeric(const char* str, size_t max_len) {
    if (!str) {
        return VALIDATION_NULL_INPUT;
    }

    for (size_t i = 0; i < max_len && str[i]; i++) {
        if (!isalnum((unsigned char)str[i])) {
            return VALIDATION_INVALID_CHARS;
        }
    }

    return VALIDATION_OK;
}

ValidationResult validate_uuid(const char* uuid) {
    if (!uuid) {
        return VALIDATION_NULL_INPUT;
    }

    size_t len = strlen(uuid);
    if (len != 36) {
        return VALIDATION_MALFORMED;
    }

    /* UUID format: 8-4-4-4-12 (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx) */
    for (int i = 0; i < 36; i++) {
        if (i == 8 || i == 13 || i == 18 || i == 23) {
            if (uuid[i] != '-') {
                return VALIDATION_MALFORMED;
            }
        } else {
            if (!isxdigit((unsigned char)uuid[i])) {
                return VALIDATION_INVALID_CHARS;
            }
        }
    }

    return VALIDATION_OK;
}

const char* validation_error_message(ValidationResult result) {
    switch (result) {
        case VALIDATION_OK:
            return "Validation successful";
        case VALIDATION_NULL_INPUT:
            return "Input is NULL";
        case VALIDATION_EMPTY_INPUT:
            return "Input is empty";
        case VALIDATION_PATH_TRAVERSAL:
            return "Path traversal detected";
        case VALIDATION_INVALID_CHARS:
            return "Invalid characters in input";
        case VALIDATION_TOO_LONG:
            return "Input exceeds maximum length";
        case VALIDATION_MALFORMED:
            return "Input is malformed";
        default:
            return "Unknown validation error";
    }
}
