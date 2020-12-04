# zaploki

A Zap extension for loki

# example

```golang
package main

import (
	"errors"
	"time"

	"github.com/akkuman/zaploki"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// default logger
	logger := zap.NewExample()

	// loki client config
	cfg := &zaploki.LokiClientConfig{
        // the loki api url
        URL:       "http://admin:admin@loki.xxx.com/api/prom/push",
        // (optional, default: severity) the label's key to distinguish log's level, it will be added to Labels map
        LevelName: "severity",
        // (optional, default: zapcore.InfoLevel) logs beyond this level will be sent
        SendLevel: zapcore.InfoLevel,
        // the labels which will be sent to loki, contains the {levelname: level}
		Labels: map[string]string{
			"application": "test",
		},
    }
    // create a LokiCore instance
	lokiCore, err := zaploki.NewLokiCore(cfg)
	if err != nil {
		panic(err)
	} else {
        // add the lokiCore to the logger
		logger = logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewTee(core, lokiCore)
		}))
    }
    
    // test log
    logger.Info("info log")

    // test log with fields
	logger.Error("this log will be auto captured by loki", zap.String("f1", "v1"), zap.Error(errors.New("this ia an error")))

	// Because the loki client use the channel to send log in an asynchronous manner, We should wait for the log to be sent
	time.Sleep(5 * time.Second)
}
```