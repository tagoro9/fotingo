package telemetry

// EventRegistry defines stable telemetry event names and their owning domains.
var EventRegistry = map[string]string{
	EventCommandStarted:    "command",
	EventCommandCompleted:  "command",
	EventCommandError:      "command",
	EventCommandCrashed:    "command",
	EventIntegrationCall:   "integration",
	EventUpdateBannerShown: "ui",
}
