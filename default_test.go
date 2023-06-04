package logger

import (
	"context"
	"testing"
)

func TestMain(t *testing.M) {
	t.Run()
}

func TestLogger_Log(t *testing.T) {
	log := NewLogger()
	log.WithContext(context.TODO())
	log.WithFields(map[string]interface{}{
		"key1": "val1",
	}).Info("info msg")
	log.Debug("debug msg")
	log.Info("info msg")
	log.Error("error msg")
	log.Warn("warn msg")
	log.Fatal("fatal msg")
}
