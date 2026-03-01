/*
 * NodePulse Agent - Input Validation Utilities
 *
 * Provides validation functions for user input to prevent
 * injection attacks and ensure data integrity.
 */

#ifndef VALIDATION_H
#define VALIDATION_H

#include <stddef.h>

/* ============================================================================
 * Path Validation
 * ============================================================================ */

/**
 * Validation result codes
 */
typedef enum {
    VALIDATION_OK = 0,
    VALIDATION_NULL_INPUT = -1,
    VALIDATION_EMPTY_INPUT = -2,
    VALIDATION_PATH_TRAVERSAL = -3,
    VALIDATION_INVALID_CHARS = -4,
    VALIDATION_TOO_LONG = -5,
    VALIDATION_MALFORMED = -6
} ValidationResult;

/**
 * Validate a file path for safety.
 * Checks for:
 * - NULL/empty input
 * - Path traversal attempts (../)
 * - Null bytes
 * - Invalid characters
 *
 * @param path Path to validate
 * @param max_len Maximum allowed length
 * @return VALIDATION_OK if valid, error code otherwise
 */
ValidationResult validate_path(const char* path, size_t max_len);

/**
 * Sanitize a file path by removing dangerous components.
 * Creates a new string with:
 * - Path traversal sequences removed
 * - Null bytes removed
 * - Normalized separators
 *
 * @param path Path to sanitize
 * @param output Buffer to write sanitized path
 * @param output_size Size of output buffer
 * @return 0 on success, -1 on error
 */
int sanitize_path(const char* path, char* output, size_t output_size);

/**
 * Normalize path separators for the current platform.
 *
 * @param path Path to normalize (modified in place)
 */
void normalize_path_separators(char* path);

/**
 * Check if path is within a base directory (no escape via symlinks or ..)
 *
 * @param path Path to check
 * @param base_dir Base directory path must be within
 * @return 1 if path is within base_dir, 0 otherwise
 */
int path_is_within(const char* path, const char* base_dir);

/* ============================================================================
 * Command Validation
 * ============================================================================ */

/**
 * Validate a shell command for execution.
 * Checks for dangerous patterns and injection attempts.
 *
 * @param cmd Command to validate
 * @param max_len Maximum allowed length
 * @return VALIDATION_OK if valid, error code otherwise
 */
ValidationResult validate_command(const char* cmd, size_t max_len);

/**
 * Check if a command contains shell metacharacters.
 *
 * @param cmd Command to check
 * @return 1 if contains metacharacters, 0 otherwise
 */
int contains_shell_metacharacters(const char* cmd);

/**
 * Escape shell metacharacters in a string.
 *
 * @param input Input string
 * @param output Buffer for escaped string
 * @param output_size Size of output buffer
 * @return 0 on success, -1 on error
 */
int escape_shell_arg(const char* input, char* output, size_t output_size);

/* ============================================================================
 * Network Validation
 * ============================================================================ */

/**
 * Validate an IP address string.
 *
 * @param ip IP address string
 * @return VALIDATION_OK if valid IPv4 or IPv6, error code otherwise
 */
ValidationResult validate_ip_address(const char* ip);

/**
 * Validate a port number.
 *
 * @param port Port number
 * @return VALIDATION_OK if valid (1-65535), error code otherwise
 */
ValidationResult validate_port(int port);

/**
 * Validate a CIDR subnet notation.
 *
 * @param subnet Subnet string (e.g., "192.168.1.0/24")
 * @return VALIDATION_OK if valid, error code otherwise
 */
ValidationResult validate_subnet(const char* subnet);

/**
 * Validate a URL.
 *
 * @param url URL string
 * @param max_len Maximum allowed length
 * @return VALIDATION_OK if valid, error code otherwise
 */
ValidationResult validate_url(const char* url, size_t max_len);

/* ============================================================================
 * String Validation
 * ============================================================================ */

/**
 * Validate a string contains only printable ASCII characters.
 *
 * @param str String to validate
 * @param max_len Maximum length to check
 * @return VALIDATION_OK if valid, error code otherwise
 */
ValidationResult validate_printable_ascii(const char* str, size_t max_len);

/**
 * Validate a string contains only alphanumeric characters.
 *
 * @param str String to validate
 * @param max_len Maximum length to check
 * @return VALIDATION_OK if valid, error code otherwise
 */
ValidationResult validate_alphanumeric(const char* str, size_t max_len);

/**
 * Validate a UUID string format.
 *
 * @param uuid UUID string to validate
 * @return VALIDATION_OK if valid UUID format, error code otherwise
 */
ValidationResult validate_uuid(const char* uuid);

/**
 * Get human-readable error message for validation result.
 *
 * @param result Validation result code
 * @return Error message string
 */
const char* validation_error_message(ValidationResult result);

#endif /* VALIDATION_H */
