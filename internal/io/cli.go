package io

import (
	"fmt"
	"time"

	"github.com/kyokomi/emoji/v2"
	"github.com/spf13/cobra"
)

func PrintDone(start time.Time) {
	fmt.Println(emoji.Sprintf("\n:sparkles: Done in: %s", time.Since(start).Round(time.Millisecond)))
}

func RunCommand(command func(cmd *cobra.Command, args []string)) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		start := time.Now()
		command(cmd, args)
		PrintDone(start)
	}
}

// MessageType defines the type of message
type MessageType string

const (
	MessageTypeError   MessageType = "error"
	MessageTypeInfo    MessageType = "info"
	MessageTypeRequest MessageType = "request"
	MessageTypeStatus  MessageType = "status"
)

// Message represents a message in the application
type Message struct {
	Detail  string // Optional detailed information
	Emoji   string // Optional emoji to display
	Message string // The main message content
	// ShowInRawMode bool        // Whether to show in raw mode
	// ShowSpinner bool        // Whether to show a spinner with the message
	Type MessageType // The type of message
}
