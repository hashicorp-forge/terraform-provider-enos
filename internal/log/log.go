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
	With(key string, value any) Logger
	// WithValues creates a new logger which will include all the provided key/value(s) in each log message
	WithValues(values map[string]any) Logger
	Trace(msg string, additionalFields ...map[string]any)
	Debug(msg string, additionalFields ...map[string]any)
	Info(msg string, additionalFields ...map[string]any)
	Warn(msg string, additionalFields ...map[string]any)
	Error(msg string, additionalFields ...map[string]any)
}

func NewLogger(ctx context.Context) Logger {
	return &logger{ctx: ctx}
}

type logger struct {
	ctx context.Context
}

func (l *logger) With(key string, value any) Logger {
	l.ctx = tfsdklog.SetField(l.ctx, key, value)

	return l
}

func (l *logger) WithValues(values map[string]any) Logger {
	for key, value := range values {
		//nolint:fatcontext // we know we're making it chonky with extra logging
		l.ctx = tfsdklog.SetField(l.ctx, key, value)
	}

	return l
}

func (l *logger) Trace(msg string, additionalFields ...map[string]any) {
	tfsdklog.Trace(l.ctx, msg, additionalFields...)
}

func (l *logger) Debug(msg string, additionalFields ...map[string]any) {
	tfsdklog.Debug(l.ctx, msg, additionalFields...)
}

func (l *logger) Info(msg string, additionalFields ...map[string]any) {
	tfsdklog.Info(l.ctx, msg, additionalFields...)
}

func (l *logger) Warn(msg string, additionalFields ...map[string]any) {
	tfsdklog.Warn(l.ctx, msg, additionalFields...)
}

func (l *logger) Error(msg string, additionalFields ...map[string]any) {
	tfsdklog.Error(l.ctx, msg, additionalFields...)
}

type noopLogger struct{}

func NewNoopLogger() Logger {
	return noopLogger{}
}

func (n noopLogger) With(key string, value any) Logger {
	return n
}

func (n noopLogger) WithValues(m map[string]any) Logger {
	return n
}

func (n noopLogger) Trace(msg string, additionalFields ...map[string]any) {
}

func (n noopLogger) Debug(msg string, additionalFields ...map[string]any) {
}

func (n noopLogger) Info(msg string, additionalFields ...map[string]any) {
}

func (n noopLogger) Warn(msg string, additionalFields ...map[string]any) {
}

func (n noopLogger) Error(msg string, additionalFields ...map[string]any) {
}
