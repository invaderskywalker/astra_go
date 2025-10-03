package logging

import (
	"context"
	"os"
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

// ensureLogsDir makes sure the ./logs folder exists
func ensureLogsDir() {
	if err := os.MkdirAll("./logs", os.ModePerm); err != nil {
		panic("Failed to create logs directory: " + err.Error())
	}
}

func InitLogger() {
	ensureLogsDir()
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// app.log (general logs)
	appCore := zapcore.NewCore(encoder,
		zapcore.AddSync(&lumberjack.Logger{
			Filename: "./logs/app.log", MaxSize: 100, MaxAge: 28, Compress: true,
		}),
		zap.InfoLevel,
	)
	AppLogger = zap.New(appCore)

	// request.log
	requestCore := zapcore.NewCore(encoder,
		zapcore.AddSync(&lumberjack.Logger{
			Filename: "./logs/request.log", MaxSize: 50, MaxAge: 7, Compress: true,
		}),
		zap.InfoLevel,
	)
	RequestLogger = zap.New(requestCore)

	// timer.log
	timerCore := zapcore.NewCore(encoder,
		zapcore.AddSync(&lumberjack.Logger{
			Filename: "./logs/timer.log", MaxSize: 50, MaxAge: 7, Compress: true,
		}),
		zap.InfoLevel,
	)
	TimerLogger = zap.New(timerCore)

	// error.log
	errorCore := zapcore.NewCore(encoder,
		zapcore.AddSync(&lumberjack.Logger{
			Filename: "./logs/error.log", MaxSize: 100, MaxAge: 30, Compress: true,
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
