package main

import (
	"reflect"

	"github.com/pkg/errors"
)

// configuration captures the plugin's external configuration as exposed in the Mattermost server
// configuration, as well as values computed from the configuration. Any public fields will be
// deserialized from the Mattermost server configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// configuration can change at any time, access to the configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
//
// If you add non-reference types to your configuration struct, be sure to rewrite Clone as a deep
// copy appropriate for your types.

const (
	EndpointTypeArc      = "arc"
	EndpointTypeWorkflow = "workflow"
)

type CustomEndpoint struct {
	Name     string
	Endpoint string
	Type     string // Added Type field: "arc" or "workflow"
}

type configuration struct {
	MaestroURL                 string // Renamed from GraphQLAgentWebSocketURL
	CustomEndpoints            []CustomEndpoint
	GraphQLPingIntervalSeconds *int `json:"GraphQLPingIntervalSeconds"`
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
// Note: CustomEndpoints is a slice of structs. If CustomEndpoint contained pointers or slices,
// a deep copy for it would be needed. Since Name, Endpoint, and Type are strings, a shallow copy is fine.
func (c *configuration) Clone() *configuration {
	var clone = *c
	// Deep copy CustomEndpoints to ensure modifications to one don't affect the other.
	if c.CustomEndpoints != nil {
		clone.CustomEndpoints = make([]CustomEndpoint, len(c.CustomEndpoints))
		copy(clone.CustomEndpoints, c.CustomEndpoints)
	}
	return &clone
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		// Ensure a default configuration with potentially default endpoint types
		// For now, returning an empty config, OnConfigurationChange will handle defaults.
		return &configuration{}
	}
	// It's important that the returned configuration is treated as immutable.
	// A clone is returned to prevent modification of the original.
	// However, the existing getConfiguration already returns p.configuration which is a pointer.
	// The convention is that the caller does not modify it.
	return p.configuration
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	var configuration = new(configuration)

	// Load the public configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	// Provide default for GraphQLPingIntervalSeconds if not set or invalid
	defaultPingInterval := 30 // Default to 30 seconds
	if configuration.GraphQLPingIntervalSeconds == nil || *configuration.GraphQLPingIntervalSeconds <= 0 {
		configuration.GraphQLPingIntervalSeconds = &defaultPingInterval
		p.API.LogInfo("GraphQLPingIntervalSeconds not configured or invalid, defaulting to 30 seconds.")
	}

	// Set default type for CustomEndpoints if not specified
	if configuration.CustomEndpoints != nil {
		for i := range configuration.CustomEndpoints {
			if configuration.CustomEndpoints[i].Type == "" {
				configuration.CustomEndpoints[i].Type = EndpointTypeArc // Default to "arc"
				p.API.LogInfo("CustomEndpoint type not set, defaulting to 'arc'", "endpoint_name", configuration.CustomEndpoints[i].Name)
			}
			// Optional: Validate the type if it is set
			// if configuration.CustomEndpoints[i].Type != EndpointTypeArc && configuration.CustomEndpoints[i].Type != EndpointTypeWorkflow {
			//  p.API.LogWarn("Invalid CustomEndpoint type", "endpoint_name", configuration.CustomEndpoints[i].Name, "type", configuration.CustomEndpoints[i].Type)
			//  // Optionally, force a default type or return an error
			//  // configuration.CustomEndpoints[i].Type = EndpointTypeArc
			// }
		}
	}

	p.setConfiguration(configuration.Clone()) // Ensure to store a clone

	return nil
}
