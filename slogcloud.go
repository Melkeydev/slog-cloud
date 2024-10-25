package slogcloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/google/uuid"
)

// Environment constants to use
const (
	PROD = "prod"
	DEV  = "dev"
)

// Logger is the interface that defines multiple log levels.
type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string, err error)
	Fatal(msg string, err error)
}

// CloudwatchClient represents the AWS CloudWatch Logs client.
type CloudwatchClient struct {
	logStream string
	logGroup  string
	client    *cloudwatchlogs.Client
}

// SlogLogger implements the Logger interface using the slog library.
type SlogLogger struct {
	handler *CloudWatchLogHandler
}

// Debug logs a debug message.
func (s *SlogLogger) Debug(msg string) {
	slog.Debug(msg)
}

// Info logs an info message.
func (s *SlogLogger) Info(msg string) {
	slog.Info(msg)
}

// Warn logs a warning message.
func (s *SlogLogger) Warn(msg string) {
	slog.Warn(msg)
}

// Error logs an error message.
func (s *SlogLogger) Error(msg string, err error) {
	if err != nil {
		// We pass this for AWS to have a specific error key
		slog.Error(msg, slog.String("error", err.Error()))
	} else {
		slog.Error(msg)
	}
}

// Fatal logs a fatal error message and exits the program.
func (s *SlogLogger) Fatal(msg string, err error) {
	slog.Error(msg, slog.Any("fatal", err))
	os.Exit(1)
}

// StdLogger implements the Logger interface for non-production environments (console output).
type StdLogger struct{}

// Debug logs a debug message to stdout.
func (l *StdLogger) Debug(msg string) {
	fmt.Println("DEBUG:", msg)
}

// Info logs an info message to stdout.
func (l *StdLogger) Info(msg string) {
	fmt.Println("INFO:", msg)
}

// Warn logs a warning message to stdout.
func (l *StdLogger) Warn(msg string) {
	fmt.Println("WARN:", msg)
}

// Error logs an error message to stdout.
func (l *StdLogger) Error(msg string, err error) {
	fmt.Println("ERROR:", msg, err)
}

// Fatal logs a fatal error message to stdout and exits the program.
func (l *StdLogger) Fatal(msg string, err error) {
	fmt.Println("FATAL:", msg, err)
	os.Exit(1)
}

// CloudWatchLogHandler is the handler that sends logs to AWS CloudWatch.
type CloudWatchLogHandler struct {
	client *CloudwatchClient
}

// Handle processes and sends logs to CloudWatch.
func (h *CloudWatchLogHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.client.EmitLog(r)
}

// Enabled returns true to allow logging for all levels.
func (h *CloudWatchLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
}

// WithAttrs is used for setting attributes in a group.
func (h *CloudWatchLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

// WithGroup sets the group name for structured logs.
func (h *CloudWatchLogHandler) WithGroup(name string) slog.Handler {
	return h
}

// NewCloudWatchLogHandler creates a new CloudWatchLogHandler.
func NewCloudWatchLogHandler(client *CloudwatchClient) *CloudWatchLogHandler {
	return &CloudWatchLogHandler{client: client}
}

// NewCloudwatchClient initializes a CloudwatchClient with user-provided AWS credentials
// and creates a log stream. If the log group doesn't exist, it will create it.
func NewCloudwatchClient(accessKey, secretAccessKey, logGroup, region string) (*CloudwatchClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("could not load AWS config: %w", err)
	}

	cwClient := cloudwatchlogs.NewFromConfig(cfg)
	_, err = cwClient.DescribeLogGroups(context.TODO(), &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(logGroup),
	})

	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			// Create log group if not found
			_, err = cwClient.CreateLogGroup(context.TODO(), &cloudwatchlogs.CreateLogGroupInput{
				LogGroupName: aws.String(logGroup),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create log group: %w", err)
			}
		} else {
			return nil, fmt.Errorf("error describing log group: %w", err)
		}
	}

	// Add a small delay after creating the log group to ensure AWS has propagated the creation
	time.Sleep(2 * time.Second)

	logStream := fmt.Sprintf("slogcloud-stream-%s-%s",
		time.Now().Format("20060102T150405"),
		uuid.New().String(),
	)

	// Create the log stream
	_, err = cwClient.CreateLogStream(context.TODO(), &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create CloudWatch log stream: %w", err)
	}

	return &CloudwatchClient{
		client:    cwClient,
		logStream: logStream,
		logGroup:  logGroup,
	}, nil
}

//////////////////////////////
///// METHODS FOR CLIENT /////
//////////////////////////////

// EmitLog sends log records to AWS CloudWatch.
func (cw *CloudwatchClient) EmitLog(r slog.Record) error {
	message := r.Message

	logEntry := map[string]interface{}{
		"message": message,
	}

	r.Attrs(func(a slog.Attr) bool {
		val := a.Value.Any()

		if errValue, ok := val.(error); ok {
			logEntry[a.Key] = errValue.Error()
		} else {
			logEntry[a.Key] = val
		}
		return true
	})

	logEntryJson, _ := json.Marshal(logEntry)

	// Prepare the log input for CloudWatch
	input := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(cw.logGroup),
		LogStreamName: aws.String(cw.logStream),
		LogEvents: []types.InputLogEvent{
			{
				Message:   aws.String(string(logEntryJson)),
				Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
			},
		},
	}

	_, err := cw.client.PutLogEvents(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to send log to CloudWatch: %w", err)
	}

	return nil
}

func GetLogger(env, accessKey, secretAccessKey, logGroup, region string) (Logger, error) {
	if env == PROD {
		// In production, log to CloudWatch using slog
		cwClient, err := NewCloudwatchClient(accessKey, secretAccessKey, logGroup, region)
		if err != nil {
			return nil, fmt.Errorf("failed to create CloudWatch client: %w", err)
		}

		cloudWatchHandler := NewCloudWatchLogHandler(cwClient)
		slog.SetDefault(slog.New(cloudWatchHandler))

		return &SlogLogger{handler: cloudWatchHandler}, nil
	}

	// For non-production environments, log to standard output
	return &StdLogger{}, nil
}
