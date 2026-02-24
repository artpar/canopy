#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// Simple JSON parser helpers — just enough for test args.
// All functions follow the uniform signature: const char* fn(const char* json_args)
// Input: JSON array of arguments.  Output: JSON value.  Caller frees result with free().

// math_add: adds two numbers from a JSON array [a, b] → returns the sum as JSON number.
const char* math_add(const char* json_args) {
    double a = 0, b = 0;
    // Parse "[a, b]" — skip '[', read two numbers separated by ','
    sscanf(json_args, "[%lf,%lf]", &a, &b);
    double result = a + b;

    char* buf = (char*)malloc(64);
    // Format as integer if whole, else as decimal
    if (result == (long long)result) {
        snprintf(buf, 64, "%lld", (long long)result);
    } else {
        snprintf(buf, 64, "%g", result);
    }
    return buf;
}

// string_reverse: reverses a JSON string from ["hello"] → "olleh"
const char* string_reverse(const char* json_args) {
    // Find the first quoted string in the array
    const char* start = strchr(json_args, '"');
    if (!start) {
        char* err = (char*)malloc(8);
        snprintf(err, 8, "\"\"");
        return err;
    }
    start++; // skip opening quote
    const char* end = strchr(start, '"');
    if (!end) {
        char* err = (char*)malloc(8);
        snprintf(err, 8, "\"\"");
        return err;
    }

    size_t len = end - start;
    // Result: "reversed_string" — need len + 2 quotes + null
    char* buf = (char*)malloc(len + 3);
    buf[0] = '"';
    for (size_t i = 0; i < len; i++) {
        buf[1 + i] = start[len - 1 - i];
    }
    buf[len + 1] = '"';
    buf[len + 2] = '\0';
    return buf;
}

// echo: returns the args array as-is (identity function for testing).
const char* echo(const char* json_args) {
    size_t len = strlen(json_args);
    char* buf = (char*)malloc(len + 1);
    memcpy(buf, json_args, len + 1);
    return buf;
}
