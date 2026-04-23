package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)

type Config struct {
	PicooraclawURL string
	Listen         string
	Password       string
	UpstreamToken  string
	Secret         string
}

func Load(args []string) (*Config, error) {
	fs := flag.NewFlagSet("picooraclaw-webui", flag.ContinueOnError)

	cfg := &Config{}
	fs.StringVar(&cfg.PicooraclawURL, "picooraclaw-url", getenv("PICOORACLAW_URL", "http://localhost:8090"), "upstream gateway URL")
	fs.StringVar(&cfg.Listen, "listen", getenv("PICOORACLAW_WEBUI_LISTEN", ":3000"), "listen address")
	fs.StringVar(&cfg.Password, "password", os.Getenv("PICOORACLAW_WEBUI_PASSWORD"), "optional login password")
	fs.StringVar(&cfg.UpstreamToken, "upstream-token", os.Getenv("PICOORACLAW_WEB_TOKEN"), "optional upstream bearer token")
	fs.StringVar(&cfg.Secret, "secret", os.Getenv("PICOORACLAW_WEBUI_SECRET"), "cookie signing key")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if cfg.Secret == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("generate secret: %w", err)
		}
		cfg.Secret = hex.EncodeToString(buf)
	}
	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
