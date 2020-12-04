package zaploki

import (
	"fmt"
	"strings"
	"time"

	"github.com/afiskon/promtail-client/promtail"
	"go.uber.org/zap/zapcore"
)

var promtailLevel = map[zapcore.Level]promtail.LogLevel{
	zapcore.DebugLevel:  promtail.DEBUG,
	zapcore.InfoLevel:   promtail.INFO,
	zapcore.WarnLevel:   promtail.WARN,
	zapcore.ErrorLevel:  promtail.ERROR,
	zapcore.DPanicLevel: promtail.ERROR,
	zapcore.PanicLevel:  promtail.ERROR,
	zapcore.FatalLevel:  promtail.ERROR,
}

// LokiClientConfig loki client base configuration
type LokiClientConfig struct {
	URL                string
	LevelName          string
	SendLevel          zapcore.Level // default: zapcore.InfoLevel
	Labels             map[string]string
	BatchWait          time.Duration
	BatchEntriesNumber int
}

// setDefault set default value for LokiClientConfig
func (c *LokiClientConfig) setDefault() {
	if c.URL == "" {
		c.URL = "http://localhost:3100/api/prom/push"
	}
	if c.LevelName == "" {
		c.LevelName = "severity"
	}
	if len(c.Labels) == 0 {
		c.Labels = map[string]string{
			"source": "test",
			"job":    "job",
		}
	}
	if c.BatchWait == time.Second {
		c.BatchWait = 5 * time.Second
	}
	if c.BatchEntriesNumber == 0 {
		c.BatchEntriesNumber = 10000
	}
}

// genLabelsWithLogLevel generate available labels of loki from level and the label dict you defined
func (c *LokiClientConfig) genLabelsWithLogLevel(level string) string {
	c.Labels[c.LevelName] = level
	labelsList := []string{}
	for k, v := range c.Labels {
		labelsList = append(labelsList, fmt.Sprintf(`%s="%s"`, k, v))
	}
	labelString := fmt.Sprintf(`{%s}`, strings.Join(labelsList, ", "))
	return labelString
}

// LokiCore the zapcore of loki
type LokiCore struct {
	cfg                  *LokiClientConfig
	clients              map[zapcore.Level]promtail.Client
	zapcore.LevelEnabler                        // LevelEnabler interface
	fields               map[string]interface{} // save Fields
}

// NewLokiCore creates a new zapcore for Loki with loki client initialized
func NewLokiCore(c *LokiClientConfig) (*LokiCore, error) {
	var err error
	if c == nil {
		c = &LokiClientConfig{}
	}
	c.setDefault()
	conf := promtail.ClientConfig{
		PushURL:            c.URL,
		BatchWait:          c.BatchWait,
		BatchEntriesNumber: c.BatchEntriesNumber,
		SendLevel:          promtailLevel[c.SendLevel],
		PrintLevel:         promtail.DISABLE,
	}

	// create different promtail client instance
	clients := make(map[zapcore.Level]promtail.Client)
	for k := range promtailLevel {
		conf.Labels = c.genLabelsWithLogLevel(k.String())
		clients[k], err = promtail.NewClientJson(conf)
		if err != nil {
			return nil, fmt.Errorf("unable to init promtail client: %v", err)
		}
	}
	return &LokiCore{
		cfg:          c,
		clients:      clients,
		fields:       make(map[string]interface{}),
		LevelEnabler: c.SendLevel,
	}, nil
}

func (c *LokiCore) with(fs []zapcore.Field) *LokiCore {
	// Copy our map.
	m := make(map[string]interface{}, len(c.fields))
	for k, v := range c.fields {
		m[k] = v
	}

	// Add fields to an in-memory encoder.
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fs {
		f.AddTo(enc)
	}

	// Merge the two maps.
	for k, v := range enc.Fields {
		m[k] = v
	}

	return &LokiCore{
		cfg:          c.cfg,
		clients:      c.clients,
		fields:       m,
		LevelEnabler: c.LevelEnabler,
	}
}

// With adds structured context to the Core.
func (c *LokiCore) With(fs []zapcore.Field) zapcore.Core {
	return c.with(fs)
}

// Check determines whether the supplied Entry should be logged
func (c *LokiCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.cfg.SendLevel.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

// Write serializes the Entry and any Fields supplied at the log site and
// writes them to their destination.
func (c *LokiCore) Write(ent zapcore.Entry, fs []zapcore.Field) error {
	clone := c.with(fs)

	lvl := promtailLevel[ent.Level]
	switch lvl {
	case promtail.DEBUG:
		c.clients[ent.Level].Debugf("%s | %s", ent.Message, clone.fields)
	case promtail.INFO:
		c.clients[ent.Level].Infof("%s | %s", ent.Message, clone.fields)
	case promtail.WARN:
		c.clients[ent.Level].Warnf("%s | %s", ent.Message, clone.fields)
	case promtail.ERROR:
		c.clients[ent.Level].Errorf("%s | %s", ent.Message, clone.fields)
	default:
		return fmt.Errorf("unknown log level")
	}
	return nil
}

// Sync flushes buffered logs (if any).
func (c *LokiCore) Sync() error {
	return nil
}
