package api

import (
	// USECASE_IMPORTS

	"buf.build/go/protovalidate"
	"github.com/darwishdev/mcp-client-api/config"
	"github.com/darwishdev/mcp-client-api/proto_gen/mcpclient/v1/mcpclientv1connect"
	"google.golang.org/genai"
)

type Api struct {
	mcpclientv1connect.UnimplementedMcpClientServiceHandler
	config    config.Config
	chat      *genai.Chat
	validator protovalidate.Validator
}

func NewApi(config config.Config, validator protovalidate.Validator, chat *genai.Chat) (mcpclientv1connect.McpClientServiceHandler, error) {
	return &Api{
		config:    config,
		chat:      chat,
		validator: validator,
	}, nil
}
