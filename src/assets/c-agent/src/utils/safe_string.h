/*
 * NodePulse Agent - Safe String Utilities
 *
 * Provides safe string operations to prevent buffer overflows.
 * All functions perform bounds checking and null-termination.
 */

#ifndef SAFE_STRING_H
#define SAFE_STRING_H

#include <stddef.h>
#include <stdarg.h>

/* ============================================================================
 * Safe String Copy Operations
 * ============================================================================ */

/**
 * Safe string copy with bounds checking.
 * Always null-terminates the destination.
 *
 * @param dst Destination buffer
 * @param src Source string
 * @param dst_size Size of destination buffer
 * @return Number of characters copied (excluding null terminator), or -1 on error
 */
int safe_strcpy(char* dst, const char* src, size_t dst_size);

/**
 * Safe string concatenation with bounds checking.
 * Always null-terminates the destination.
 *
 * @param dst Destination buffer (must already be null-terminated)
 * @param src Source string to append
 * @param dst_size Total size of destination buffer
 * @return Total length of resulting string, or -1 on error
 */
int safe_strcat(char* dst, const char* src, size_t dst_size);

/**
 * Safe formatted string print with bounds checking.
 * Always null-terminates the destination.
 *
 * @param dst Destination buffer
 * @param dst_size Size of destination buffer
 * @param fmt Format string
 * @return Number of characters written (excluding null), or -1 on error
 */
int safe_snprintf(char* dst, size_t dst_size, const char* fmt, ...);

/**
 * Safe formatted string print (va_list version)
 */
int safe_vsnprintf(char* dst, size_t dst_size, const char* fmt, va_list args);

/* ============================================================================
 * Safe Memory Operations
 * ============================================================================ */

/**
 * Safe memory copy with bounds checking.
 *
 * @param dst Destination buffer
 * @param dst_size Size of destination buffer
 * @param src Source data
 * @param count Number of bytes to copy
 * @return 0 on success, -1 on error (would overflow)
 */
int safe_memcpy(void* dst, size_t dst_size, const void* src, size_t count);

/**
 * Safe memory set with bounds checking.
 *
 * @param dst Destination buffer
 * @param dst_size Size of destination buffer
 * @param value Value to set
 * @param count Number of bytes to set
 * @return 0 on success, -1 on error
 */
int safe_memset(void* dst, size_t dst_size, int value, size_t count);

/* ============================================================================
 * Dynamic String Buffer
 * ============================================================================ */

/**
 * Dynamic string buffer that grows automatically.
 * Prevents buffer overflows by managing its own memory.
 */
typedef struct {
    char* data;
    size_t length;
    size_t capacity;
} StringBuffer;

/**
 * Initialize a string buffer with initial capacity.
 *
 * @param sb String buffer to initialize
 * @param initial_capacity Initial capacity (0 for default)
 * @return 0 on success, -1 on error
 */
int strbuf_init(StringBuffer* sb, size_t initial_capacity);

/**
 * Free string buffer resources.
 */
void strbuf_free(StringBuffer* sb);

/**
 * Append a string to the buffer, growing if necessary.
 *
 * @param sb String buffer
 * @param str String to append
 * @return 0 on success, -1 on error
 */
int strbuf_append(StringBuffer* sb, const char* str);

/**
 * Append formatted string to buffer.
 *
 * @param sb String buffer
 * @param fmt Format string
 * @return 0 on success, -1 on error
 */
int strbuf_appendf(StringBuffer* sb, const char* fmt, ...);

/**
 * Append raw bytes to buffer.
 *
 * @param sb String buffer
 * @param data Data to append
 * @param len Length of data
 * @return 0 on success, -1 on error
 */
int strbuf_append_bytes(StringBuffer* sb, const char* data, size_t len);

/**
 * Clear the buffer without freeing memory.
 */
void strbuf_clear(StringBuffer* sb);

/**
 * Get the current string (null-terminated).
 * The returned pointer is valid until the next buffer operation.
 */
const char* strbuf_get(const StringBuffer* sb);

/**
 * Detach and return the string, resetting the buffer.
 * Caller is responsible for freeing the returned string.
 */
char* strbuf_detach(StringBuffer* sb);

/* ============================================================================
 * String Utilities
 * ============================================================================ */

/**
 * Safely duplicate a string.
 *
 * @param str String to duplicate
 * @return Newly allocated copy, or NULL on error
 */
char* safe_strdup(const char* str);

/**
 * Safely duplicate a string with length limit.
 *
 * @param str String to duplicate
 * @param max_len Maximum length to copy
 * @return Newly allocated copy, or NULL on error
 */
char* safe_strndup(const char* str, size_t max_len);

/**
 * Trim whitespace from both ends of a string in-place.
 *
 * @param str String to trim
 * @return Pointer to trimmed string (same buffer, possibly offset)
 */
char* str_trim(char* str);

/**
 * Check if a string starts with a prefix.
 *
 * @param str String to check
 * @param prefix Prefix to look for
 * @return 1 if string starts with prefix, 0 otherwise
 */
int str_starts_with(const char* str, const char* prefix);

/**
 * Check if a string ends with a suffix.
 *
 * @param str String to check
 * @param suffix Suffix to look for
 * @return 1 if string ends with suffix, 0 otherwise
 */
int str_ends_with(const char* str, const char* suffix);

/**
 * Find the first occurrence of a character in a string with length limit.
 *
 * @param str String to search
 * @param c Character to find
 * @param max_len Maximum characters to search
 * @return Pointer to character, or NULL if not found
 */
char* safe_strchr(const char* str, int c, size_t max_len);

/**
 * Calculate string length with maximum limit.
 *
 * @param str String to measure
 * @param max_len Maximum length to check
 * @return Length of string, or max_len if no null terminator found
 */
size_t safe_strlen(const char* str, size_t max_len);

/* ============================================================================
 * Portable strcasestr
 * ============================================================================ */

/**
 * Case-insensitive substring search (portable).
 * strcasestr is a GNU extension not available on Windows/MinGW.
 *
 * @param haystack String to search in
 * @param needle String to search for
 * @return Pointer to first occurrence, or NULL if not found
 */
#if defined(_WIN32) || defined(_WIN64)
char* portable_strcasestr(const char* haystack, const char* needle);
#define strcasestr portable_strcasestr
#endif

#endif /* SAFE_STRING_H */
