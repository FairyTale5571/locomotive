package railway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/ferretcode/locomotive/config"
	"github.com/ferretcode/locomotive/logger"
	"github.com/ferretcode/locomotive/util"
	"github.com/google/uuid"
)

func (g *GraphQLClient) buildMetadataMap(ctx context.Context, cfg *config.Config) (map[string]string, error) {
	if g.client == nil {
		return nil, errors.New("client is nil")
	}

	var projectId string

	// If we have ProjectId directly, use it. Otherwise, get it from EnvironmentId
	if cfg.ProjectId != "" {
		projectId = cfg.ProjectId
	} else {
		environment := &Environment{}

		variables := map[string]any{
			"id": cfg.EnvironmentId,
		}

		if err := g.client.Exec(ctx, environmentQuery, &environment, variables); err != nil {
			return nil, err
		}

		projectId = environment.Environment.ProjectID
	}

	project := &Project{}

	variables := map[string]any{
		"id": projectId,
	}

	if err := g.client.Exec(ctx, projectQuery, &project, variables); err != nil {
		return nil, err
	}

	idNameMap := make(map[string]string)

	for _, e := range project.Project.Environments.Edges {
		idNameMap[e.Node.ID] = e.Node.Name
	}

	for _, s := range project.Project.Services.Edges {
		idNameMap[s.Node.ID] = s.Node.Name
	}

	idNameMap[project.Project.ID] = project.Project.Name

	return idNameMap, nil
}

type operationMessage struct {
	Id      string  `json:"id"`
	Type    string  `json:"type"`
	Payload payload `json:"payload"`
}

type payload struct {
	Query     string     `json:"query"`
	Variables *variables `json:"variables"`
}

type variables struct {
	EnvironmentId string `json:"environmentId"`
	Filter        string `json:"filter"`
	BeforeLimit   int64  `json:"beforeLimit"`
	BeforeDate    string `json:"beforeDate"`
}

var (
	connectionInit = []byte(`{"type":"connection_init"}`)
	connectionAck  = []byte(`{"type":"connection_ack"}`)
)

func (g *GraphQLClient) createSubscription(ctx context.Context, cfg *config.Config) (*websocket.Conn, error) {
	var services []string

	// If Train services are specified, use them. Otherwise, auto-discover services.
	if len(cfg.Train) > 0 {
		services = cfg.Train
	} else if cfg.ProjectId != "" {
		// Auto-discover all services in the environment
		autoServices, err := g.GetAllServicesInEnvironment(ctx, cfg.ProjectId, cfg.EnvironmentId)
		if err != nil {
			return nil, fmt.Errorf("error auto-discovering services: %w", err)
		}
		services = autoServices
		logger.Stdout.Info("auto-discovered services", slog.Any("services", services), slog.Int("count", len(services)))
	} else {
		return nil, errors.New("either TRAIN services must be specified or RAILWAY_PROJECT_ID must be provided for auto-discovery")
	}

	payload := &payload{
		Query: streamEnvironmentLogsQuery,
		Variables: &variables{
			EnvironmentId: cfg.EnvironmentId,
			Filter:        buildServiceFilter(services),

			// needed for seamless subscription resuming
			BeforeDate:  time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339Nano),
			BeforeLimit: 500,
		},
	}

	subPayload := operationMessage{
		Id:      uuid.Must(uuid.NewUUID()).String(),
		Type:    "subscribe",
		Payload: *payload,
	}

	payloadBytes, err := json.Marshal(&subPayload)
	if err != nil {
		return nil, err
	}

	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + g.AuthToken},
			"Content-Type":  []string{"application/json"},
		},
		Subprotocols: []string{"graphql-transport-ws"},
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, (10 * time.Second))
	defer cancel()

	c, _, err := websocket.Dial(ctxTimeout, g.BaseSubscriptionURL, opts)
	if err != nil {
		return nil, err
	}

	c.SetReadLimit(-1)

	if err := c.Write(ctx, websocket.MessageText, connectionInit); err != nil {
		return nil, err
	}

	_, ackMessage, err := c.Read(ctx)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(ackMessage, connectionAck) {
		return nil, errors.New("did not receive connection ack from server")
	}

	if err := c.Write(ctx, websocket.MessageText, payloadBytes); err != nil {
		return nil, err
	}

	return c, nil
}

func (g *GraphQLClient) SubscribeToLogs(ctx context.Context, logTrack chan<- []EnvironmentLog, cfg *config.Config) error {
	metadataMap, err := g.buildMetadataMap(ctx, cfg)
	if err != nil {
		return fmt.Errorf("error building metadata map: %w", err)
	}

	conn, err := g.createSubscription(ctx, cfg)
	if err != nil {
		return err
	}

	defer conn.CloseNow()

	LogTime := time.Now().UTC()

	for {
		_, logPayload, err := safeConnRead(conn, ctx)
		if err != nil {
			logger.Stdout.Debug("resubscribing", slog.Any("reason", err))

			safeConnCloseNow(conn)

			conn, err = g.createSubscription(ctx, cfg)
			if err != nil {
				return err
			}

			continue
		}

		logs := &LogPayloadResponse{}

		if err := json.Unmarshal(logPayload, &logs); err != nil {
			return err
		}

		if logs.Type != TypeNext {
			logger.Stdout.Debug("resubscribing", slog.String("reason", fmt.Sprintf("log type not next: %s", logs.Type)))

			safeConnCloseNow(conn)

			conn, err = g.createSubscription(ctx, cfg)
			if err != nil {
				return err
			}

			continue
		}

		filteredLogs := []EnvironmentLog{}

		for i := range logs.Payload.Data.EnvironmentLogs {
			// skip logs with empty messages and no attributes
			// we check for 1 attribute because empty logs will always have at least one attribute, the level
			if logs.Payload.Data.EnvironmentLogs[i].Message == "" && len(logs.Payload.Data.EnvironmentLogs[i].Attributes) == 1 {
				continue
			}

			// skip container logs, container logs don't have deployment instance ids
			if logs.Payload.Data.EnvironmentLogs[i].Tags.DeploymentInstanceID == "" {
				logger.Stdout.Debug("skipping container log message")
				continue
			}

			// on first subscription skip logs if they where logged before the first subscription, on resubscription skip logs if they where already processed
			if logs.Payload.Data.EnvironmentLogs[i].Timestamp.Before(LogTime) || LogTime == logs.Payload.Data.EnvironmentLogs[i].Timestamp {
				// logger.Stdout.Debug("skipping stale log message")
				continue
			}

			// skip logs that don't match our desired global filter(s)
			if !util.IsWantedLevel(cfg.LogsFilterGlobal, logs.Payload.Data.EnvironmentLogs[i].Severity) {
				logger.Stdout.Debug("skipping undesired global log level", slog.String("level", logs.Payload.Data.EnvironmentLogs[i].Severity), slog.Any("wanted", cfg.LogsFilterGlobal))
				continue
			}

			// skip logs that don't match our desired global content filter(s)
			if !util.MatchesContentFilter(cfg.LogsContentFilterGlobal, logs.Payload.Data.EnvironmentLogs[i].Message) {
				logger.Stdout.Debug("skipping undesired global log content", slog.String("content", logs.Payload.Data.EnvironmentLogs[i].Message), slog.String("filter", cfg.LogsContentFilterGlobal))
				continue
			}

			LogTime = logs.Payload.Data.EnvironmentLogs[i].Timestamp

			serviceName, ok := metadataMap[logs.Payload.Data.EnvironmentLogs[i].Tags.ServiceID]
			if !ok {
				logger.Stdout.Warn("service name could not be found")
				serviceName = "undefined"
			}

			logs.Payload.Data.EnvironmentLogs[i].Tags.ServiceName = serviceName

			environmentName, ok := metadataMap[logs.Payload.Data.EnvironmentLogs[i].Tags.EnvironmentID]
			if !ok {
				logger.Stdout.Warn("environment name could not be found")
				environmentName = "undefined"
			}

			logs.Payload.Data.EnvironmentLogs[i].Tags.EnvironmentName = environmentName

			projectName, ok := metadataMap[logs.Payload.Data.EnvironmentLogs[i].Tags.ProjectID]
			if !ok {
				logger.Stdout.Warn("project name could not be found")
				projectName = "undefined"
			}

			logs.Payload.Data.EnvironmentLogs[i].Tags.ProjectName = projectName

			filteredLogs = append(filteredLogs, logs.Payload.Data.EnvironmentLogs[i])
		}

		if len(filteredLogs) == 0 {
			continue
		}

		logTrack <- filteredLogs
	}
}

// helper function to build a service filter string from provided service ids
func buildServiceFilter(serviceIds []string) string {
	var filterString string

	for i, serviceId := range serviceIds {
		filterString += "@service:" + serviceId
		if i < len(serviceIds)-1 {
			filterString += " OR "
		}
	}

	return filterString
}

// Railway tends to close the connection abruptly, this is needed to prevent any panics caused by reading from an abruptly closed connection
func safeConnRead(conn *websocket.Conn, ctx context.Context) (mT websocket.MessageType, b []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from panic: %v", r)
		}
	}()

	return conn.Read(ctx)
}

func safeConnCloseNow(conn *websocket.Conn) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from panic: %v", r)
		}
	}()

	return conn.CloseNow()
}
