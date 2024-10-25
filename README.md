<div style="text-align: center;">
  <h1>
    Slog Cloud - Cloud Provider Logging for Go
  </h1>
</div>

Slog Cloud is a Go package that extends the standard `slog` library to seamlessly integrate with cloud provider logging services. Currently supporting AWS CloudWatch Logs, with plans to expand to other cloud providers in the future.

### Why Would I Use This?

- Easy integration with cloud logging services
- Built on top of Go's standard `slog` package
- Automatic log group creation
- Fallback to standard logging for development
- Simple setup and configuration

<a id="installation"></a>

## 📦 Installation

```bash
go get github.com/melkeydev/slog-cloud
```

This will install the package and its dependencies. The main dependencies include:

- `github.com/aws/aws-sdk-go-v2` and related AWS packages
- Go standard library `log/slog`

<a id="aws-configuration"></a>

## 🔐 AWS Configuration

Slog Cloud requires AWS credentials to interact with CloudWatch Logs. You can provide these through static credentials:

```go
accessKey := "YOUR_AWS_ACCESS_KEY"
secretAccessKey := "YOUR_AWS_SECRET_KEY"
region := "us-west-2"
logGroup := "your-log-group-name"
```

Required IAM Permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ],
      "Resource": [
        "arn:aws:logs:*:*:log-group:*",
        "arn:aws:logs:*:*:log-group:*:log-stream:*"
      ]
    }
  ]
}
```

## 🚀 Usage

Initialize the logger:

```go
import (
    slodcloud "github.com/melkeydev/slog-cloud"
)

func main() {
    // Initialize the logger for production environment
    logger, err := slogcloud.GetLogger(
        slogcloud.PROD,
        accessKey,
        secretAccessKey,
        logGroup,
        region,
    )
    if err != nil {
        log.Fatalf("Failed to initialize logger: %v", err)
    }

    // Example log messages
    logger.Info("Testing info level logging")
    logger.Debug("Testing debug level logging")
    logger.Warn("This is a warning message")
    logger.Error("An error occurred", fmt.Errorf("this is an error"))
}
```

Log messages will automatically be sent to CloudWatch Logs. If the specified log group doesn't exist, it will be created automatically.

Example output in CloudWatch:

```
2024/10/24 21:48:10 INFO Testing info level logging
2024/10/24 21:48:10 DEBUG Testing debug level logging
2024/10/24 21:48:10 WARN This is a warning message
2024/10/24 21:48:10 ERROR An error occurred: this is an error
```

## 💻 Development Mode

For local development, you can use the DEV mode which falls back to standard logging:

```go
logger, err := slogcloud.GetLogger(slogcloud.DEV, "", "", "", "")
if err != nil {
    log.Fatalf("Failed to initialize logger: %v", err)
}

// Logs will output to standard output
logger.Info("Local development logging")
```

This mode doesn't require any cloud credentials and logs directly to stdout, making it perfect for local development and testing.

## 🔮 Future Plans

We're planning to expand support to other cloud providers:

- Google Cloud Logging
- Azure Monitor
- Datadog

If you're interested in contributing support for additional providers, please check out our contribution guidelines.

## 📄 License

Licensed under [MIT License](./LICENSE)
