/*
 * Copyright (C) 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy of
 * the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
 * License for the specific language governing permissions and limitations under
 * the License.
 */

package utility

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

var Logger = log.New(os.Stdout, "", log.LstdFlags)

// SetupLogger initializes logging to both a file and console if file logging is enabled.
func SetupLogger(logFileName string, enableFileLogging bool) (*os.File, error) {
	var logFile *os.File
	var err error
	if enableFileLogging {
		if logFileName == "" {
			logFileName = fmt.Sprintf("test_result_%d.log", time.Now().Unix())
		}
		logFile, err = setupLogging(logFileName)
		if err != nil {
			return nil, fmt.Errorf("failed to set up logging: %v", err)
		}
		Logger = log.New(io.MultiWriter(logFile, os.Stdout), "", log.LstdFlags)
	}
	return logFile, nil
}

func setupLogging(logPath string) (*os.File, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	// Log to both console and file
	multi := io.MultiWriter(logFile, os.Stdout)
	log.SetOutput(multi)
	return logFile, nil
}

// LogSuccess logs a successful query execution with optional test integration.
func LogSuccess(t *testing.T, opDesc, queryType string) {
	msg := fmt.Sprintf("✅ Passed (%s): %s\n", strings.ToUpper(queryType), opDesc)
	Logger.Print(msg)
	if t != nil {
		t.Log(msg) // Also log in the testing framework if `t` is provided
	}
}

// LogFailure logs a query execution failure with expected vs actual differences.
func LogFailure(t *testing.T, filename, opDesc, query, diff string) {
	msg := fmt.Sprintf("❌ Failed(%s) \nDescription:%s \nQuery: %s\nDifference: %s\n", filename, opDesc, query, diff)
	Logger.Print(msg)
	if t != nil {
		t.Errorf("%s", msg)
	}
}

// LogError logs an error when query execution fails.
func LogError(t *testing.T, filename, opDesc, query string, err error) {
	msg := fmt.Sprintf("\n❌ Error(%s)\nDescription:%s\nQuery: %s\nError: %v\n", filename, opDesc, query, err)
	Logger.Print(msg)
	if t != nil {
		t.Errorf("%s", msg)
	}
}

// LogWarning logs warnings or unexpected but non-fatal conditions.
func LogWarning(t *testing.T, msg string) {
	Logger.Printf("\n❌ %s\n", msg)
	if t != nil {
		t.Errorf("%s", msg)
	}
}

// LogWarning logs warnings or unexpected but non-fatal conditions.
func LogTestFatal(t *testing.T, msg string) {
	Logger.Printf("\n❌ %s\n", msg)
	if t != nil {
		t.Fatalf("%s", msg)
	}
}

// LogWarning logs warnings or unexpected but non-fatal conditions.
func LogTestError(msg string) {
	Logger.Printf("\n❌ %s\n", msg)

}

// LogInfo logs general info messages.
func LogInfo(msg string) {
	Logger.Printf("\nℹ️ %s\n", msg)
}

// LogFatal logs a critical error message and exits the application.
func LogFatal(msg string) {
	Logger.Printf("\n❌ %s\n", msg)
}

// LogSectionDivider logs a section divider for test case separation.
func LogSectionDivider() {
	Logger.Println("\n-------------------------------------------------------")
}
