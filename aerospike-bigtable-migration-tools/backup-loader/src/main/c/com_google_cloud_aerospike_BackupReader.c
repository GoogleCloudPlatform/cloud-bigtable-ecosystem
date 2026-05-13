// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/**
 * We use an error handling convention similar to JNI:
 *
 * - If a non-void function doesn't return an error (e.g., by returning a
 *   non-null object), it did not leave the JVM in an exceptional state.
 * - If a non-void function returns an error (e.g., NULL), it may have left the
 *   JVM in an exceptional state — the caller should check for it.
 * - Void functions are assumed by default to be capable of leaving the JVM in
 *   an exceptional state, unless explicitly documented otherwise in comments.
 *   If a void function can cause an exception, the caller is responsible for
 *   detecting and handling it.
 *
 * Functions are unsafe to call in exception state unless explicitly documented
 * otherwise in comments.
 *
 * This code potentially creates a lot of Java objects since Aerospike records
 * can contain lists with millions of values, so we must be meticulous about
 * cleaning up local references to prevent JVM from running out of memory and/or
 * its local reference budget. Thus we clean up all the local references. Still,
 * we convert maps and lists using recursion, so extremely deeply nested maps
 * and lists could make us run out of stack space or memory.
 */

#include "com_google_cloud_aerospike_BackupReader.h"
#include "dec_text.h"
#include "restore.h"
#include <aerospike/as_bytes.h>
#include <aerospike/as_list.h>
#include <aerospike/as_map.h>
#include <aerospike/as_msgpack.h>
#include <aerospike/as_serializer.h>
#include <aerospike/as_val.h>
#include <ctype.h>
#include <jni.h>
#include <locale.h>
#include <math.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define LOG_ERROR_STDERR(fmt, ...) \
  fprintf(stderr, "(%s:%u): " fmt "\n", __func__, __LINE__, ##__VA_ARGS__)
#ifdef NDEBUG
#define LOG_DEBUG_STDERR(...) ((void)0)
#else
#define LOG_DEBUG_STDERR(fmt, ...) LOG_ERROR_STDERR(fmt, ##__VA_ARGS__)
#endif  // NDEBUG

#define FALLBACK_EXCEPTION_MESSAGE "JNI Error"

#define DEBUG logger_debug
#define WARN logger_warn
#define ERROR logger_error

#define JNI_VERSION JNI_VERSION_1_4

/**
 * Structure to pass arguments to the consume_list function.
 * Used when iterating over Aerospike lists to populate a Java ArrayList.
 * @field result In/out pointer to the Java ArrayList object to be populated.
 * @field env JNI environment pointer.
 */
typedef struct consume_list_args_s {
  jobject result;
  JNIEnv* env;
} consume_list_args;

/**
 * Structure to pass arguments to the consume_record function.
 * Used when iterating over Aerospike record bins to populate a Java
 * ReadRecordResult.
 * @field result In/out pointer to the Java ReadRecordResult object to be
 * populated.
 * @field env JNI environment pointer.
 */
typedef struct consume_record_args_s {
  jobject result;
  JNIEnv* env;
} consume_record_args;

/**
 * Structure to pass arguments to the consume_map function.
 * Used when iterating over Aerospike maps to populate a Java HashMap.
 * @field result in/out pointer to the Java HashMap object to be populated.
 * @field env JNI environment pointer.
 */
typedef struct consume_map_args_s {
  jobject result;
  JNIEnv* env;
} consume_map_args;

/**
 * Passes a log message to Java side by invoking an appropriate method
 * of the LOG field of BackupReader class. If called in an exception state, it
 * stores and rethrows the original exception.
 * @param env the JNI environment
 * @param level Logger method ID for logging at appropriate level.
 * @param format the format string
 * @param ... the arguments to the format string
 */
static void log_java(JNIEnv* env, jmethodID level, char const* format, ...);

/**
 * Initialize a class and store its reference in a global variable.
 * We need to create a GlobalRef so that it is kept between JNI calls.
 * This function will throw an exception if the class cannot be found.
 * @param env the JNI environment
 * @param clazz the class reference to be initialized
 * @param name the name of the class
 * @return 0 on success, non-zero on failure
 */
static int init_class(JNIEnv* env, jclass* clazz, char const* name);

/**
 * Initialize a method and store its reference in a global variable.
 * We don't create a GlobalRef because Method IDs remain valid until their
 * defining class is unloaded. This function will throw an exception if the
 * method cannot be found.
 * @param env the JNI environment
 * @param clazz the class reference
 * @param name the name of the method
 * @param signature the signature of the method
 * @param method_id the method ID to be initialized
 * @param is_static true if the method is static, false otherwise
 * @return 0 on success, non-zero on failure
 */
static int init_method(JNIEnv* env, jclass clazz, char const* name,
                       char const* signature, jmethodID* method_id,
                       bool is_static);

/**
 * Initialize a field and store its reference in a global variable.
 * We don't create a GlobalRef because field IDs remain valid until their
 * defining class is unloaded. This function will throw an exception if the
 * field cannot be found.
 * @param env the JNI environment
 * @param clazz the class reference
 * @param name the name of the field
 * @param signature the signature of the field
 * @param field_id the field ID to be initialized
 * @param is_static true if the method is static, false otherwise
 * @return 0 on success, non-zero on failure
 */
static int init_field(JNIEnv* env, jclass clazz, char const* name,
                      char const* signature, jfieldID* field_id,
                      bool is_static);

/**
 * Initialize a static object and store its reference in a global variable.
 * We need to create a GlobalRef so that it is kept between JNI calls.
 * This function will throw an exception if the object cannot be received or is
 * NULL.
 * @param env the JNI environment
 * @param clazz the class reference
 * @param field_id the ID of the field
 * @param field_name the name of the field, used for debug messages
 * @param out the object to be initialized with a global reference
 * @return 0 on success, non-zero on failure
 */
static int init_static_object(JNIEnv* env, jclass clazz, jfieldID field_id,
                              char const* field_name, jobject* out);

/**
 * Creates references to classes and methods as GlobalRefs and stores them in
 * static variables. If any of the init steps fail, this will throw an
 * exception.
 * @return 0 on success, non-zero on failure.
 */
static int init(JNIEnv* env);

/**
 * Deletes all the GlobalRefs created during init.
 * This function will not throw an exception.
 */
static void cleanup(JNIEnv* env);

/**
 * Creates a new Java String object from a UTF-8 encoded C string.
 * Uses the Java constructor String(byte[], Charset) with the UTF-8 charset,
 * because JNI's NewStringUTF uses a Modified UTF-8 encoding that is not fully
 * compatible with standard UTF-8. This ensures correct handling of all valid
 * UTF-8 data.
 *
 * @param env The JNI environment pointer.
 * @param str The UTF-8 encoded C string to convert.
 * @return A new Java String object, or NULL if creation fails.
 */
static jobject new_string_utf8(JNIEnv* env, char const* str);

/**
 * Returns a newly allocated C string containing the UTF-8 bytes of the given
 * Java String or NULL in case of an empty or NULL input Java string.
 * Uses str.getBytes(charset) to obtain the bytes, ensuring compatibility
 * with standard UTF-8. JNI's GetStringUTFChars uses a Modified UTF-8
 * encoding, so this approach is required for full UTF-8 support. The
 * returned pointer must be freed by the caller.
 *
 * @param env The JNI environment pointer.
 * @param str The Java String object.
 * @return A newly allocated C string, or NULL on error. Caller must free().
 */
static char* get_string_utf8_chars(JNIEnv* env, jobject str);

/**
 * Reads a record belonging to a given namespace from the backup file.
 * This function will not throw an exception.
 * @param env The JNI environment pointer
 * @param fd the io_read_proxy_t object holding the file descriptor
 * @param namespace the namespace to read
 * @param record in/out pointer to a stack-allocated (an assumption made by
 * text_parse() this functions wraps around; if this assumption is broken, it
 * might cause a memory leak) uninitialized as_record. It gets initialized (and
 * populated) by this function iff this function returns DECODER_RECORD. Caller
 * must call as_record_destroy() on it if this function returns DECODER_RECORD.
 * @return the status indicating if the read was successful
 */
static decoder_status read_record_impl(JNIEnv* env, io_read_proxy_t* fd,
                                       char* namespace, as_record* record);

/**
 * Consumes as_list values and appends them to the result object which is a Java
 * List. This function is called for each element in the Aerospike list.
 *
 * @param value The value to be added to the list.
 * @param udata Pointer to the consume_list_args containing the resulting Java
 * ArrayList.
 * @return true if the value was successfully added, false otherwise.
 */
static bool consume_list(as_val* value, void* udata);

/**
 * Consumes as_map entries and adds them to the result object which is a Java
 * Map. This function is called for each key-value pair in the Aerospike map.
 *
 * @param key The key of the map entry (must be a string).
 * @param value The value associated with the key.
 * @param udata Pointer to the consume_map_args containing the resulting Java
 * HashMap.
 * @return true if the entry was successfully added, false otherwise.
 */
static bool consume_map(as_val const* key, as_val const* value, void* udata);

/**
 * Converts an Aerospike value to a corresponding Java Object.
 *
 * @param env the JNI environment
 * @param value The Aerospike value to convert.
 * @return Java Object equivalent to the input value.
 */
static jobject create_java_obj_from_value(JNIEnv* env, as_val const* value);

/**
 * Deserializes a serialized Aerospike value from bytes back to its original
 * form. This function takes a raw AS_BYTES value containing serialized data and
 * attempts to deserialize it back to its original Aerospike value type using
 * the global aerospike_serializer. This function will not throw an exception.
 *
 * @param raw The raw Aerospike value containing serialized data.
 * @param deserialized Pointer to store the deserialized Aerospike value. The
 * caller is responsible for destroying this value with as_val_destroy().
 * @return true if deserialization was successful, false otherwise.
 */
static bool deserialize_value(as_bytes const* raw, as_val** deserialized);

/**
 * Consume a record and populate the result object.
 * @param binName the name of the bin
 * @param raw_value the value of the bin
 * @param udata pointer to the consume_record_args
 * @return true if the record was consumed successfully, false otherwise
 */
static bool consume_record(char const* bin_name, as_val const* raw_value,
                           void* udata);

/**
 * Throws a Java exception of the given class with the provided message
 * unless the code is already in exception state with one of the following
 * exceptions:
 * <ul>
 *   <li>UnsupportedBackupEntryException
 *   <li>MalformedBackupDataException
 *   <li>IOException
 * <ul/>
 * <p>If already in an exception state, it attempts to chain the old exception
 * as parent of the new exception.
 * <p>If the exception chaining fails, only the
 * new exception is thrown.
 * <p>If throwing the new exception fails, a default
 * JNIException is thrown.
 *
 * @param env The JNI environment pointer.
 * @param exc_cls The Java exception class to throw.
 * @param exc_str_ctor The method ID of the Java exception String constructor.
 * @param format format string.
 * @param ... format args.
 */
static void throw_exception(JNIEnv* env, jclass exc_cls, jmethodID exc_str_ctor,
                            char const* format, ...);

/**
 * Skips metadata lines (starting with '#') in the backup file.
 * If EOF is reached before any non-metadata line, logs and throws an
 * IOException.
 *
 * @param env The JNI environment pointer.
 * @param fd The io_read_proxy_t object holding the file descriptor.
 */
static void skip_metadata(JNIEnv* env, io_read_proxy_t* fd);

/**
 * Reads a line to verify the version string at the start of the backup file.
 * If the version string is invalid or missing, logs and throws an IOException.
 *
 * @param env The JNI environment pointer.
 * @param fd The io_read_proxy_t object holding the file descriptor.
 */
static void read_and_validate_version_string_line(JNIEnv* env,
                                                  io_read_proxy_t* fd);

/**
 * Create a ReadRecordResult object from an as_record.
 * @param env the JNI environment
 * @param backupReaderObj the BackupReader object
 * @param record the as_record to create the result from
 * @return the ReadRecordResult object
 */
static jobject create_result_from_record(JNIEnv* env, as_record* record);

/**
 * Skip to the next line in the input file.
 * @param env The JNI environment pointer.
 * @param fd the io_read_proxy_t object holding the file descriptor
 */
static void skip_to_next_line(JNIEnv* env, io_read_proxy_t* fd);

static jclass string_class = NULL;
static jmethodID string_ctor = NULL;
static jmethodID get_bytes = NULL;

static jclass throwable_class = NULL;
static jmethodID init_cause_method = NULL;
static jmethodID add_suppressed_method = NULL;

static jclass io_exception_class = NULL;
static jmethodID io_exception_str_ctor = NULL;

static jclass jni_exception_class = NULL;
static jmethodID jni_exception_str_ctor = NULL;

static jclass malformed_backup_data_exception_class = NULL;
static jmethodID malformed_backup_data_exception_str_ctor = NULL;

static jclass unsupported_backup_entry_exception_class = NULL;
static jmethodID unsupported_backup_entry_exception_str_ctor = NULL;

static jclass boolean_class = NULL;
static jmethodID boolean_ctor = NULL;

static jclass long_class = NULL;
static jmethodID long_ctor = NULL;

static jclass double_class = NULL;
static jmethodID double_ctor = NULL;

static jclass array_list_class = NULL;
static jmethodID array_list_ctor = NULL;
static jmethodID array_list_add = NULL;

static jclass hash_map_class = NULL;
static jmethodID hash_map_ctor = NULL;
static jmethodID hash_map_put = NULL;

static jclass read_record_result_class = NULL;
static jmethodID read_record_result_valid_ctor = NULL;
static jmethodID read_record_result_add_bin_value = NULL;

static jclass backup_reader_class = NULL;
static jmethodID backup_reader_value_create = NULL;
static jfieldID file_ptr_id = NULL;
static jfieldID charset_id = NULL;
static jfieldID log_id = NULL;
static jobject charset_obj = NULL;
static jobject log_obj = NULL;

static jclass logger_class = NULL;
static jmethodID logger_debug = NULL;
static jmethodID logger_warn = NULL;
static jmethodID logger_error = NULL;

static jobject fallback_exception_message = NULL;
static jobject fallback_jni_exception = NULL;

static as_serializer* aerospike_serializer = NULL;

static bool local_capacity_assurance_failed = false;

static void log_java(JNIEnv* env, jmethodID level, char const* format, ...) {
  char buffer[1024];
  va_list args;
  va_start(args, format);
  int format_retval = vsnprintf(buffer, sizeof(buffer), format, args);
  va_end(args);
  if (format_retval < 0) {
    LOG_ERROR_STDERR("Failed to format a log message using format string '%s'.",
                     format);
    return;
  }

  // Save the original exception if present and clear it
  jthrowable original_exc = (*env)->ExceptionOccurred(env);
  (*env)->ExceptionClear(env);
  jthrowable log_exc = NULL;

  jobject jstr = new_string_utf8(env, buffer);
  if (jstr != NULL) {
    (*env)->CallVoidMethod(env, log_obj, level, jstr);
  } else {
    LOG_ERROR_STDERR("Failed to create a Java log message from C string: '%s'.",
                     buffer);
  }

  // We may have failed to create the message string or log the message.
  // We may also have an original exception.
  // If there was an original exception, we want to add the log exception as a
  // suppressed one to the original one. If there was no original exception, we
  // don't touch the exception state so that we throw an exception if and only
  // if there was one during the logging.
  if (original_exc != NULL) {
    log_exc = (*env)->ExceptionOccurred(env);
    (*env)->ExceptionClear(env);
    (*env)->CallVoidMethod(env, original_exc, add_suppressed_method, log_exc);
    if ((*env)->ExceptionCheck(env) && log_exc != NULL) {
      LOG_ERROR_STDERR(
          "Failed to add a suppressed exception to the original exception.");
    }
    // If adding the suppressed exception failed, we just rethrow the original
    (*env)->ExceptionClear(env);
    if ((*env)->Throw(env, original_exc) != 0) {
      LOG_ERROR_STDERR(
          "Failed to rethrow the original exception after logging.");
      (*env)->FatalError(env, "Rethrowing an exception failed.");
    }
  }

  (*env)->DeleteLocalRef(env, jstr);
  (*env)->DeleteLocalRef(env, log_exc);
  (*env)->DeleteLocalRef(env, original_exc);
}

static int init_class(JNIEnv* env, jclass* clazz, char const* name) {
  LOG_DEBUG_STDERR("Initializing class '%s'...", name);
  jclass local_ref = (*env)->FindClass(env, name);
  if (local_ref == NULL) {
    LOG_ERROR_STDERR("Failed to find class '%s'.", name);
    return 1;
  }
  *clazz = (*env)->NewGlobalRef(env, local_ref);
  (*env)->DeleteLocalRef(env, local_ref);
  if (*clazz == NULL) {
    LOG_ERROR_STDERR("Failed to create global reference for class '%s'.", name);
    return 1;
  }
  return 0;
}

static int init_method(JNIEnv* env, jclass clazz, char const* name,
                       char const* signature, jmethodID* method_id,
                       bool is_static) {
  LOG_DEBUG_STDERR("Initializing method '%s' with sig: '%s'...", name,
                   signature);
  if (is_static) {
    *method_id = (*env)->GetStaticMethodID(env, clazz, name, signature);
  } else {
    *method_id = (*env)->GetMethodID(env, clazz, name, signature);
  }
  if (*method_id == NULL) {
    LOG_ERROR_STDERR("Failed to find method '%s'.", signature);
    return 1;
  }
  return 0;
}

static int init_field(JNIEnv* env, jclass clazz, char const* name,
                      char const* signature, jfieldID* field_id,
                      bool is_static) {
  LOG_DEBUG_STDERR("Initializing field '%s' with sig: '%s'...", name,
                   signature);
  if (is_static) {
    *field_id = (*env)->GetStaticFieldID(env, clazz, name, signature);
  } else {
    *field_id = (*env)->GetFieldID(env, clazz, name, signature);
  }
  if (*field_id == NULL) {
    LOG_ERROR_STDERR("Failed to find field '%s'.", signature);
    return 1;
  }
  return 0;
}

static int init_static_object(JNIEnv* env, jclass clazz, jfieldID field_id,
                              char const* field_name, jobject* out) {
  LOG_DEBUG_STDERR("Initializing static field '%s' value...", field_name);
  jobject value = (*env)->GetStaticObjectField(env, clazz, field_id);
  if (value == NULL) {
    LOG_ERROR_STDERR("Failed to get field '%s' value.", field_name);
    return 1;
  }
  *out = (*env)->NewGlobalRef(env, value);
  (*env)->DeleteLocalRef(env, value);
  if (*out == NULL) {
    LOG_ERROR_STDERR("Failed to create global reference for field '%s' value.",
                     field_name);
    return 1;
  }
  return 0;
}

#define GOTO_ERR_IF_NOT_0(expr) \
  do {                          \
    int _ret_code = (expr);     \
    if (_ret_code != 0) {       \
      goto err;                 \
    }                           \
  } while (0)

static int init(JNIEnv* env) {
  LOG_DEBUG_STDERR("JNI initialization...");

  // Initialization for new_string_utf8 dependencies (to paste into init)
  GOTO_ERR_IF_NOT_0(init_class(env, &string_class, "java/lang/String"));
  GOTO_ERR_IF_NOT_0(init_method(env, string_class, "<init>",
                                "([BLjava/nio/charset/Charset;)V", &string_ctor,
                                false));
  GOTO_ERR_IF_NOT_0(init_method(env, string_class, "getBytes",
                                "(Ljava/nio/charset/Charset;)[B", &get_bytes,
                                false));

  // Throwable and initCause initialization
  GOTO_ERR_IF_NOT_0(init_class(env, &throwable_class, "java/lang/Throwable"));
  GOTO_ERR_IF_NOT_0(init_method(env, throwable_class, "initCause",
                                "(Ljava/lang/Throwable;)Ljava/lang/Throwable;",
                                &init_cause_method, false));
  GOTO_ERR_IF_NOT_0(init_method(env, throwable_class, "addSuppressed",
                                "(Ljava/lang/Throwable;)V",
                                &add_suppressed_method, false));

  // Exceptions
  GOTO_ERR_IF_NOT_0(
      init_class(env, &io_exception_class, "java/io/IOException"));
  GOTO_ERR_IF_NOT_0(init_method(env, io_exception_class, "<init>",
                                "(Ljava/lang/String;)V", &io_exception_str_ctor,
                                false));

  GOTO_ERR_IF_NOT_0(
      init_class(env, &jni_exception_class,
                 "com/google/cloud/aerospike/exceptions/JNIException"));
  GOTO_ERR_IF_NOT_0(init_method(env, jni_exception_class, "<init>",
                                "(Ljava/lang/String;)V",
                                &jni_exception_str_ctor, false));

  GOTO_ERR_IF_NOT_0(init_class(
      env, &malformed_backup_data_exception_class,
      "com/google/cloud/aerospike/exceptions/MalformedBackupDataException"));
  GOTO_ERR_IF_NOT_0(init_method(env, malformed_backup_data_exception_class,
                                "<init>", "(Ljava/lang/String;)V",
                                &malformed_backup_data_exception_str_ctor,
                                false));

  GOTO_ERR_IF_NOT_0(init_class(
      env, &unsupported_backup_entry_exception_class,
      "com/google/cloud/aerospike/exceptions/UnsupportedBackupEntryException"));
  GOTO_ERR_IF_NOT_0(init_method(env, unsupported_backup_entry_exception_class,
                                "<init>", "(Ljava/lang/String;)V",
                                &unsupported_backup_entry_exception_str_ctor,
                                false));

  // Boolean
  GOTO_ERR_IF_NOT_0(init_class(env, &boolean_class, "java/lang/Boolean"));
  GOTO_ERR_IF_NOT_0(
      init_method(env, boolean_class, "<init>", "(Z)V", &boolean_ctor, false));

  // Integer
  GOTO_ERR_IF_NOT_0(init_class(env, &long_class, "java/lang/Long"));
  GOTO_ERR_IF_NOT_0(
      init_method(env, long_class, "<init>", "(J)V", &long_ctor, false));

  // Double
  GOTO_ERR_IF_NOT_0(init_class(env, &double_class, "java/lang/Double"));
  GOTO_ERR_IF_NOT_0(
      init_method(env, double_class, "<init>", "(D)V", &double_ctor, false));

  // ArrayList
  GOTO_ERR_IF_NOT_0(init_class(env, &array_list_class, "java/util/ArrayList"));
  GOTO_ERR_IF_NOT_0(init_method(env, array_list_class, "<init>", "(I)V",
                                &array_list_ctor, false));
  GOTO_ERR_IF_NOT_0(init_method(env, array_list_class, "add",
                                "(Ljava/lang/Object;)Z", &array_list_add,
                                false));

  // HashMap
  GOTO_ERR_IF_NOT_0(init_class(env, &hash_map_class, "java/util/HashMap"));
  GOTO_ERR_IF_NOT_0(init_method(env, hash_map_class, "<init>", "(I)V",
                                &hash_map_ctor, false));
  GOTO_ERR_IF_NOT_0(
      init_method(env, hash_map_class, "put",
                  "(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;",
                  &hash_map_put, false));

  // ReadRecordResult
  GOTO_ERR_IF_NOT_0(init_class(env, &read_record_result_class,
                               "com/google/cloud/aerospike/ReadRecordResult"));
  GOTO_ERR_IF_NOT_0(init_method(env, read_record_result_class, "<init>",
                                "([B)V", &read_record_result_valid_ctor,
                                false));
  GOTO_ERR_IF_NOT_0(init_method(
      env, read_record_result_class, "addBinValue",
      "(Ljava/lang/String;Lcom/google/cloud/aerospike/values/Value;)V",
      &read_record_result_add_bin_value, false));

  // BackupReader
  GOTO_ERR_IF_NOT_0(init_class(env, &backup_reader_class,
                               "com/google/cloud/aerospike/BackupReader"));
  GOTO_ERR_IF_NOT_0(init_method(
      env, backup_reader_class, "createValue",
      "(Ljava/lang/Object;)Lcom/google/cloud/aerospike/values/Value;",
      &backup_reader_value_create, true));
  GOTO_ERR_IF_NOT_0(init_field(env, backup_reader_class, "filePtr", "J",
                               &file_ptr_id, false));
  GOTO_ERR_IF_NOT_0(init_field(env, backup_reader_class, "charset",
                               "Ljava/nio/charset/Charset;", &charset_id,
                               true));
  GOTO_ERR_IF_NOT_0(init_static_object(env, backup_reader_class, charset_id,
                                       "charset", &charset_obj));
  GOTO_ERR_IF_NOT_0(init_field(env, backup_reader_class, "LOG",
                               "Lorg/slf4j/Logger;", &log_id, true));
  GOTO_ERR_IF_NOT_0(
      init_static_object(env, backup_reader_class, log_id, "LOG", &log_obj));

  // Logger
  GOTO_ERR_IF_NOT_0(init_class(env, &logger_class, "org/slf4j/Logger"));
  GOTO_ERR_IF_NOT_0(init_method(env, logger_class, "debug",
                                "(Ljava/lang/String;)V", &logger_debug, false));
  GOTO_ERR_IF_NOT_0(init_method(env, logger_class, "warn",
                                "(Ljava/lang/String;)V", &logger_warn, false));
  GOTO_ERR_IF_NOT_0(init_method(env, logger_class, "error",
                                "(Ljava/lang/String;)V", &logger_error, false));

  aerospike_serializer = as_msgpack_new();
  if (aerospike_serializer == NULL) {
    LOG_ERROR_STDERR("Failed to create Aerospike MessagePack (de)serializer.");
    goto err;
  }
  local_capacity_assurance_failed = false;

  // From now on, the init functions may assume that Java globals are
  // initialized.

  jobject fallback_string = new_string_utf8(env, FALLBACK_EXCEPTION_MESSAGE);
  if (fallback_string == NULL) {
    LOG_ERROR_STDERR("Failed to create fallback exception message.");
    goto err;
  }
  fallback_exception_message = (*env)->NewGlobalRef(env, fallback_string);
  (*env)->DeleteLocalRef(env, fallback_string);
  if (fallback_exception_message == NULL) {
    LOG_ERROR_STDERR(
        "Failed to create global reference for fallback exception message.");
    goto err;
  }

  jobject fallback_exception =
      (*env)->NewObject(env, jni_exception_class, jni_exception_str_ctor,
                        fallback_exception_message);
  if (fallback_exception == NULL) {
    LOG_ERROR_STDERR("Failed to create fallback exception.");
    goto err;
  }
  fallback_jni_exception = (*env)->NewGlobalRef(env, fallback_exception);
  (*env)->DeleteLocalRef(env, fallback_exception);
  if (fallback_jni_exception == NULL) {
    LOG_ERROR_STDERR(
        "Failed to create global reference for fallback exception.");
    goto err;
  }

  LOG_DEBUG_STDERR("JNI initialization... Done.");
  return 0;

err:
  cleanup(env);
  return 1;
}

#undef GOTO_ERR_IF_NOT_0

static void cleanup(JNIEnv* env) {
  (*env)->DeleteGlobalRef(env, string_class);
  (*env)->DeleteGlobalRef(env, throwable_class);
  (*env)->DeleteGlobalRef(env, io_exception_class);
  (*env)->DeleteGlobalRef(env, jni_exception_class);
  (*env)->DeleteGlobalRef(env, malformed_backup_data_exception_class);
  (*env)->DeleteGlobalRef(env, unsupported_backup_entry_exception_class);
  (*env)->DeleteGlobalRef(env, boolean_class);
  (*env)->DeleteGlobalRef(env, long_class);
  (*env)->DeleteGlobalRef(env, double_class);
  (*env)->DeleteGlobalRef(env, array_list_class);
  (*env)->DeleteGlobalRef(env, hash_map_class);
  (*env)->DeleteGlobalRef(env, read_record_result_class);
  (*env)->DeleteGlobalRef(env, backup_reader_class);
  (*env)->DeleteGlobalRef(env, charset_obj);
  (*env)->DeleteGlobalRef(env, log_obj);
  (*env)->DeleteGlobalRef(env, logger_class);
  (*env)->DeleteGlobalRef(env, fallback_exception_message);
  (*env)->DeleteGlobalRef(env, fallback_jni_exception);
  if (aerospike_serializer != NULL) {
    as_serializer_destroy(aerospike_serializer);
  }
  local_capacity_assurance_failed = false;
}

jint JNI_OnLoad(JavaVM* vm, void* reserved) {
  (void)reserved;
  JNIEnv* env = NULL;
  // Set the locale to C to avoid issues with number formatting
  setlocale(LC_NUMERIC, "C");

  if ((*vm)->GetEnv(vm, (void**)&env, JNI_VERSION) != JNI_OK) {
    LOG_ERROR_STDERR("Failed to initialize env.");
    return JNI_ERR;
  }

  if (init(env) != 0) {
    LOG_ERROR_STDERR("Failed to initialize classes.");
    return JNI_ERR;
  }

  return JNI_VERSION;
}

void JNI_OnUnload(JavaVM* vm, void* reserved) {
  (void)reserved;
  JNIEnv* env = NULL;
  if ((*vm)->GetEnv(vm, (void**)&env, JNI_VERSION) == JNI_OK) {
    cleanup(env);
  } else {
    LOG_ERROR_STDERR("Failed to initialize env.");
  }
}

static jobject new_string_utf8(JNIEnv* env, char const* str) {
  jobject result = NULL;
  jbyteArray bytes = NULL;

  // Convert the C string to a Java byte array
  jsize len = (jsize)MIN(strlen(str), INT32_MAX);
  bytes = (*env)->NewByteArray(env, len);
  if (bytes == NULL) {
    goto out;
  }
  (*env)->SetByteArrayRegion(env, bytes, 0, len, (jbyte const*)str);
  if ((*env)->ExceptionCheck(env)) {
    goto out;
  }

  // Create the Java String object
  result =
      (*env)->NewObject(env, string_class, string_ctor, bytes, charset_obj);

out:
  (*env)->DeleteLocalRef(env, bytes);
  return result;
}

static char* get_string_utf8_chars(JNIEnv* env, jobject str) {
  char* result = NULL;
  jbyteArray bytes = NULL;

  if (str == NULL) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "get_string_utf8_chars got null as argument.");
    goto out;
  }

  bytes =
      (jbyteArray)(*env)->CallObjectMethod(env, str, get_bytes, charset_obj);
  if (bytes == NULL || (*env)->ExceptionCheck(env)) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to get bytes from Java String.");
    goto out;
  }

  jsize len = (*env)->GetArrayLength(env, bytes);
  if ((*env)->ExceptionCheck(env)) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to get byte string's length.");
    goto out;
  }
  if (len == 0) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "get_string_utf8_chars got an empty string as argument.");
    goto out;
  }

  result = (char*)malloc(len + 1);
  if (result == NULL) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to allocate memory for UTF-8 string.");
    goto out;
  }
  result[len] = '\0';
  (*env)->GetByteArrayRegion(env, bytes, 0, len, (jbyte*)result);
  if ((*env)->ExceptionCheck(env)) {
    free(result);
    goto out;
  }

out:
  (*env)->DeleteLocalRef(env, bytes);
  return result;
}

decoder_status read_record_impl(JNIEnv* env, io_read_proxy_t* fd,
                                char* namespace, as_record* record) {
  // Read-only arguments to text_parse().
  as_vector* do_not_transform_namespaces = as_vector_create(sizeof(char*), 0);
  as_vector* do_not_filter_bins = as_vector_create(sizeof(char*), 0);

  decoder_status res = DECODER_ERROR;
  while (true) {
    index_param ip;
    udf_param up;
    bool is_expired = false;
    // Seems to be only used to make error messages more precise.
    uint32_t line_no = 0;

    res = text_parse(fd, false, do_not_transform_namespaces, do_not_filter_bins,
                     &line_no, record, 0, &is_expired, &ip, &up);
    if (res == DECODER_UDF) {
      free_udf(&up);
    } else if (res == DECODER_INDEX) {
      free_index(&ip);
    }

    // If the result isn't a record, we can return it immediately.
    if (res != DECODER_RECORD) {
      break;
    }

    // If the result is a record, we can return it only if it belongs
    // to the expected namespace, otherwise we need to read another entry.
    char* candidate_ns = record->key.ns;
    if (strlen(namespace) <= AS_NAMESPACE_MAX_SIZE &&
        strncmp(namespace, candidate_ns,
                MIN(strlen(namespace), AS_NAMESPACE_MAX_SIZE)) == 0) {
      break;
    }

    log_java(env, DEBUG, "Record with unexpected namespace '%.*s' encountered.",
             strnlen(candidate_ns, AS_NAMESPACE_MAX_SIZE), candidate_ns);
    (*env)->ExceptionClear(env);
    as_record_destroy(record);
  }

  as_vector_destroy(do_not_filter_bins);
  as_vector_destroy(do_not_transform_namespaces);

  return res;
}

static bool consume_list(as_val* value, void* udata) {
  consume_list_args* args = (consume_list_args*)udata;
  JNIEnv* env = args->env;
  jobject result = args->result;

  jobject java_value = NULL;
  bool success = false;

  java_value = create_java_obj_from_value(env, value);
  if (java_value == NULL) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to convert list element to Java Object.");
    goto out;
  }

  // Add the java_value to the ArrayList (result)
  (*env)->CallBooleanMethod(env, result, array_list_add, java_value);
  if ((*env)->ExceptionCheck(env)) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Error adding element to ArrayList.");
    goto out;
  }
  success = true;

out:
  (*env)->DeleteLocalRef(env, java_value);
  return success;
}

static bool consume_map(as_val const* key, as_val const* value, void* udata) {
  consume_map_args* args = (consume_map_args*)udata;
  JNIEnv* env = args->env;
  jobject result = args->result;

  jobject java_key = NULL;
  jobject java_value = NULL;
  bool success = false;

  java_key = create_java_obj_from_value(env, key);
  if (java_key == NULL) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to convert map key to Java Object.");
    goto out;
  }

  java_value = create_java_obj_from_value(env, value);
  if (java_value == NULL) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to convert map value to Java Object.");
    goto out;
  }

  // Add the key-value pair to the HashMap (result)
  (*env)->CallObjectMethod(env, result, hash_map_put, java_key, java_value);
  if ((*env)->ExceptionCheck(env)) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Exception occurred while adding entry to HashMap.");
    goto out;
  }
  success = true;

out:
  (*env)->DeleteLocalRef(env, java_value);
  (*env)->DeleteLocalRef(env, java_key);
  return success;
}

static jobject create_java_obj_from_value(JNIEnv* env, as_val const* value) {
  switch (as_val_type(value)) {
    case AS_BOOLEAN: {
      bool v = as_boolean_get((as_boolean const*)value);
      jobject java_value = (*env)->NewObject(env, boolean_class, boolean_ctor,
                                             v ? JNI_TRUE : JNI_FALSE);
      if (java_value == NULL) {
        throw_exception(
            env, jni_exception_class, jni_exception_str_ctor,
            "Failed to create a Java Boolean from AS_BOOLEAN value.");
        return NULL;
      }
      return java_value;
    }
    case AS_INTEGER: {
      jlong v = as_integer_get((as_integer const*)value);
      jobject java_value = (*env)->NewObject(env, long_class, long_ctor, v);
      if (java_value == NULL) {
        throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                        "Failed to create a Java Long from AS_INTEGER value.");
        return NULL;
      }
      return java_value;
    }
    case AS_DOUBLE: {
      jdouble v = as_double_get((as_double const*)value);
      jobject java_value = (*env)->NewObject(env, double_class, double_ctor, v);
      if (java_value == NULL) {
        throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                        "Failed to create a Java Double from AS_DOUBLE value.");
        return NULL;
      }
      return java_value;
    }
    case AS_STRING: {
      char const* v = as_string_get((as_string const*)value);
      if (v == NULL) {
        throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                        "NULL-valued AS_STRING encountered.");
        return NULL;
      }
      jobject java_value = new_string_utf8(env, v);
      if (java_value == NULL) {
        throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                        "Failed to create a Java String from AS_STRING value.");
        return NULL;
      }
      return java_value;
    }
    case AS_BYTES: {
      uint8_t* bytes = as_bytes_get((as_bytes const*)value);
      uint32_t size = as_bytes_size((as_bytes const*)value);
      if (bytes == NULL) {
        throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                        "NULL-valued AS_BYTES encountered.");
        return NULL;
      }
      jbyteArray jbytes = (*env)->NewByteArray(env, size);
      if (jbytes == NULL) {
        throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                        "Failed to create a new byte array.");
        return NULL;
      }
      (*env)->SetByteArrayRegion(env, jbytes, 0, size, (jbyte*)bytes);
      if ((*env)->ExceptionCheck(env)) {
        throw_exception(
            env, jni_exception_class, jni_exception_str_ctor,
            "Failed to copy bytes from AS_BYTES into a Java byte array.");
        (*env)->DeleteLocalRef(env, jbytes);
        return NULL;
      }
      return jbytes;
    }
    case AS_LIST: {
      as_list const* list = (as_list const*)value;
      jint list_size = MIN(as_list_size(list), INT32_MAX);
      jobject java_list =
          (*env)->NewObject(env, array_list_class, array_list_ctor, list_size);
      if (java_list == NULL) {
        throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                        "Failed to create an ArrayList.");
        return NULL;
      }
      consume_list_args list_args = {java_list, env};
      // We can call consume_list in a loop even though it might enter exception
      // state, because then it returns false and the loop is broken.
      if (!as_list_foreach(list, consume_list, (void*)&list_args)) {
        throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                        "Failed to convert a Java List from AS_LIST.");
        (*env)->DeleteLocalRef(env, java_list);
        return NULL;
      }
      return java_list;
    }
    case AS_MAP: {
      as_map const* map = (as_map const*)value;
      jint map_size = MIN(as_map_size(map), INT32_MAX);
      jobject java_map =
          (*env)->NewObject(env, hash_map_class, hash_map_ctor, map_size);
      if (java_map == NULL) {
        throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                        "Failed to create a HashMap.");
        return NULL;
      }
      consume_map_args map_args = {java_map, env};
      // We can call consume_map in a loop even though it might enter exception
      // state, because then it returns false and the loop is broken.
      if (!as_map_foreach(map, consume_map, (void*)&map_args)) {
        throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                        "Failed to convert a Java Map from AS_MAP.");
        (*env)->DeleteLocalRef(env, java_map);
        return NULL;
      }
      return java_map;
    }
    case AS_REC: {
      throw_exception(env, unsupported_backup_entry_exception_class,
                      unsupported_backup_entry_exception_str_ctor,
                      "AS_REC as_value encountered.");
      return NULL;
    }
    case AS_PAIR: {
      throw_exception(env, unsupported_backup_entry_exception_class,
                      unsupported_backup_entry_exception_str_ctor,
                      "AS_PAIR as_value encountered.");
      return NULL;
    }
    case AS_GEOJSON: {
      throw_exception(env, unsupported_backup_entry_exception_class,
                      unsupported_backup_entry_exception_str_ctor,
                      "AS_GEOJSON as_value encountered.");
      return NULL;
    }
    default: {
      throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                      "'%d'-type as_value encountered.", as_val_type(value));
      return NULL;
    }
  }
}

static bool deserialize_value(as_bytes const* raw, as_val** deserialized) {
  uint8_t* bytes = as_bytes_get(raw);
  uint32_t size = as_bytes_size(raw);
  as_buffer buf;
  buf.data = bytes;
  buf.size = size;
  buf.capacity = size;

  if (as_serializer_deserialize(aerospike_serializer, &buf, deserialized) !=
      AEROSPIKE_OK) {
    return false;
  }
  return true;
}

static bool consume_record(char const* bin_name, as_val const* raw_value,
                           void* udata) {
  consume_record_args* const args = (consume_record_args*)udata;
  JNIEnv* env = args->env;
  jobject result = args->result;
  as_val const* value = NULL;

  as_val* deserialized_value = NULL;
  jobject jbin_name = NULL;
  jobject java_value = NULL;
  jobject wrapped_value = NULL;
  bool success = false;

  as_bytes* raw_bytes = as_bytes_fromval(raw_value);
  if (raw_bytes != NULL &&
      (raw_bytes->type == AS_BYTES_MAP || raw_bytes->type == AS_BYTES_LIST)) {
    if (deserialize_value(raw_bytes, &deserialized_value)) {
      value = deserialized_value;
    } else {
      throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                      "Failed to deserialize value for bin: %s.", bin_name);
      goto out;
    }
  } else {
    value = raw_value;
  }

  jbin_name = new_string_utf8(env, bin_name);
  if (jbin_name == NULL) {
    throw_exception(
        env, jni_exception_class, jni_exception_str_ctor,
        "Failed to convert bin name string to Java String for bin: %s.",
        bin_name);
    goto out;
  }
  java_value = create_java_obj_from_value(env, value);
  if ((*env)->ExceptionCheck(env)) {
    throw_exception(
        env, jni_exception_class, jni_exception_str_ctor,
        "Failed to convert an Aerospike value into a Java one for bin: %s.",
        bin_name);
    goto out;
  }
  wrapped_value = (*env)->CallStaticObjectMethod(
      env, backup_reader_class, backup_reader_value_create, java_value);
  if (wrapped_value == NULL || (*env)->ExceptionCheck(env)) {
    throw_exception(
        env, jni_exception_class, jni_exception_str_ctor,
        "Failed to wrap a Java value into a Value wrapper for bin: %s.",
        bin_name);
    goto out;
  }
  (*env)->CallVoidMethod(env, result, read_record_result_add_bin_value,
                         jbin_name, wrapped_value);
  if ((*env)->ExceptionCheck(env)) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Exception occurred while adding bin value for bin: %s.",
                    bin_name);
    goto out;
  }
  success = true;

out:
  (*env)->DeleteLocalRef(env, wrapped_value);
  (*env)->DeleteLocalRef(env, java_value);
  (*env)->DeleteLocalRef(env, jbin_name);
  if (deserialized_value != NULL) {
    as_val_destroy(deserialized_value);
  }
  return success;
}

static void throw_exception(JNIEnv* env, jclass exc_cls, jmethodID exc_str_ctor,
                            char const* format, ...) {
  jobject maybe_original_exc = NULL;
  jobject java_message = NULL;
  jobject exc = NULL;

  char buffer[1024];
  va_list args;
  va_start(args, format);
  int format_retval = vsnprintf(buffer, sizeof(buffer), format, args);
  va_end(args);
  char const* c_message = format;
  if (format_retval >= 0) {
    c_message = buffer;
  }

  maybe_original_exc = (*env)->ExceptionOccurred(env);
  (*env)->ExceptionClear(env);

  java_message = new_string_utf8(env, c_message);
  if (java_message == NULL) {
    (*env)->ExceptionClear(env);
    log_java(env, ERROR,
             "Failed to convert exception message to Java. Using fallback text "
             "of '%s'.",
             FALLBACK_EXCEPTION_MESSAGE);
    (*env)->ExceptionClear(env);
    java_message = fallback_exception_message;
  }

  exc = (*env)->NewObject(env, exc_cls, exc_str_ctor, java_message);
  if (exc != NULL) {
    if (maybe_original_exc != NULL) {
      // We used string constructor, so cause is still uninitialized.
      (*env)->CallObjectMethod(env, exc, init_cause_method, maybe_original_exc);
      if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        log_java(env, ERROR, "Failed to set an exception's cause.");
        (*env)->ExceptionClear(env);
      }
    }
  } else {
    (*env)->ExceptionClear(env);
    log_java(env, ERROR, "Failed to construct an exception with message '%s'.",
             c_message);
    (*env)->ExceptionClear(env);
  }

  // On one hand, we want to give the user well annotated errors, so we often
  // wrap exceptions. On the other hand, we want to give the user meaningful
  // top-level exception classes so that they can selectively catch them.
  // So we need to avoid wrapping exceptions whose class is important by itself.
  bool entry_is_unsupported = (*env)->IsInstanceOf(
      env, maybe_original_exc, unsupported_backup_entry_exception_class);
  (*env)->ExceptionClear(env);
  bool data_is_malformed = (*env)->IsInstanceOf(
      env, maybe_original_exc, malformed_backup_data_exception_class);
  (*env)->ExceptionClear(env);
  bool io_exception_happened =
      (*env)->IsInstanceOf(env, maybe_original_exc, io_exception_class);
  (*env)->ExceptionClear(env);
  bool skip_exception_wrapping =
      maybe_original_exc != NULL &&
      (entry_is_unsupported || data_is_malformed || io_exception_happened);
  if (skip_exception_wrapping) {
    log_java(
        env, DEBUG,
        "Skipping wrapping an exception with an exception with message '%s'.",
        c_message);
    (*env)->ExceptionClear(env);
  }

  // This is the main error handling logic used by all the native methods.
  // Some of them (such as `readRecord()`) are problematic because they
  // sometimes return `null` as a valid output, so if throwing an exception
  // failed, the `NULL`  return value that normally accompanies an exception
  // could be misinterpreted. To prevent this, we use more complex logic to
  // ensure all possible errors are caught and thrown properly, so that failure
  // is never silently ignored.
  if (!skip_exception_wrapping && exc != NULL) {
    if ((*env)->Throw(env, exc) == 0) {
      goto out;
    }
    log_java(env, ERROR, "Failed to throw an exception with message '%s'.",
             c_message);
    (*env)->ExceptionClear(env);
    if ((*env)->ThrowNew(env, exc_cls, c_message) == 0) {
      goto out;
    }
    log_java(env, ERROR,
             "Failed to throw an exception with message '%s' using "
             "fallback method.",
             c_message);
    (*env)->ExceptionClear(env);
  }
  if (maybe_original_exc != NULL) {
    if ((*env)->Throw(env, maybe_original_exc) == 0) {
      goto out;
    }
    log_java(env, ERROR,
             "Failed to throw original exception when throwing an exception "
             "with message '%s'.",
             c_message);
    (*env)->ExceptionClear(env);
  }
  if ((*env)->Throw(env, fallback_jni_exception) == 0) {
    goto out;
  }
  log_java(env, ERROR, "Failed to throw fallback exception.");
  (*env)->ExceptionClear(env);
  // Returning from the function without a thrown exception could violate
  // correctness. We must throw something, even at the price of a crash.
  (*env)->FatalError(env, "Throwing an exception failed.");
out:
  (*env)->DeleteLocalRef(env, exc);
  (*env)->DeleteLocalRef(env, java_message);
  (*env)->DeleteLocalRef(env, maybe_original_exc);
}

static jobject create_result_from_record(JNIEnv* env, as_record* record) {
  jobject result = NULL;
  jbyteArray digest = (*env)->NewByteArray(env, AS_DIGEST_VALUE_SIZE);
  if (digest == NULL) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to create digest byte array.");
    goto out;
  }
  as_digest* digest_obj = as_key_digest(&record->key);
  if (digest_obj == NULL) {
    throw_exception(env, malformed_backup_data_exception_class,
                    malformed_backup_data_exception_str_ctor,
                    "Record's digest is NULL.");
    goto out;
  }
  if (!digest_obj->init) {
    throw_exception(env, malformed_backup_data_exception_class,
                    malformed_backup_data_exception_str_ctor,
                    "Encountered an uninitialzied digest.");
    goto out;
  }
  (*env)->SetByteArrayRegion(env, digest, 0, AS_DIGEST_VALUE_SIZE,
                             (jbyte*)digest_obj->value);
  if ((*env)->ExceptionCheck(env)) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to set digest byte array region.");
    goto out;
  }

  // Create the ReadRecordResult object
  jobject partial_result = (*env)->NewObject(
      env, read_record_result_class, read_record_result_valid_ctor, digest);
  if (partial_result == NULL) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to create ReadRecordResult object.");
    goto out;
  }

  // Iterate over the record bins and populate the result
  consume_record_args args = {partial_result, env};
  // We can call consume_record in a loop even though it might enter exception
  // state, because then it returns false and the loop is broken.
  if (!as_record_foreach(record, consume_record, (void*)&args)) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Error occurred while iterating over record bins.");
    (*env)->DeleteLocalRef(env, partial_result);
    goto out;
  }
  result = partial_result;

out:
  (*env)->DeleteLocalRef(env, digest);
  return result;
}

/**
 * JNI method to open a backup file for reading.
 * The method initializes the io_read_proxy_t object and stores a pointer
 * in the BackupReader.filePtr field.
 */
JNIEXPORT void JNICALL Java_com_google_cloud_aerospike_BackupReader_openFile(
    JNIEnv* env, jobject obj, jobject java_file_path, jboolean use_zstd_comp) {
  char* file_path = NULL;
  // NOLINTBEGIN(performance-no-int-to-ptr)
  io_read_proxy_t* fd =
      (io_read_proxy_t*)(*env)->GetLongField(env, obj, file_ptr_id);
  // NOLINTEND(performance-no-int-to-ptr)
  if ((*env)->ExceptionCheck(env)) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to get filePtr field.");
    return;
  }
  if (fd != NULL) {
    throw_exception(env, io_exception_class, io_exception_str_ctor,
                    "File already open.");
    return;
  }

  fd = (io_read_proxy_t*)malloc(sizeof(io_read_proxy_t));
  if (fd == NULL) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to allocate memory for a file descriptor.");
    goto cleanup_allocations;
  }

  file_path = get_string_utf8_chars(env, java_file_path);
  if (file_path == NULL) {
    throw_exception(
        env, jni_exception_class, jni_exception_str_ctor,
        "Failed to convert the file path to be opened to a C String.");
    goto cleanup_allocations;
  }

  int file_loaded_status = io_read_proxy_init(fd, file_path);
  if (file_loaded_status != 0) {
    throw_exception(env, io_exception_class, io_exception_str_ctor,
                    "Failed to open file '%s'. Status: %d.", file_path,
                    file_loaded_status);
    goto cleanup_allocations;
  }

  compression_opt c_opt =
      use_zstd_comp ? IO_PROXY_COMPRESS_ZSTD : IO_PROXY_COMPRESS_NONE;
  io_proxy_init_compression(fd, c_opt);

  (*env)->SetLongField(env, obj, file_ptr_id, (jlong)fd);
  if ((*env)->ExceptionCheck(env)) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to set filePtr field for file '%s'.", file_path);
    goto cleanup_file;
  }

  read_and_validate_version_string_line(env, fd);
  if ((*env)->ExceptionCheck(env)) {
    goto cleanup_file;
  }

  skip_metadata(env, fd);
  if ((*env)->ExceptionCheck(env)) {
    goto cleanup_file;
  }

  log_java(env, DEBUG, "File '%s' opened successfully.", file_path);
  (*env)->ExceptionClear(env);
  free(file_path);
  return;

cleanup_file:
  if (io_proxy_close(fd) == EOF) {
    LOG_ERROR_STDERR(
        "Failed to close opened backup file. Might leak resources.");
  }
cleanup_allocations:
  free(fd);
  free(file_path);
}

/**
 * JNI method to close the backup file.
 * The method closes the file descriptor and frees the memory.
 */
JNIEXPORT void JNICALL Java_com_google_cloud_aerospike_BackupReader_closeFile(
    JNIEnv* env, jobject obj) {
  // NOLINTBEGIN(performance-no-int-to-ptr)
  io_read_proxy_t* fd =
      (io_read_proxy_t*)(*env)->GetLongField(env, obj, file_ptr_id);
  // NOLINTEND(performance-no-int-to-ptr)
  if ((*env)->ExceptionCheck(env)) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to get filePtr field.");
    return;
  }
  if (fd == NULL) {
    log_java(env, WARN,
             "Failed close the reader file, which is already closed.");
    (*env)->ExceptionClear(env);
    return;
  }

  (*env)->SetLongField(env, obj, file_ptr_id, (jlong)NULL);
  if ((*env)->ExceptionCheck(env)) {
    throw_exception(
        env, jni_exception_class, jni_exception_str_ctor,
        "Couldn't reset the filePtr field. File may still be open.");
    return;
  }

  if (io_proxy_close(fd) == EOF) {
    throw_exception(env, io_exception_class, io_exception_str_ctor,
                    "Error while closing backup file.");
    goto out;
  }

  log_java(env, DEBUG, "File closed successfully.");
  (*env)->ExceptionClear(env);

out:
  free(fd);
}

static void skip_to_next_line(JNIEnv* env, io_read_proxy_t* fd) {
  int32_t ch;
  do {
    ch = io_proxy_getc_unlocked(fd);
  } while (ch != EOF && ch != '\n');
  if (ch == EOF && io_proxy_error(fd) != 0) {
    throw_exception(env, io_exception_class, io_exception_str_ctor,
                    "Failed to skip line, error reading file.");
  }
}

static void skip_metadata(JNIEnv* env, io_read_proxy_t* fd) {
  int32_t ch = io_proxy_peekc_unlocked(fd);
  while (ch == '#') {
    skip_to_next_line(env, fd);
    if ((*env)->ExceptionCheck(env)) {
      return;
    }
    ch = io_proxy_peekc_unlocked(fd);
  }
  if (ch == EOF && io_proxy_error(fd) != 0) {
    throw_exception(env, io_exception_class, io_exception_str_ctor,
                    "Failed to read metadata, error reading file.");
  }
}

static void read_and_validate_version_string_line(JNIEnv* env,
                                                  io_read_proxy_t* fd) {
  // Version string has always exactly 12 characters and looks like this
  // "Version <3 chars for version number>\n". For more info please refer to
  // https://github.com/aerospike/aerospike-tools-backup?tab=readme-ov-file#backup-file-format
  char version[13];
  if (io_proxy_gets(fd, version, sizeof(version)) == NULL) {
    throw_exception(env, io_exception_class, io_exception_str_ctor,
                    "Failed to read backup header, an unknown error.");
    return;
  }

  if (strncmp("Version ", version, 8) != 0 || version[11] != '\n' ||
      version[12] != '\0') {
    throw_exception(env, malformed_backup_data_exception_class,
                    malformed_backup_data_exception_str_ctor,
                    "Invalid backup header.");
    return;
  }
  if (strncmp("3.1", version + 8, 3) != 0) {
    throw_exception(env, malformed_backup_data_exception_class,
                    malformed_backup_data_exception_str_ctor,
                    "Unknown version in header: '%s'", version);
    return;
  }
}

/**
 * JNI method to read a record from the backup file.
 * The method reads a record from the file and returns a ReadRecordResult
 * object.
 */
JNIEXPORT jobject JNICALL
Java_com_google_cloud_aerospike_BackupReader_readRecord(JNIEnv* env,
                                                        jobject obj,
                                                        jobject ns) {
  // This call might be needed in JVMs that do require that
  // EnsureLocalCapacity() is called before more than standard-specified [0] 16
  // local references can be created. In practice, OpenJDK distributions seem to
  // allow creating local references until the very OOM.
  // The actual number of references requested by us here is arbitrary (it
  // can be only reached when creating a very deeply nested map or list), but we
  // picked it expecting that a JVM that does require this call is also likely
  // to disallow allocating a large number of local references with it.
  //
  // [0]
  // https://docs.oracle.com/en/java/javase/17/docs/specs/jni/functions.html#ensurelocalcapacity
  jint local_capacity_assurance = (*env)->EnsureLocalCapacity(env, 250);
  (*env)->ExceptionClear(env);
  if (local_capacity_assurance != JNI_OK && !local_capacity_assurance_failed) {
    log_java(env, WARN, "Failed to ensure a local reference capacity.");
    (*env)->ExceptionClear(env);
    local_capacity_assurance_failed = true;
  }

  jobject return_value = NULL;
  as_record record;
  char* namespace = get_string_utf8_chars(env, ns);
  if (namespace == NULL) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Failed to retrieve namespace string.");
    goto out;
  }
  if (strchr(namespace, ',')) {
    // asrestore lets you map input namespace into output namespace when
    // loading a backup into an Aerospike instance. However, BackupReader
    // returns Java Objects, hence no need for such mapping. Thus, it does
    // not support multiple namespaces in the namespace string.
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Multiple namespaces found in namespace string.");
    goto out;
  }

  // NOLINTBEGIN(performance-no-int-to-ptr)
  io_read_proxy_t* fd =
      (io_read_proxy_t*)(*env)->GetLongField(env, obj, file_ptr_id);
  // NOLINTEND(performance-no-int-to-ptr)
  if (fd == NULL) {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "File is closed! Please open the file first.");
    goto out;
  }

  decoder_status result = read_record_impl(env, fd, namespace, &record);

  if (result == DECODER_RECORD) {
    return_value = create_result_from_record(env, &record);
  } else if (result == DECODER_ERROR) {
    // We want to allow the user to continue reading backup file when
    // encountering a malformed backup file. Unfortunately, sometimes
    // `text_parse()` returns DECODER_ERROR without advancing `fd`. Thus we need
    // to force it to advance somehow to prevent the user from looping
    // endlessly. Here we use a property of a valid backup file: a valid record
    // is preceded by a `'\n'`, so we can skip to the next line and - possibly
    // after a few further skips - arrive to a valid record.
    //
    // Note that the backup reader already failed with DECODER_ERROR, so we know
    // that we are reading an invalid file. This mode is a best-effort fallback,
    // which - as seen in the unit tests - can sometimes recover cleanly, but
    // can as well keep throwing MalformedBackupDataException or be tricked
    // by a maliciously crafted file to read records present in the original
    // file only as string values of some bins.
    skip_to_next_line(env, fd);
    (*env)->ExceptionClear(env);
    log_java(env, DEBUG, "Encountered a decoder error, skipping a line.");
    (*env)->ExceptionClear(env);
    throw_exception(env, malformed_backup_data_exception_class,
                    malformed_backup_data_exception_str_ctor,
                    "Couldn't read the record.");
  } else if (result == DECODER_EOF) {
    log_java(env, DEBUG, "End of file reached.");
    (*env)->ExceptionClear(env);
  } else if (result == DECODER_INDEX) {
    throw_exception(env, unsupported_backup_entry_exception_class,
                    unsupported_backup_entry_exception_str_ctor,
                    "Index entry encountered.");
  } else if (result == DECODER_UDF) {
    throw_exception(env, unsupported_backup_entry_exception_class,
                    unsupported_backup_entry_exception_str_ctor,
                    "UDF entry encountered.");
  } else {
    throw_exception(env, jni_exception_class, jni_exception_str_ctor,
                    "Unsupported decoder status returned: %u.", result);
  }

out:
  free(namespace);
  if (result == DECODER_RECORD) {
    as_record_destroy(&record);
  }
  return return_value;
}
