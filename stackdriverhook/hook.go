package stackdriverhook

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"

	"cloud.google.com/go/errorreporting"
	"cloud.google.com/go/logging"
	"github.com/rs/zerolog"
)

// A StackdriverLoggingWriter accepts pre-encoded JSON messages and writes
// them to Google Stackdriver Logging. It implements zerolog.LevelWriter and
// maps Zerolog levels to Stackdriver levels.
type StackdriverLoggingWriter struct {
	client      *logging.Client
	errorClient *errorreporting.Client
	logger      *logging.Logger
}

// Write always returns len(p), nil.
func (sw *StackdriverLoggingWriter) Write(p []byte) (int, error) {
	sw.logger.Log(logging.Entry{Payload: rawJSON(p), Severity: logging.Default})
	return len(p), nil
}

// WriteLevel implements zerolog.LevelWriter. It always returns len(p), nil.
func (sw *StackdriverLoggingWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	severity := logging.Default

	// More efficient than logging.ParseSeverity(level.String())
	switch level {
	case zerolog.DebugLevel:
		severity = logging.Debug
	case zerolog.InfoLevel:
		severity = logging.Info
	case zerolog.WarnLevel:
		severity = logging.Warning
	case zerolog.ErrorLevel:
		severity = logging.Error
	case zerolog.FatalLevel:
		severity = logging.Critical
	case zerolog.PanicLevel:
		severity = logging.Critical
	}

	if level < zerolog.ErrorLevel {
		sw.logger.Log(logging.Entry{Payload: rawJSON(p), Severity: severity})
	} else {
		sw.logger.Log(logging.Entry{Payload: rawJSON(p), Severity: severity})
		sw.logger.Log(logging.Entry{Payload: map[string]string{"stack trace": string(sw.getStackTrace())}, Severity: severity})
		sw.errorClient.ReportSync(context.Background(), errorreporting.Entry{
			Error: errors.New(string(rawJSON(p))),
			Stack: sw.getStackTrace(),
		})
	}

	return len(p), nil
}

func (sw *StackdriverLoggingWriter) getStackTrace() []byte {
	stackSlice := make([]byte, 2048)
	length := runtime.Stack(stackSlice, false)
	stack := string(stackSlice[0:length])
	re := regexp.MustCompile("[\r\n].*zerolog.*")
	res := re.ReplaceAllString(stack, "")
	return []byte(res)
}

// Flush log
func (sw *StackdriverLoggingWriter) Flush() error {
	return sw.logger.Flush()
}

// Close Close
func (sw *StackdriverLoggingWriter) Close() {
	sw.logger.Flush()
	sw.errorClient.Flush()
	sw.client.Close()
	sw.errorClient.Close()
}

// NewStackdriverLoggingWriter create new writer
func NewStackdriverLoggingWriter(GCPProjectID, logName string, labels map[string]string, opts ...logging.LoggerOption) (*StackdriverLoggingWriter, error) {
	client, err := logging.NewClient(context.Background(), GCPProjectID)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}
	if err := client.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	// labels comes before opts so that any CommonLabels in opts take precedence.
	opts = append([]logging.LoggerOption{logging.CommonLabels(labels)}, opts...)
	logger := client.Logger(logName, opts...)

	errorClient, err := errorreporting.NewClient(context.Background(), GCPProjectID, errorreporting.Config{
		ServiceName: GCPProjectID,
		OnError: func(err error) {
			logger.StandardLogger(logging.Error).Printf("Could not log error: %v", err)
		},
	})
	if err != nil {
		return nil, err
	}

	return &StackdriverLoggingWriter{
		logger:      logger,
		client:      client,
		errorClient: errorClient,
	}, nil
}

type rawJSON []byte

func (r rawJSON) MarshalJSON() ([]byte, error)  { return []byte(r), nil }
func (r *rawJSON) UnmarshalJSON(b []byte) error { *r = rawJSON(b); return nil }
