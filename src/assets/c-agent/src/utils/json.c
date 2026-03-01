/*
 * NodePulse Agent - Lightweight JSON Parser Implementation
 */

#include "json.h"
#include "safe_string.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <math.h>

/* Initial capacity for arrays and objects */
#define INITIAL_CAPACITY 8
#define GROWTH_FACTOR 2

/* Parser context */
typedef struct {
    const char* json;
    size_t pos;
    size_t max_len;
    const char* error;
} ParseContext;

/* Forward declarations */
static JsonValue* parse_value(ParseContext* ctx);
static void skip_whitespace(ParseContext* ctx);

/* ============================================================================
 * Memory Management
 * ============================================================================ */

static JsonValue* json_value_alloc(JsonType type) {
    JsonValue* value = (JsonValue*)calloc(1, sizeof(JsonValue));
    if (value) {
        value->type = type;
    }
    return value;
}

void json_free(JsonValue* value) {
    if (!value) return;

    switch (value->type) {
        case JSON_STRING:
            free(value->string_value);
            break;

        case JSON_ARRAY:
            if (value->array_value) {
                for (size_t i = 0; i < value->array_value->count; i++) {
                    json_free(value->array_value->items[i]);
                }
                free(value->array_value->items);
                free(value->array_value);
            }
            break;

        case JSON_OBJECT:
            if (value->object_value) {
                for (size_t i = 0; i < value->object_value->count; i++) {
                    free(value->object_value->pairs[i].key);
                    json_free(value->object_value->pairs[i].value);
                }
                free(value->object_value->pairs);
                free(value->object_value);
            }
            break;

        default:
            break;
    }

    free(value);
}

/* ============================================================================
 * JSON Value Creation
 * ============================================================================ */

JsonValue* json_null(void) {
    return json_value_alloc(JSON_NULL);
}

JsonValue* json_bool(int value) {
    JsonValue* v = json_value_alloc(JSON_BOOL);
    if (v) {
        v->bool_value = value ? 1 : 0;
    }
    return v;
}

JsonValue* json_number(double value) {
    JsonValue* v = json_value_alloc(JSON_NUMBER);
    if (v) {
        v->number_value = value;
    }
    return v;
}

JsonValue* json_string(const char* value) {
    JsonValue* v = json_value_alloc(JSON_STRING);
    if (v) {
        v->string_value = value ? safe_strdup(value) : safe_strdup("");
        if (!v->string_value) {
            free(v);
            return NULL;
        }
    }
    return v;
}

JsonValue* json_array(void) {
    JsonValue* v = json_value_alloc(JSON_ARRAY);
    if (v) {
        v->array_value = (JsonArray*)calloc(1, sizeof(JsonArray));
        if (!v->array_value) {
            free(v);
            return NULL;
        }
    }
    return v;
}

JsonValue* json_object(void) {
    JsonValue* v = json_value_alloc(JSON_OBJECT);
    if (v) {
        v->object_value = (JsonObject*)calloc(1, sizeof(JsonObject));
        if (!v->object_value) {
            free(v);
            return NULL;
        }
    }
    return v;
}

/* ============================================================================
 * JSON Value Access
 * ============================================================================ */

JsonType json_get_type(const JsonValue* value) {
    return value ? value->type : JSON_NULL;
}

int json_get_bool(const JsonValue* value) {
    return (value && value->type == JSON_BOOL) ? value->bool_value : 0;
}

double json_get_number(const JsonValue* value) {
    return (value && value->type == JSON_NUMBER) ? value->number_value : 0.0;
}

int json_get_int(const JsonValue* value) {
    return (value && value->type == JSON_NUMBER) ? (int)value->number_value : 0;
}

const char* json_get_string(const JsonValue* value) {
    return (value && value->type == JSON_STRING) ? value->string_value : NULL;
}

int json_is_null(const JsonValue* value) {
    return !value || value->type == JSON_NULL;
}

/* ============================================================================
 * JSON Object Operations
 * ============================================================================ */

JsonValue* json_object_get(const JsonValue* obj, const char* key) {
    if (!obj || obj->type != JSON_OBJECT || !obj->object_value || !key) {
        return NULL;
    }

    for (size_t i = 0; i < obj->object_value->count; i++) {
        if (strcmp(obj->object_value->pairs[i].key, key) == 0) {
            return obj->object_value->pairs[i].value;
        }
    }

    return NULL;
}

const char* json_object_get_string(const JsonValue* obj, const char* key) {
    JsonValue* v = json_object_get(obj, key);
    return (v && v->type == JSON_STRING) ? v->string_value : NULL;
}

int json_object_get_int(const JsonValue* obj, const char* key, int default_val) {
    JsonValue* v = json_object_get(obj, key);
    return (v && v->type == JSON_NUMBER) ? (int)v->number_value : default_val;
}

int json_object_get_bool(const JsonValue* obj, const char* key, int default_val) {
    JsonValue* v = json_object_get(obj, key);
    return (v && v->type == JSON_BOOL) ? v->bool_value : default_val;
}

int json_object_set(JsonValue* obj, const char* key, JsonValue* value) {
    if (!obj || obj->type != JSON_OBJECT || !obj->object_value || !key) {
        return -1;
    }

    JsonObject* o = obj->object_value;

    /* Check if key exists */
    for (size_t i = 0; i < o->count; i++) {
        if (strcmp(o->pairs[i].key, key) == 0) {
            json_free(o->pairs[i].value);
            o->pairs[i].value = value;
            return 0;
        }
    }

    /* Add new key */
    if (o->count >= o->capacity) {
        size_t new_cap = o->capacity ? o->capacity * GROWTH_FACTOR : INITIAL_CAPACITY;
        JsonKeyValue* new_pairs = (JsonKeyValue*)realloc(o->pairs, new_cap * sizeof(JsonKeyValue));
        if (!new_pairs) {
            return -1;
        }
        o->pairs = new_pairs;
        o->capacity = new_cap;
    }

    o->pairs[o->count].key = safe_strdup(key);
    if (!o->pairs[o->count].key) {
        return -1;
    }
    o->pairs[o->count].value = value;
    o->count++;

    return 0;
}

int json_object_set_string(JsonValue* obj, const char* key, const char* value) {
    JsonValue* v = json_string(value);
    if (!v) return -1;
    if (json_object_set(obj, key, v) != 0) {
        json_free(v);
        return -1;
    }
    return 0;
}

int json_object_set_int(JsonValue* obj, const char* key, int value) {
    JsonValue* v = json_number((double)value);
    if (!v) return -1;
    if (json_object_set(obj, key, v) != 0) {
        json_free(v);
        return -1;
    }
    return 0;
}

int json_object_set_bool(JsonValue* obj, const char* key, int value) {
    JsonValue* v = json_bool(value);
    if (!v) return -1;
    if (json_object_set(obj, key, v) != 0) {
        json_free(v);
        return -1;
    }
    return 0;
}

size_t json_object_count(const JsonValue* obj) {
    if (!obj || obj->type != JSON_OBJECT || !obj->object_value) {
        return 0;
    }
    return obj->object_value->count;
}

const char* json_object_key_at(const JsonValue* obj, size_t index) {
    if (!obj || obj->type != JSON_OBJECT || !obj->object_value) {
        return NULL;
    }
    if (index >= obj->object_value->count) {
        return NULL;
    }
    return obj->object_value->pairs[index].key;
}

JsonValue* json_object_value_at(const JsonValue* obj, size_t index) {
    if (!obj || obj->type != JSON_OBJECT || !obj->object_value) {
        return NULL;
    }
    if (index >= obj->object_value->count) {
        return NULL;
    }
    return obj->object_value->pairs[index].value;
}

/* ============================================================================
 * JSON Array Operations
 * ============================================================================ */

size_t json_array_length(const JsonValue* arr) {
    if (!arr || arr->type != JSON_ARRAY || !arr->array_value) {
        return 0;
    }
    return arr->array_value->count;
}

JsonValue* json_array_get(const JsonValue* arr, size_t index) {
    if (!arr || arr->type != JSON_ARRAY || !arr->array_value) {
        return NULL;
    }
    if (index >= arr->array_value->count) {
        return NULL;
    }
    return arr->array_value->items[index];
}

int json_array_append(JsonValue* arr, JsonValue* value) {
    if (!arr || arr->type != JSON_ARRAY || !arr->array_value) {
        return -1;
    }

    JsonArray* a = arr->array_value;

    if (a->count >= a->capacity) {
        size_t new_cap = a->capacity ? a->capacity * GROWTH_FACTOR : INITIAL_CAPACITY;
        JsonValue** new_items = (JsonValue**)realloc(a->items, new_cap * sizeof(JsonValue*));
        if (!new_items) {
            return -1;
        }
        a->items = new_items;
        a->capacity = new_cap;
    }

    a->items[a->count++] = value;
    return 0;
}

/* ============================================================================
 * JSON Parsing
 * ============================================================================ */

static int peek_char(ParseContext* ctx) {
    if (ctx->pos >= ctx->max_len) {
        return -1;
    }
    return (unsigned char)ctx->json[ctx->pos];
}

static int next_char(ParseContext* ctx) {
    if (ctx->pos >= ctx->max_len) {
        return -1;
    }
    return (unsigned char)ctx->json[ctx->pos++];
}

static void skip_whitespace(ParseContext* ctx) {
    while (ctx->pos < ctx->max_len) {
        char c = ctx->json[ctx->pos];
        if (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
            ctx->pos++;
        } else {
            break;
        }
    }
}

static JsonValue* parse_string(ParseContext* ctx) {
    if (next_char(ctx) != '"') {
        ctx->error = "Expected '\"'";
        return NULL;
    }

    StringBuffer sb;
    if (strbuf_init(&sb, 64) != 0) {
        ctx->error = "Memory allocation failed";
        return NULL;
    }

    while (ctx->pos < ctx->max_len) {
        char c = ctx->json[ctx->pos++];

        if (c == '"') {
            JsonValue* v = json_string(strbuf_get(&sb));
            strbuf_free(&sb);
            return v;
        }

        if (c == '\\') {
            if (ctx->pos >= ctx->max_len) {
                ctx->error = "Unterminated string escape";
                strbuf_free(&sb);
                return NULL;
            }

            char escape = ctx->json[ctx->pos++];
            switch (escape) {
                case '"':  strbuf_append(&sb, "\""); break;
                case '\\': strbuf_append(&sb, "\\"); break;
                case '/':  strbuf_append(&sb, "/"); break;
                case 'b':  strbuf_append(&sb, "\b"); break;
                case 'f':  strbuf_append(&sb, "\f"); break;
                case 'n':  strbuf_append(&sb, "\n"); break;
                case 'r':  strbuf_append(&sb, "\r"); break;
                case 't':  strbuf_append(&sb, "\t"); break;
                case 'u': {
                    /* Unicode escape: \uXXXX */
                    if (ctx->pos + 4 > ctx->max_len) {
                        ctx->error = "Incomplete unicode escape";
                        strbuf_free(&sb);
                        return NULL;
                    }
                    char hex[5] = {0};
                    memcpy(hex, ctx->json + ctx->pos, 4);
                    ctx->pos += 4;

                    unsigned int codepoint;
                    if (sscanf(hex, "%4x", &codepoint) != 1) {
                        ctx->error = "Invalid unicode escape";
                        strbuf_free(&sb);
                        return NULL;
                    }

                    /* Convert to UTF-8 */
                    if (codepoint < 0x80) {
                        char utf8[2] = {(char)codepoint, 0};
                        strbuf_append(&sb, utf8);
                    } else if (codepoint < 0x800) {
                        char utf8[3] = {
                            (char)(0xC0 | (codepoint >> 6)),
                            (char)(0x80 | (codepoint & 0x3F)),
                            0
                        };
                        strbuf_append(&sb, utf8);
                    } else {
                        char utf8[4] = {
                            (char)(0xE0 | (codepoint >> 12)),
                            (char)(0x80 | ((codepoint >> 6) & 0x3F)),
                            (char)(0x80 | (codepoint & 0x3F)),
                            0
                        };
                        strbuf_append(&sb, utf8);
                    }
                    break;
                }
                default:
                    ctx->error = "Invalid escape sequence";
                    strbuf_free(&sb);
                    return NULL;
            }
        } else if ((unsigned char)c < 0x20) {
            ctx->error = "Invalid control character in string";
            strbuf_free(&sb);
            return NULL;
        } else {
            char ch[2] = {c, 0};
            strbuf_append(&sb, ch);
        }
    }

    ctx->error = "Unterminated string";
    strbuf_free(&sb);
    return NULL;
}

static JsonValue* parse_number(ParseContext* ctx) {
    size_t start = ctx->pos;

    /* Optional negative sign */
    if (peek_char(ctx) == '-') {
        ctx->pos++;
    }

    /* Integer part */
    if (peek_char(ctx) == '0') {
        ctx->pos++;
    } else if (isdigit(peek_char(ctx))) {
        while (isdigit(peek_char(ctx))) {
            ctx->pos++;
        }
    } else {
        ctx->error = "Invalid number";
        return NULL;
    }

    /* Fractional part */
    if (peek_char(ctx) == '.') {
        ctx->pos++;
        if (!isdigit(peek_char(ctx))) {
            ctx->error = "Invalid number: expected digit after decimal";
            return NULL;
        }
        while (isdigit(peek_char(ctx))) {
            ctx->pos++;
        }
    }

    /* Exponent */
    int c = peek_char(ctx);
    if (c == 'e' || c == 'E') {
        ctx->pos++;
        c = peek_char(ctx);
        if (c == '+' || c == '-') {
            ctx->pos++;
        }
        if (!isdigit(peek_char(ctx))) {
            ctx->error = "Invalid number: expected digit in exponent";
            return NULL;
        }
        while (isdigit(peek_char(ctx))) {
            ctx->pos++;
        }
    }

    /* Parse the number */
    size_t len = ctx->pos - start;
    char* num_str = safe_strndup(ctx->json + start, len);
    if (!num_str) {
        ctx->error = "Memory allocation failed";
        return NULL;
    }

    double value = strtod(num_str, NULL);
    free(num_str);

    return json_number(value);
}

static JsonValue* parse_array(ParseContext* ctx) {
    if (next_char(ctx) != '[') {
        ctx->error = "Expected '['";
        return NULL;
    }

    JsonValue* arr = json_array();
    if (!arr) {
        ctx->error = "Memory allocation failed";
        return NULL;
    }

    skip_whitespace(ctx);

    if (peek_char(ctx) == ']') {
        ctx->pos++;
        return arr;
    }

    while (1) {
        skip_whitespace(ctx);

        JsonValue* item = parse_value(ctx);
        if (!item) {
            json_free(arr);
            return NULL;
        }

        if (json_array_append(arr, item) != 0) {
            json_free(item);
            json_free(arr);
            ctx->error = "Memory allocation failed";
            return NULL;
        }

        skip_whitespace(ctx);

        int c = peek_char(ctx);
        if (c == ']') {
            ctx->pos++;
            return arr;
        }

        if (c != ',') {
            json_free(arr);
            ctx->error = "Expected ',' or ']'";
            return NULL;
        }
        ctx->pos++;
    }
}

static JsonValue* parse_object(ParseContext* ctx) {
    if (next_char(ctx) != '{') {
        ctx->error = "Expected '{'";
        return NULL;
    }

    JsonValue* obj = json_object();
    if (!obj) {
        ctx->error = "Memory allocation failed";
        return NULL;
    }

    skip_whitespace(ctx);

    if (peek_char(ctx) == '}') {
        ctx->pos++;
        return obj;
    }

    while (1) {
        skip_whitespace(ctx);

        if (peek_char(ctx) != '"') {
            json_free(obj);
            ctx->error = "Expected string key";
            return NULL;
        }

        JsonValue* key_value = parse_string(ctx);
        if (!key_value) {
            json_free(obj);
            return NULL;
        }

        const char* key = json_get_string(key_value);

        skip_whitespace(ctx);

        if (next_char(ctx) != ':') {
            json_free(key_value);
            json_free(obj);
            ctx->error = "Expected ':'";
            return NULL;
        }

        skip_whitespace(ctx);

        JsonValue* value = parse_value(ctx);
        if (!value) {
            json_free(key_value);
            json_free(obj);
            return NULL;
        }

        if (json_object_set(obj, key, value) != 0) {
            json_free(key_value);
            json_free(value);
            json_free(obj);
            ctx->error = "Memory allocation failed";
            return NULL;
        }

        json_free(key_value);

        skip_whitespace(ctx);

        int c = peek_char(ctx);
        if (c == '}') {
            ctx->pos++;
            return obj;
        }

        if (c != ',') {
            json_free(obj);
            ctx->error = "Expected ',' or '}'";
            return NULL;
        }
        ctx->pos++;
    }
}

static JsonValue* parse_value(ParseContext* ctx) {
    skip_whitespace(ctx);

    int c = peek_char(ctx);
    if (c < 0) {
        ctx->error = "Unexpected end of input";
        return NULL;
    }

    if (c == '"') {
        return parse_string(ctx);
    }

    if (c == '[') {
        return parse_array(ctx);
    }

    if (c == '{') {
        return parse_object(ctx);
    }

    if (c == '-' || isdigit(c)) {
        return parse_number(ctx);
    }

    /* Check for keywords */
    if (ctx->pos + 4 <= ctx->max_len && strncmp(ctx->json + ctx->pos, "true", 4) == 0) {
        ctx->pos += 4;
        return json_bool(1);
    }

    if (ctx->pos + 5 <= ctx->max_len && strncmp(ctx->json + ctx->pos, "false", 5) == 0) {
        ctx->pos += 5;
        return json_bool(0);
    }

    if (ctx->pos + 4 <= ctx->max_len && strncmp(ctx->json + ctx->pos, "null", 4) == 0) {
        ctx->pos += 4;
        return json_null();
    }

    ctx->error = "Invalid JSON value";
    return NULL;
}

JsonValue* json_parse(const char* json, const char** error) {
    if (!json) {
        if (error) *error = "NULL input";
        return NULL;
    }
    return json_parse_n(json, strlen(json), error);
}

JsonValue* json_parse_n(const char* json, size_t max_len, const char** error) {
    if (!json) {
        if (error) *error = "NULL input";
        return NULL;
    }

    ParseContext ctx = {
        .json = json,
        .pos = 0,
        .max_len = max_len,
        .error = NULL
    };

    JsonValue* value = parse_value(&ctx);

    if (!value && error) {
        *error = ctx.error ? ctx.error : "Unknown error";
    }

    return value;
}

/* ============================================================================
 * JSON Serialization
 * ============================================================================ */

static int stringify_value(const JsonValue* value, StringBuffer* sb, const char* indent, int depth);

static void append_indent(StringBuffer* sb, const char* indent, int depth) {
    if (!indent) return;
    for (int i = 0; i < depth; i++) {
        strbuf_append(sb, indent);
    }
}

static int stringify_string(const char* str, StringBuffer* sb) {
    strbuf_append(sb, "\"");

    for (const char* p = str; *p; p++) {
        switch (*p) {
            case '"':  strbuf_append(sb, "\\\""); break;
            case '\\': strbuf_append(sb, "\\\\"); break;
            case '\b': strbuf_append(sb, "\\b"); break;
            case '\f': strbuf_append(sb, "\\f"); break;
            case '\n': strbuf_append(sb, "\\n"); break;
            case '\r': strbuf_append(sb, "\\r"); break;
            case '\t': strbuf_append(sb, "\\t"); break;
            default:
                if ((unsigned char)*p < 0x20) {
                    strbuf_appendf(sb, "\\u%04x", (unsigned char)*p);
                } else {
                    char ch[2] = {*p, 0};
                    strbuf_append(sb, ch);
                }
                break;
        }
    }

    strbuf_append(sb, "\"");
    return 0;
}

static int stringify_value(const JsonValue* value, StringBuffer* sb, const char* indent, int depth) {
    if (!value) {
        strbuf_append(sb, "null");
        return 0;
    }

    switch (value->type) {
        case JSON_NULL:
            strbuf_append(sb, "null");
            break;

        case JSON_BOOL:
            strbuf_append(sb, value->bool_value ? "true" : "false");
            break;

        case JSON_NUMBER: {
            /* Check if it's an integer */
            double d = value->number_value;
            if (d == (double)(long long)d && fabs(d) < 1e15) {
                strbuf_appendf(sb, "%lld", (long long)d);
            } else {
                strbuf_appendf(sb, "%.15g", d);
            }
            break;
        }

        case JSON_STRING:
            stringify_string(value->string_value ? value->string_value : "", sb);
            break;

        case JSON_ARRAY: {
            strbuf_append(sb, "[");
            size_t len = json_array_length(value);
            for (size_t i = 0; i < len; i++) {
                if (i > 0) strbuf_append(sb, ",");
                if (indent) {
                    strbuf_append(sb, "\n");
                    append_indent(sb, indent, depth + 1);
                }
                stringify_value(json_array_get(value, i), sb, indent, depth + 1);
            }
            if (indent && len > 0) {
                strbuf_append(sb, "\n");
                append_indent(sb, indent, depth);
            }
            strbuf_append(sb, "]");
            break;
        }

        case JSON_OBJECT: {
            strbuf_append(sb, "{");
            size_t count = json_object_count(value);
            for (size_t i = 0; i < count; i++) {
                if (i > 0) strbuf_append(sb, ",");
                if (indent) {
                    strbuf_append(sb, "\n");
                    append_indent(sb, indent, depth + 1);
                }
                stringify_string(json_object_key_at(value, i), sb);
                strbuf_append(sb, indent ? ": " : ":");
                stringify_value(json_object_value_at(value, i), sb, indent, depth + 1);
            }
            if (indent && count > 0) {
                strbuf_append(sb, "\n");
                append_indent(sb, indent, depth);
            }
            strbuf_append(sb, "}");
            break;
        }
    }

    return 0;
}

char* json_stringify(const JsonValue* value) {
    return json_stringify_pretty(value, NULL);
}

char* json_stringify_pretty(const JsonValue* value, const char* indent) {
    StringBuffer sb;
    if (strbuf_init(&sb, 256) != 0) {
        return NULL;
    }

    stringify_value(value, &sb, indent, 0);

    return strbuf_detach(&sb);
}

/* ============================================================================
 * JSON Clone
 * ============================================================================ */

JsonValue* json_clone(const JsonValue* value) {
    if (!value) {
        return NULL;
    }

    switch (value->type) {
        case JSON_NULL:
            return json_null();

        case JSON_BOOL:
            return json_bool(value->bool_value);

        case JSON_NUMBER:
            return json_number(value->number_value);

        case JSON_STRING:
            return json_string(value->string_value);

        case JSON_ARRAY: {
            JsonValue* arr = json_array();
            if (!arr) return NULL;
            for (size_t i = 0; i < json_array_length(value); i++) {
                JsonValue* item = json_clone(json_array_get(value, i));
                if (!item || json_array_append(arr, item) != 0) {
                    json_free(item);
                    json_free(arr);
                    return NULL;
                }
            }
            return arr;
        }

        case JSON_OBJECT: {
            JsonValue* obj = json_object();
            if (!obj) return NULL;
            for (size_t i = 0; i < json_object_count(value); i++) {
                JsonValue* v = json_clone(json_object_value_at(value, i));
                if (!v || json_object_set(obj, json_object_key_at(value, i), v) != 0) {
                    json_free(v);
                    json_free(obj);
                    return NULL;
                }
            }
            return obj;
        }
    }

    return NULL;
}
