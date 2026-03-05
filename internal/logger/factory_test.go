package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory("debug", "json")
	factoryInstance, ok := f.(*factory)
	assert.True(t, ok)
	assert.Equal(t, zapcore.DebugLevel, factoryInstance.level)
	assert.Equal(t, "json", factoryInstance.format)
}

func TestFactory_New(t *testing.T) {
	f := NewFactory("info", "json")
	logger := f.New("test-component")
	assert.NotNil(t, logger)
}
