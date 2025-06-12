package railway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ferretcode/locomotive/util"
)

// searches for the given key and returns the corresponding value (and true) if found, or an empty string (and false)
func AttributesHasKeys(attributes []Attributes, keys []string) (string, bool) {
	for i := range attributes {
		for j := range keys {
			if keys[j] == attributes[i].Key {
				return attributes[i].Value, true
			}
		}
	}

	return "", false
}

func FilterLogs(logs []EnvironmentLog, wantedLevel []string, contentFilter string) []EnvironmentLog {
	if len(wantedLevel) == 0 && contentFilter == "" {
		return logs
	}

	filteredLogs := []EnvironmentLog{}

	for i := range logs {
		if !util.IsWantedLevel(wantedLevel, logs[i].Severity) {
			continue
		}

		// Convert log to JSON string for content filtering
		logJSON, _ := json.Marshal(logs[i])
		if !util.MatchesContentFilter(contentFilter, string(logJSON)) {
			continue
		}

		filteredLogs = append(filteredLogs, logs[i])
	}

	return filteredLogs
}

// GetAllServicesInEnvironment returns all service IDs for a given project and environment
func (g *GraphQLClient) GetAllServicesInEnvironment(ctx context.Context, projectId, environmentId string) ([]string, error) {
	if g.client == nil {
		return nil, errors.New("client is nil")
	}

	project := &Project{}

	variables := map[string]any{
		"id": projectId,
	}

	if err := g.client.Exec(ctx, projectQuery, &project, variables); err != nil {
		return nil, fmt.Errorf("error fetching project: %w", err)
	}

	var serviceIds []string

	// Find all services that have instances in the specified environment
	for _, service := range project.Project.Services.Edges {
		for _, instance := range service.Node.ServiceInstances.Edges {
			if instance.Node.EnvironmentID == environmentId {
				serviceIds = append(serviceIds, service.Node.ID)
				break // Only add the service ID once, even if it has multiple instances
			}
		}
	}

	return serviceIds, nil
}
