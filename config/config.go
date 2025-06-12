package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v10"
)

type AdditionalHeaders map[string]string

func (h *AdditionalHeaders) UnmarshalText(envByte []byte) error {
	envString := string(envByte)
	headers := make(map[string]string)

	headerPairs := strings.SplitN(envString, ";", 2)

	for _, header := range headerPairs {
		keyValue := strings.SplitN(header, "=", 2)

		if len(keyValue) != 2 {
			return fmt.Errorf("header key value pair must be in format k=v")
		}

		headers[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
	}

	*h = headers

	return nil
}

type Config struct {
	RailwayApiKey string   `env:"RAILWAY_API_KEY,required"`
	ProjectId     string   `env:"RAILWAY_PROJECT_ID"`
	EnvironmentId string   `env:"RAILWAY_ENVIRONMENT_ID"`
	Train         []string `env:"TRAIN" envSeparator:","`

	// Fallback for backward compatibility - can be either old ENVIRONMENT_ID or new RAILWAY_ENVIRONMENT_ID
	LegacyEnvironmentId string `env:"ENVIRONMENT_ID"`

	DiscordWebhookUrl string `env:"DISCORD_WEBHOOK_URL"`
	DiscordPrettyJson bool   `env:"DISCORD_PRETTY_JSON" envDefault:"false"`

	SlackWebhookUrl string   `env:"SLACK_WEBHOOK_URL"`
	SlackPrettyJson bool     `env:"SLACK_PRETTY_JSON" envDefault:"false"`
	SlackTags       []string `env:"SLACK_TAGS" envSeparator:","`

	LokiIngestUrl string `env:"LOKI_INGEST_URL"`

	IngestUrl         string            `env:"INGEST_URL"`
	AdditionalHeaders AdditionalHeaders `env:"ADDITIONAL_HEADERS"`

	ReportStatusEvery time.Duration `env:"REPORT_STATUS_EVERY" envDefault:"10s"`

	LogsFilterGlobal  []string `env:"LOGS_FILTER" envSeparator:","`
	LogsFilterDiscord []string `env:"LOGS_FILTER_DISCORD" envSeparator:","`
	LogsFilterSlack   []string `env:"LOGS_FILTER_SLACK" envSeparator:","`
	LogsFilterLoki    []string `env:"LOGS_FILTER_LOKI" envSeparator:","`
	LogsFilterWebhook []string `env:"LOGS_FILTER_WEBHOOK" envSeparator:","`

	// New content filter fields
	LogsContentFilterGlobal  string `env:"LOGS_CONTENT_FILTER"`
	LogsContentFilterDiscord string `env:"LOGS_CONTENT_FILTER_DISCORD"`
	LogsContentFilterSlack   string `env:"LOGS_CONTENT_FILTER_SLACK"`
	LogsContentFilterLoki    string `env:"LOGS_CONTENT_FILTER_LOKI"`
	LogsContentFilterWebhook string `env:"LOGS_CONTENT_FILTER_WEBHOOK"`
}

func GetConfig() (*Config, error) {
	config := Config{}

	if err := env.Parse(&config); err != nil {
		return nil, err
	}

	// Handle backward compatibility - use LegacyEnvironmentId if EnvironmentId is not set
	if config.EnvironmentId == "" && config.LegacyEnvironmentId != "" {
		config.EnvironmentId = config.LegacyEnvironmentId
	}

	// Validate that we have either both ProjectId and EnvironmentId or just EnvironmentId
	if config.ProjectId != "" && config.EnvironmentId == "" {
		return nil, errors.New("RAILWAY_ENVIRONMENT_ID is required when RAILWAY_PROJECT_ID is specified")
	}

	if config.ProjectId == "" && config.EnvironmentId == "" {
		return nil, errors.New("either RAILWAY_ENVIRONMENT_ID or ENVIRONMENT_ID must be specified")
	}

	// If ProjectId is specified but no Train services, we'll auto-discover them
	// If only EnvironmentId is specified, Train services must be provided (backward compatibility)
	if config.ProjectId == "" && len(config.Train) == 0 {
		return nil, errors.New("TRAIN services must be specified when using ENVIRONMENT_ID (backward compatibility mode)")
	}

	if config.DiscordWebhookUrl != "" && !strings.HasPrefix(config.DiscordWebhookUrl, "https://discord.com/api/webhooks/") {
		return nil, errors.New("invalid Discord webhook URL")
	}

	if config.SlackWebhookUrl != "" && !strings.HasPrefix(config.SlackWebhookUrl, "https://hooks.slack.com/services/") {
		return nil, errors.New("invalid Slack webhook URL")
	}

	if config.DiscordWebhookUrl == "" && config.IngestUrl == "" && config.SlackWebhookUrl == "" && config.LokiIngestUrl == "" {
		return nil, errors.New("specify either a discord webhook url or an ingest url or a slack webhook url or a loki url")
	}

	return &config, nil
}
