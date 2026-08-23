package logging

import (
	"astra/astra/config"
	"context"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	AppLogger     *zap.Logger
	RequestLogger *zap.Logger
	TimerLogger   *zap.Logger
	ErrorLogger   *zap.Logger
)

// ensureLogsDir makes sure Astra's private log folder exists. Logs are
// runtime telemetry, not project artifacts, so they must never be created in
// the connected repository's ./logs directory.
func ensureLogsDir() string {
	logRoot := os.Getenv("ASTRA_LOG_DIR")
	if logRoot == "" {
		logRoot = filepath.Join(config.LoadConfig().AstraRoot, "logs")
	}
	if err := os.MkdirAll(logRoot, 0700); err != nil {
		// Logging must never prevent the agent from starting. This fallback is
		// still outside the connected repository and is useful in restricted
		// sandboxes or when the user's home directory is temporarily read-only.
		fallback := filepath.Join(os.TempDir(), "astra-logs")
		if fallbackErr := os.MkdirAll(fallback, 0700); fallbackErr == nil {
			logRoot = fallback
		} else {
			// The standard temporary directory is expected to be writable on the
			// supported platforms; if it is not, keep the logger pointed at a
			// harmless sink rather than panic during CLI startup.
			return os.TempDir()
		}
	}
	return logRoot
}

func InitLogger() {
	logRoot := ensureLogsDir()
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// app.log (general logs)
	appCore := zapcore.NewCore(encoder,
		zapcore.AddSync(&lumberjack.Logger{
			Filename: filepath.Join(logRoot, "app.log"), MaxSize: 100, MaxAge: 28, Compress: true,
		}),
		zap.InfoLevel,
	)
	AppLogger = zap.New(appCore)

	// request.log
	requestCore := zapcore.NewCore(encoder,
		zapcore.AddSync(&lumberjack.Logger{
			Filename: filepath.Join(logRoot, "request.log"), MaxSize: 50, MaxAge: 7, Compress: true,
		}),
		zap.InfoLevel,
	)
	RequestLogger = zap.New(requestCore)

	// timer.log
	timerCore := zapcore.NewCore(encoder,
		zapcore.AddSync(&lumberjack.Logger{
			Filename: filepath.Join(logRoot, "timer.log"), MaxSize: 50, MaxAge: 7, Compress: true,
		}),
		zap.InfoLevel,
	)
	TimerLogger = zap.New(timerCore)

	// error.log
	errorCore := zapcore.NewCore(encoder,
		zapcore.AddSync(&lumberjack.Logger{
			Filename: filepath.Join(logRoot, "error.log"), MaxSize: 100, MaxAge: 30, Compress: true,
		}),
		zap.ErrorLevel,
	)
	ErrorLogger = zap.New(errorCore)
}

// LogDuration lets you do: defer logging.LogDuration(ctx, "FuncName")()
func LogDuration(ctx context.Context, name string) func() {
	start := time.Now()

	// (Optional) extract trace_id from ctx
	traceID, _ := ctx.Value("trace_id").(string)

	return func() {
		duration := time.Since(start).Milliseconds()
		fields := []zap.Field{
			zap.String("func", name),
			zap.Int64("duration_ms", duration),
		}
		if traceID != "" {
			fields = append(fields, zap.String("trace_id", traceID))
		}

		// write ONLY to timer.log
		TimerLogger.Info("Function timed", fields...)
	}
}
