package infrastructure

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a singleton wrapper for zap logger
type Logger struct {
	zapLogger *zap.Logger
}

var (
	instance *Logger
	once     sync.Once
)

// GetLogger returns the singleton instance of Logger
func GetLogger() *Logger {
	once.Do(func() {
		runtimeEnvironment := os.Getenv("ENVIRONMENT")
		isProduction := runtimeEnvironment == "production"

		// Set the log level based on the environment variable
		var logLevel zapcore.Level

		if isProduction {
			logLevel = zapcore.InfoLevel
		} else {
			logLevel = zapcore.DebugLevel
		}

		// Configure zap logger
		config := zap.Config{
			Level:       zap.NewAtomicLevelAt(logLevel),
			Development: false,
			Sampling: &zap.SamplingConfig{
				Initial:    100,
				Thereafter: 100,
			},
			Encoding:         "json",
			EncoderConfig:    newEncoderConfig(),
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		}

		logger, err := config.Build(zap.AddCallerSkip(1))
		if err != nil {
			panic("Failed to initialize logger: " + err.Error())
		}

		instance = &Logger{
			zapLogger: logger,
		}
	})

	return instance
}

// newEncoderConfig creates an encoder config with reasonable defaults
func newEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// Info logs a message at Info level
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.zapLogger.Info(msg, fields...)
}

// Debug logs a message at Debug level
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.zapLogger.Debug(msg, fields...)
}

// Warn logs a message at Warn level
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.zapLogger.Warn(msg, fields...)
}

// Error logs a message at Error level
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.zapLogger.Error(msg, fields...)
}

// Fatal logs a message at Fatal level and then calls os.Exit(1)
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.zapLogger.Fatal(msg, fields...)
}

// WithField adds a field to the logger context
func (l *Logger) WithField(key string, value any) *Logger {
	return &Logger{
		zapLogger: l.zapLogger.With(zap.Any(key, value)),
	}
}

// WithFields adds multiple fields to the logger context
func (l *Logger) WithFields(fields map[string]any) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}

	return &Logger{
		zapLogger: l.zapLogger.With(zapFields...),
	}
}

// WithError adds an error field to the logger context
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		zapLogger: l.zapLogger.With(zap.Error(err)),
	}
}

// Helper functions to create zap fields
func WithError(err error) zap.Field {
	return zap.Error(err)
}

// WithField creates a field with any value
func WithField(key string, value any) zap.Field {
	return zap.Any(key, value)
}

// WithString creates a string field
func WithString(key string, value string) zap.Field {
	return zap.String(key, value)
}

// WithInt creates an int field
func WithInt(key string, value int) zap.Field {
	return zap.Int(key, value)
}

// WithBool creates a bool field
func WithBool(key string, value bool) zap.Field {
	return zap.Bool(key, value)
}

// InfoObj logs a message and an object at Info level
func (l *Logger) InfoObj(msg string, obj any) {
	l.zapLogger.Info(msg, zap.Any("data", obj))
}

// ErrorObj logs a message and an object at Error level
func (l *Logger) ErrorObj(msg string, obj any) {
	l.zapLogger.Error(msg, zap.Any("data", obj))
}

// Close flushes any buffered log entries
func (l *Logger) Close() error {
	return l.zapLogger.Sync()
}
