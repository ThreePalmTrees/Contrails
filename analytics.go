package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/posthog/posthog-go"
)

// Analytics wraps the PostHog client with fail-safe, opt-out, and device ID management.
// All public methods are safe to call even when analytics is disabled or the client
// failed to initialize — they silently no-op.
type Analytics struct {
	client    posthog.Client
	deviceID  string
	enabled   bool
	startedAt time.Time
	mu        sync.RWMutex
	logger    Logger

	// getProjects retrieves the current project list (injected to avoid circular deps)
	getProjects func() ([]Project, error)
}

// NewAnalytics creates a new Analytics instance. If the API key is empty (dev builds),
// analytics is completely disabled with zero network calls.
func NewAnalytics(apiKey string, logger Logger, getProjects func() ([]Project, error)) *Analytics {
	a := &Analytics{
		logger:      logger,
		startedAt:   time.Now(),
		getProjects: getProjects,
	}

	if apiKey == "" {
		return a
	}

	// Load or generate device ID
	deviceID, err := a.loadOrCreateDeviceID()
	if err != nil {
		// Can't identify this device — disable analytics
		return a
	}
	a.deviceID = deviceID

	// Create PostHog client
	client, err := posthog.NewWithConfig(apiKey, posthog.Config{
		Endpoint:  "https://eu.i.posthog.com",
		BatchSize: 20,
		// Retry with exponential backoff
		RetryAfter: func(attempt int) time.Duration {
			return time.Duration(100<<uint(attempt)) * time.Millisecond
		},
	})
	if err != nil {
		// PostHog init failed — run without analytics
		return a
	}

	a.client = client
	a.enabled = true

	return a
}

// Close flushes any pending events. Call during app shutdown.
func (a *Analytics) Close() {
	defer func() { recover() }() //nolint: errcheck
	if a.client != nil {
		a.client.Close()
	}
}

// SetEnabled toggles analytics on or off at runtime.
func (a *Analytics) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
}

// IsEnabled returns the current opt-out state.
func (a *Analytics) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled && a.client != nil
}

// Track sends an event to PostHog. Silently no-ops on any failure.
func (a *Analytics) Track(event string, properties map[string]interface{}) {
	defer func() { recover() }() //nolint: errcheck

	if !a.IsEnabled() {
		return
	}

	props := posthog.NewProperties()
	props.Set("source", "app")
	props.Set("app_version", Version)
	props.Set("os", runtime.GOOS)
	props.Set("arch", runtime.GOARCH)
	for k, v := range properties {
		props.Set(k, v)
	}

	_ = a.client.Enqueue(posthog.Capture{
		DistinctId: a.deviceID,
		Event:      event,
		Properties: props,
	})
}

// TrackAnonymous sends an event with an ephemeral random ID and minimal properties.
// It fires regardless of the opt-out setting since the data is fully anonymous
// (no stored identifiers, no behavioral fingerprinting properties).
// Only use this for GDPR-safe aggregate counters.
func (a *Analytics) TrackAnonymous(event string, properties map[string]interface{}) {
	defer func() { recover() }() //nolint: errcheck

	if a.client == nil {
		return
	}

	props := posthog.NewProperties()
	props.Set("source", "app")
	props.Set("app_version", Version)
	props.Set("os", runtime.GOOS)
	props.Set("arch", runtime.GOARCH)
	for k, v := range properties {
		props.Set(k, v)
	}

	// Ephemeral UUID — not stored, not reused — so this event cannot be linked
	// back to any device or person.
	_ = a.client.Enqueue(posthog.Capture{
		DistinctId: uuid.New().String(),
		Event:      event,
		Properties: props,
	})
}

// Identify sets person properties on the PostHog profile for this device.
func (a *Analytics) Identify(properties map[string]interface{}) {
	defer func() { recover() }() //nolint: errcheck

	if !a.IsEnabled() {
		return
	}

	props := posthog.NewProperties()
	for k, v := range properties {
		props.Set(k, v)
	}

	_ = a.client.Enqueue(posthog.Identify{
		DistinctId: a.deviceID,
		Properties: props,
	})
}

// TrackAppStarted sends the app_started event with version, OS, and project stats.
func (a *Analytics) TrackAppStarted() {
	defer func() { recover() }() //nolint: errcheck

	// Always send an anonymous ping (GDPR-safe: no device ID, no behavioral data)
	a.TrackAnonymous("app_started_anonymous", nil)

	if !a.IsEnabled() {
		return
	}

	projectCount := 0
	agentTypes := map[string]bool{}

	if a.getProjects != nil {
		if projects, err := a.getProjects(); err == nil {
			projectCount = len(projects)
			for _, p := range projects {
				for _, s := range p.Sources {
					agentTypes[string(s.Type)] = true
				}
			}
		}
	}

	agents := make([]string, 0, len(agentTypes))
	for t := range agentTypes {
		agents = append(agents, t)
	}

	a.Track("app_started", map[string]interface{}{
		"project_count": projectCount,
		"agent_types":   agents,
	})

	// Set person properties so PostHog profile always has latest info
	a.Identify(map[string]interface{}{
		"app_version": Version,
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
	})
}

// TrackAppClosed sends the app_closed event with session duration.
func (a *Analytics) TrackAppClosed() {
	a.Track("app_closed", map[string]interface{}{
		"uptime_seconds": int(time.Since(a.startedAt).Seconds()),
	})
}

// TrackProjectAdded sends the project_added event.
func (a *Analytics) TrackProjectAdded(agentTypes []string, projectCount int) {
	a.Track("project_added", map[string]interface{}{
		"agent_types":   agentTypes,
		"project_count": projectCount,
	})
}

// TrackProjectRemoved sends the project_removed event.
func (a *Analytics) TrackProjectRemoved(projectCount int) {
	a.Track("project_removed", map[string]interface{}{
		"project_count": projectCount,
	})
}

// TrackAgentSourceAdded sends the agent_source_added event.
func (a *Analytics) TrackAgentSourceAdded(agentType string) {
	a.Track("agent_source_added", map[string]interface{}{
		"agent": agentType,
	})
}

// TrackAgentSourceRemoved sends the agent_source_removed event.
func (a *Analytics) TrackAgentSourceRemoved(agentType string) {
	a.Track("agent_source_removed", map[string]interface{}{
		"agent": agentType,
	})
}

// TrackContrailCreated sends the contrail_created event.
func (a *Analytics) TrackContrailCreated(agentType string, messageCount int) {
	a.Track("contrail_created", map[string]interface{}{
		"agent":         agentType,
		"message_count": messageCount,
	})
}

// TrackProcessAll sends the process_all event (batch processing).
func (a *Analytics) TrackProcessAll(agentType string, count int) {
	a.Track("process_all", map[string]interface{}{
		"agent": agentType,
		"count": count,
	})
}

// --- Device ID management ---

func (a *Analytics) configDir() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(baseDir, "contrails")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func (a *Analytics) loadOrCreateDeviceID() (string, error) {
	dir, err := a.configDir()
	if err != nil {
		return "", err
	}

	idFile := filepath.Join(dir, "device_id")
	data, err := os.ReadFile(idFile)
	if err == nil {
		id := string(data)
		if len(id) >= 36 {
			return id, nil
		}
	}

	// Generate new UUID
	id := uuid.New().String()
	if err := os.WriteFile(idFile, []byte(id), 0644); err != nil {
		return "", fmt.Errorf("writing device_id: %w", err)
	}
	return id, nil
}
