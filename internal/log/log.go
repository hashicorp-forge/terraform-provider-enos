// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package log

import (
	"context"

	"github.com/hashicorp/terraform-plugin-log/tfsdklog"
)

// Logger provides an interface to the terraform-plugin-log to make it easier to use.
type Logger interface {
	// With creates a new logger which will include the provided key/value in each log message
	With(key string, value interface{}) Logger
	// WithValues creates a new logger which will include all the provided key/value(s) in each log message
	WithValues(values map[string]interface{}) Logger
	Trace(msg string, additionalFields ...map[string]interface{})
	Debug(msg string, additionalFields ...map[string]interface{})
	Info(msg string, additionalFields ...map[string]interface{})
	Warn(msg string, additionalFields ...map[string]interface{})
	Error(msg string, additionalFields ...map[string]interface{})
}

func NewLogger(ctx context.Context) Logger {
	return &logger{ctx: ctx}
}

type logger struct {
	ctx context.Context
}

func (l *logger) With(key string, value interface{}) Logger {
	l.ctx = tfsdklog.SetField(l.ctx, key, value)

	return l
}

func (l *logger) WithValues(values map[string]interface{}) Logger {
	for key, value := range values {
		l.ctx = tfsdklog.SetField(l.ctx, key, value)
	}

	return l
}

func (l *logger) Trace(msg string, additionalFields ...map[string]interface{}) {
	tfsdklog.Trace(l.ctx, msg, additionalFields...)
}

func (l *logger) Debug(msg string, additionalFields ...map[string]interface{}) {
	tfsdklog.Debug(l.ctx, msg, additionalFields...)
}

func (l *logger) Info(msg string, additionalFields ...map[string]interface{}) {
	tfsdklog.Info(l.ctx, msg, additionalFields...)
}

func (l *logger) Warn(msg string, additionalFields ...map[string]interface{}) {
	tfsdklog.Warn(l.ctx, msg, additionalFields...)
}

func (l *logger) Error(msg string, additionalFields ...map[string]interface{}) {
	tfsdklog.Error(l.ctx, msg, additionalFields...)
}

type noopLogger struct{}

func NewNoopLogger() Logger {
	return noopLogger{}
}

func (n noopLogger) With(key string, value interface{}) Logger {
	return n
}

func (n noopLogger) WithValues(m map[string]interface{}) Logger {
	return n
}

func (n noopLogger) Trace(msg string, additionalFields ...map[string]interface{}) {
}

func (n noopLogger) Debug(msg string, additionalFields ...map[string]interface{}) {
}

func (n noopLogger) Info(msg string, additionalFields ...map[string]interface{}) {
}

func (n noopLogger) Warn(msg string, additionalFields ...map[string]interface{}) {
}

func (n noopLogger) Error(msg string, additionalFields ...map[string]interface{}) {
}
