/*
 * NodePulse Agent - Lightweight JSON Parser
 *
 * Simple JSON parser optimized for agent communication.
 * Supports parsing and building JSON objects safely.
 */

#ifndef JSON_H
#define JSON_H

#include <stddef.h>

/* ============================================================================
 * JSON Value Types
 * ============================================================================ */

typedef enum {
    JSON_NULL,
    JSON_BOOL,
    JSON_NUMBER,
    JSON_STRING,
    JSON_ARRAY,
    JSON_OBJECT
} JsonType;

/* Forward declarations */
typedef struct JsonValue JsonValue;
typedef struct JsonArray JsonArray;
typedef struct JsonObject JsonObject;

/* ============================================================================
 * JSON Value Structure
 * ============================================================================ */

struct JsonValue {
    JsonType type;
    union {
        int bool_value;
        double number_value;
        char* string_value;
        JsonArray* array_value;
        JsonObject* object_value;
    };
};

/* Array: dynamic list of JsonValue */
struct JsonArray {
    JsonValue** items;
    size_t count;
    size_t capacity;
};

/* Object: list of key-value pairs */
typedef struct JsonKeyValue {
    char* key;
    JsonValue* value;
} JsonKeyValue;

struct JsonObject {
    JsonKeyValue* pairs;
    size_t count;
    size_t capacity;
};

/* ============================================================================
 * JSON Parsing
 * ============================================================================ */

/**
 * Parse a JSON string into a JsonValue.
 *
 * @param json JSON string to parse
 * @param error If non-NULL, receives error message on failure
 * @return Parsed JsonValue, or NULL on error
 */
JsonValue* json_parse(const char* json, const char** error);

/**
 * Parse a JSON string with length limit.
 *
 * @param json JSON string to parse
 * @param max_len Maximum length to parse
 * @param error If non-NULL, receives error message on failure
 * @return Parsed JsonValue, or NULL on error
 */
JsonValue* json_parse_n(const char* json, size_t max_len, const char** error);

/* ============================================================================
 * JSON Value Creation
 * ============================================================================ */

JsonValue* json_null(void);
JsonValue* json_bool(int value);
JsonValue* json_number(double value);
JsonValue* json_string(const char* value);
JsonValue* json_array(void);
JsonValue* json_object(void);

/* ============================================================================
 * JSON Value Access
 * ============================================================================ */

/**
 * Get type of JSON value.
 */
JsonType json_get_type(const JsonValue* value);

/**
 * Get boolean value (returns 0 if not a boolean).
 */
int json_get_bool(const JsonValue* value);

/**
 * Get number value (returns 0.0 if not a number).
 */
double json_get_number(const JsonValue* value);

/**
 * Get integer value (returns 0 if not a number).
 */
int json_get_int(const JsonValue* value);

/**
 * Get string value (returns NULL if not a string).
 * The returned pointer is valid until the value is freed.
 */
const char* json_get_string(const JsonValue* value);

/**
 * Check if value is null.
 */
int json_is_null(const JsonValue* value);

/* ============================================================================
 * JSON Object Operations
 * ============================================================================ */

/**
 * Get a value from an object by key.
 *
 * @param obj JSON object
 * @param key Key to look up
 * @return Value for key, or NULL if not found
 */
JsonValue* json_object_get(const JsonValue* obj, const char* key);

/**
 * Get a string from an object by key.
 *
 * @param obj JSON object
 * @param key Key to look up
 * @return String value, or NULL if not found or not a string
 */
const char* json_object_get_string(const JsonValue* obj, const char* key);

/**
 * Get an integer from an object by key.
 *
 * @param obj JSON object
 * @param key Key to look up
 * @param default_val Default value if not found
 * @return Integer value, or default_val if not found
 */
int json_object_get_int(const JsonValue* obj, const char* key, int default_val);

/**
 * Get a boolean from an object by key.
 *
 * @param obj JSON object
 * @param key Key to look up
 * @param default_val Default value if not found
 * @return Boolean value, or default_val if not found
 */
int json_object_get_bool(const JsonValue* obj, const char* key, int default_val);

/**
 * Set a key-value pair in an object.
 * The object takes ownership of the value.
 *
 * @param obj JSON object
 * @param key Key (will be copied)
 * @param value Value (ownership transferred)
 * @return 0 on success, -1 on error
 */
int json_object_set(JsonValue* obj, const char* key, JsonValue* value);

/**
 * Set a string value in an object.
 */
int json_object_set_string(JsonValue* obj, const char* key, const char* value);

/**
 * Set an integer value in an object.
 */
int json_object_set_int(JsonValue* obj, const char* key, int value);

/**
 * Set a boolean value in an object.
 */
int json_object_set_bool(JsonValue* obj, const char* key, int value);

/**
 * Get number of keys in an object.
 */
size_t json_object_count(const JsonValue* obj);

/**
 * Get key at index in an object.
 */
const char* json_object_key_at(const JsonValue* obj, size_t index);

/**
 * Get value at index in an object.
 */
JsonValue* json_object_value_at(const JsonValue* obj, size_t index);

/* ============================================================================
 * JSON Array Operations
 * ============================================================================ */

/**
 * Get array length.
 */
size_t json_array_length(const JsonValue* arr);

/**
 * Get element at index.
 */
JsonValue* json_array_get(const JsonValue* arr, size_t index);

/**
 * Append a value to an array.
 * The array takes ownership of the value.
 *
 * @return 0 on success, -1 on error
 */
int json_array_append(JsonValue* arr, JsonValue* value);

/* ============================================================================
 * JSON Serialization
 * ============================================================================ */

/**
 * Serialize a JSON value to a string.
 *
 * @param value JSON value to serialize
 * @return Newly allocated string, or NULL on error
 */
char* json_stringify(const JsonValue* value);

/**
 * Serialize a JSON value to a string with pretty printing.
 *
 * @param value JSON value to serialize
 * @param indent Indentation string (e.g., "  " for 2 spaces)
 * @return Newly allocated string, or NULL on error
 */
char* json_stringify_pretty(const JsonValue* value, const char* indent);

/* ============================================================================
 * JSON Memory Management
 * ============================================================================ */

/**
 * Free a JSON value and all its contents.
 */
void json_free(JsonValue* value);

/**
 * Deep copy a JSON value.
 */
JsonValue* json_clone(const JsonValue* value);

#endif /* JSON_H */
