/*
 * NodePulse Agent - Safe String Utilities Implementation
 */

#include "safe_string.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>

/* Default initial capacity for string buffers */
#define DEFAULT_STRBUF_CAPACITY 256

/* Growth factor for string buffers */
#define STRBUF_GROWTH_FACTOR 2

/* ============================================================================
 * Safe String Copy Operations
 * ============================================================================ */

int safe_strcpy(char* dst, const char* src, size_t dst_size) {
    if (!dst || dst_size == 0) {
        return -1;
    }

    if (!src) {
        dst[0] = '\0';
        return 0;
    }

    size_t src_len = strlen(src);
    size_t copy_len = (src_len < dst_size - 1) ? src_len : (dst_size - 1);

    memcpy(dst, src, copy_len);
    dst[copy_len] = '\0';

    return (int)copy_len;
}

int safe_strcat(char* dst, const char* src, size_t dst_size) {
    if (!dst || dst_size == 0) {
        return -1;
    }

    size_t dst_len = strnlen(dst, dst_size);
    if (dst_len >= dst_size) {
        return -1;  /* dst not null-terminated within bounds */
    }

    if (!src) {
        return (int)dst_len;
    }

    size_t remaining = dst_size - dst_len - 1;
    size_t src_len = strlen(src);
    size_t copy_len = (src_len < remaining) ? src_len : remaining;

    memcpy(dst + dst_len, src, copy_len);
    dst[dst_len + copy_len] = '\0';

    return (int)(dst_len + copy_len);
}

int safe_snprintf(char* dst, size_t dst_size, const char* fmt, ...) {
    if (!dst || dst_size == 0 || !fmt) {
        return -1;
    }

    va_list args;
    va_start(args, fmt);
    int result = safe_vsnprintf(dst, dst_size, fmt, args);
    va_end(args);

    return result;
}

int safe_vsnprintf(char* dst, size_t dst_size, const char* fmt, va_list args) {
    if (!dst || dst_size == 0 || !fmt) {
        return -1;
    }

    int result = vsnprintf(dst, dst_size, fmt, args);

    /* Ensure null termination */
    if (result < 0) {
        dst[0] = '\0';
        return -1;
    }

    if ((size_t)result >= dst_size) {
        dst[dst_size - 1] = '\0';
        return (int)(dst_size - 1);
    }

    return result;
}

/* ============================================================================
 * Safe Memory Operations
 * ============================================================================ */

int safe_memcpy(void* dst, size_t dst_size, const void* src, size_t count) {
    if (!dst || !src) {
        return -1;
    }

    if (count > dst_size) {
        return -1;  /* Would overflow */
    }

    memcpy(dst, src, count);
    return 0;
}

int safe_memset(void* dst, size_t dst_size, int value, size_t count) {
    if (!dst) {
        return -1;
    }

    if (count > dst_size) {
        count = dst_size;  /* Clamp to available size */
    }

    memset(dst, value, count);
    return 0;
}

/* ============================================================================
 * Dynamic String Buffer
 * ============================================================================ */

static int strbuf_ensure_capacity(StringBuffer* sb, size_t needed) {
    if (!sb) {
        return -1;
    }

    size_t required = sb->length + needed + 1;  /* +1 for null terminator */

    if (required <= sb->capacity) {
        return 0;
    }

    /* Calculate new capacity */
    size_t new_capacity = sb->capacity ? sb->capacity : DEFAULT_STRBUF_CAPACITY;
    while (new_capacity < required) {
        new_capacity *= STRBUF_GROWTH_FACTOR;
    }

    /* Reallocate */
    char* new_data = (char*)realloc(sb->data, new_capacity);
    if (!new_data) {
        return -1;
    }

    sb->data = new_data;
    sb->capacity = new_capacity;

    return 0;
}

int strbuf_init(StringBuffer* sb, size_t initial_capacity) {
    if (!sb) {
        return -1;
    }

    sb->data = NULL;
    sb->length = 0;
    sb->capacity = 0;

    if (initial_capacity > 0) {
        sb->data = (char*)malloc(initial_capacity);
        if (!sb->data) {
            return -1;
        }
        sb->capacity = initial_capacity;
        sb->data[0] = '\0';
    }

    return 0;
}

void strbuf_free(StringBuffer* sb) {
    if (sb) {
        free(sb->data);
        sb->data = NULL;
        sb->length = 0;
        sb->capacity = 0;
    }
}

int strbuf_append(StringBuffer* sb, const char* str) {
    if (!sb) {
        return -1;
    }

    if (!str) {
        return 0;
    }

    size_t len = strlen(str);
    if (strbuf_ensure_capacity(sb, len) != 0) {
        return -1;
    }

    memcpy(sb->data + sb->length, str, len);
    sb->length += len;
    sb->data[sb->length] = '\0';

    return 0;
}

int strbuf_appendf(StringBuffer* sb, const char* fmt, ...) {
    if (!sb || !fmt) {
        return -1;
    }

    va_list args;

    /* First, calculate required size */
    va_start(args, fmt);
    int required = vsnprintf(NULL, 0, fmt, args);
    va_end(args);

    if (required < 0) {
        return -1;
    }

    if (strbuf_ensure_capacity(sb, (size_t)required) != 0) {
        return -1;
    }

    /* Now format into buffer */
    va_start(args, fmt);
    vsnprintf(sb->data + sb->length, (size_t)required + 1, fmt, args);
    va_end(args);

    sb->length += (size_t)required;

    return 0;
}

int strbuf_append_bytes(StringBuffer* sb, const char* data, size_t len) {
    if (!sb) {
        return -1;
    }

    if (!data || len == 0) {
        return 0;
    }

    if (strbuf_ensure_capacity(sb, len) != 0) {
        return -1;
    }

    memcpy(sb->data + sb->length, data, len);
    sb->length += len;
    sb->data[sb->length] = '\0';

    return 0;
}

void strbuf_clear(StringBuffer* sb) {
    if (sb) {
        sb->length = 0;
        if (sb->data) {
            sb->data[0] = '\0';
        }
    }
}

const char* strbuf_get(const StringBuffer* sb) {
    if (!sb || !sb->data) {
        return "";
    }
    return sb->data;
}

char* strbuf_detach(StringBuffer* sb) {
    if (!sb) {
        return NULL;
    }

    char* result = sb->data;

    /* Ensure we return a valid string */
    if (!result) {
        result = (char*)malloc(1);
        if (result) {
            result[0] = '\0';
        }
    }

    sb->data = NULL;
    sb->length = 0;
    sb->capacity = 0;

    return result;
}

/* ============================================================================
 * String Utilities
 * ============================================================================ */

char* safe_strdup(const char* str) {
    if (!str) {
        return NULL;
    }

    size_t len = strlen(str);
    char* copy = (char*)malloc(len + 1);
    if (!copy) {
        return NULL;
    }

    memcpy(copy, str, len + 1);
    return copy;
}

char* safe_strndup(const char* str, size_t max_len) {
    if (!str) {
        return NULL;
    }

    size_t len = strnlen(str, max_len);
    char* copy = (char*)malloc(len + 1);
    if (!copy) {
        return NULL;
    }

    memcpy(copy, str, len);
    copy[len] = '\0';
    return copy;
}

char* str_trim(char* str) {
    if (!str) {
        return NULL;
    }

    /* Trim leading whitespace */
    while (*str && isspace((unsigned char)*str)) {
        str++;
    }

    if (*str == '\0') {
        return str;
    }

    /* Trim trailing whitespace */
    char* end = str + strlen(str) - 1;
    while (end > str && isspace((unsigned char)*end)) {
        end--;
    }
    end[1] = '\0';

    return str;
}

int str_starts_with(const char* str, const char* prefix) {
    if (!str || !prefix) {
        return 0;
    }

    size_t prefix_len = strlen(prefix);
    return strncmp(str, prefix, prefix_len) == 0;
}

int str_ends_with(const char* str, const char* suffix) {
    if (!str || !suffix) {
        return 0;
    }

    size_t str_len = strlen(str);
    size_t suffix_len = strlen(suffix);

    if (suffix_len > str_len) {
        return 0;
    }

    return strcmp(str + str_len - suffix_len, suffix) == 0;
}

char* safe_strchr(const char* str, int c, size_t max_len) {
    if (!str) {
        return NULL;
    }

    for (size_t i = 0; i < max_len && str[i] != '\0'; i++) {
        if (str[i] == (char)c) {
            return (char*)(str + i);
        }
    }

    return NULL;
}

size_t safe_strlen(const char* str, size_t max_len) {
    if (!str) {
        return 0;
    }

    return strnlen(str, max_len);
}

/* ============================================================================
 * Portable strcasestr
 * ============================================================================ */

#if defined(_WIN32) || defined(_WIN64)
char* portable_strcasestr(const char* haystack, const char* needle) {
    if (!haystack || !needle) {
        return NULL;
    }

    if (needle[0] == '\0') {
        return (char*)haystack;
    }

    size_t needle_len = strlen(needle);

    for (; *haystack; haystack++) {
        if (_strnicmp(haystack, needle, needle_len) == 0) {
            return (char*)haystack;
        }
    }

    return NULL;
}
#endif
