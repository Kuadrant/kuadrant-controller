/*
Copyright 2021 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package log

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	// Log is the base logger
	Log logr.Logger = ctrllog.NullLogger{}
)

// SetLogger sets a concrete logging implementation for all deferred Loggers.
// Being top level application and not a library or dependency,
// let's delegation logger used by any dependency
func SetLogger(logger logr.Logger) {
	Log = logger

	ctrl.SetLogger(logger) // fulfills `logger` as the de facto logger used by controller-runtime
	klog.SetLogger(logger)
}

// ToLevel converts a string to a log level.
func ToLevel(level string) zapcore.Level {
	var l zapcore.Level
	err := l.UnmarshalText([]byte(level))
	if err != nil {
		panic(err)
	}
	return l
}

// Mode defines the log output mode.
type Mode int8

const (
	// ModeProd is the log mode for production.
	ModeProd Mode = iota
	// ModeDev is for more human-readable outputs, extra stack traces
	// and logging info. (aka Zap's "development config".)
	// https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/log/zap#UseDevMode
	ModeDev
)

// ToMode converts a string to a log mode.
// Use either 'production' for `LogModeProd` or 'development' for `LogModeDev`.
func ToMode(mode string) Mode {
	switch strings.ToLower(mode) {
	case "production":
		return ModeProd
	case "development":
		return ModeDev
	default:
		panic(fmt.Sprintf("unknown log mode: %s", mode))
	}
}
