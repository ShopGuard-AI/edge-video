package logger

import (
	"go.uber.org/zap"
)

var (
	Log *zap.SugaredLogger
)

func InitLogger(development bool) error {
	var logger *zap.Logger
	var err error
	
	if development {
		config := zap.NewDevelopmentConfig()
		config.Sampling = &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		}
		logger, err = config.Build()
	} else {
		config := zap.NewProductionConfig()
		config.Sampling = &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		}
		logger, err = config.Build()
	}
	
	if err != nil {
		return err
	}
	
	Log = logger.Sugar()
	return nil
}

func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}

func WithFields(fields map[string]interface{}) *zap.SugaredLogger {
	if Log == nil {
		return nil
	}
	
	keyValuePairs := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		keyValuePairs = append(keyValuePairs, k, v)
	}
	
	return Log.With(keyValuePairs...)
}
