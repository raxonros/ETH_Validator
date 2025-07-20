package logger

import (
    "go.uber.org/zap"
)

func Init() (*zap.Logger, error) {
    return zap.NewProduction()
}