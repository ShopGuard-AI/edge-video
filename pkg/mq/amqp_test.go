package mq

import (
"testing"
)

func TestExtractVhostFromURL(t *testing.T) {
tests := []struct {
name        string
amqpURL     string
expected    string
expectError bool
}{
{
name:        "URL with explicit vhost",
amqpURL:     "amqp://user:password@localhost:5672/myvhost",
expected:    "myvhost",
expectError: false,
},
{
name:        "URL with default vhost (no path)",
amqpURL:     "amqp://user:password@localhost:5672",
expected:    "/",
expectError: false,
},
{
name:        "URL with default vhost (empty path)",
amqpURL:     "amqp://user:password@localhost:5672/",
expected:    "/",
expectError: false,
},
{
name:        "URL with complex vhost name",
amqpURL:     "amqp://user:password@localhost:5672/supermercado_vhost",
expected:    "supermercado_vhost",
expectError: false,
},
{
name:        "URL with encoded vhost",
amqpURL:     "amqp://user:password@localhost:5672/%2Fmyvhost",
expected:    "%2Fmyvhost",
expectError: false,
},
{
name:        "Invalid URL",
amqpURL:     "not a valid url",
expected:    "",
expectError: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
vhost, err := ExtractVhostFromURL(tt.amqpURL)

if tt.expectError {
if err == nil {
t.Errorf("Expected error but got none")
}
return
}

if err != nil {
t.Errorf("Unexpected error: %v", err)
return
}

if vhost != tt.expected {
t.Errorf("Expected vhost '%s', got '%s'", tt.expected, vhost)
}
})
}
}
