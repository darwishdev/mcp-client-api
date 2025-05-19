package api

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	mcpclientv1 "github.com/darwishdev/mcp-client-api/proto_gen/mcpclient/v1"
	"github.com/rs/zerolog/log"
	"google.golang.org/genai"
)

func (api *Api) SendMessage(
	ctx context.Context,
	req *connect.Request[mcpclientv1.SendMessageRequest],
	st *connect.ServerStream[mcpclientv1.SendMessageResponse],
) error {
	if err := ctx.Err(); err != nil {
		log.Error().Err(err).Msg("Context cancelled before processing request")
		return err
	}

	if api.chat == nil {
		log.Error().Msg("Chat object is not initialized in Api struct")
		return connect.NewError(connect.CodeInternal, errors.New("server configuration error: chat not initialized"))
	}

	// Send the user's message to the GenAI model and get a streaming response
	// stream := api.chat.SendMessageStream(ctx, genai.Part{Text: req.Msg.Content})
	systemInstructionContent := &genai.Content{
		Parts: []*genai.Part{ // This should be []*genai.Part (slice of pointers to Part structs)
			&genai.Part{ // Each element is a pointer to a genai.Part struct
				Text: req.Msg.Instructions,
			},
		},
		// Role is typically not set for the SystemInstruction Content
	}
	result, _ := api.chat.Models.GenerateContent(
		ctx,
		"gemini-2.0-flash",
		genai.Text(req.Msg.Content),
		&genai.GenerateContentConfig{SystemInstruction: systemInstructionContent},
	)

	// Check if the result has candidates
	if len(result.Candidates) == 0 {
		log.Error().Msg("No candidates returned from GenAI")
		return connect.NewError(connect.CodeUnknown, errors.New("no response from GenAI"))
	}

	candidate := result.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		log.Error().Msg("No content parts in the candidate")
		return connect.NewError(connect.CodeUnknown, errors.New("empty response from GenAI"))
	}

	// Send the response text back to the client
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			err := st.Send(&mcpclientv1.SendMessageResponse{
				Content: part.Text,
			})
			if err != nil {
				log.Error().Err(err).Msg("Failed to send message to client")
				return connect.NewError(connect.CodeUnknown, err)
			}
		}
	}
	log.Debug().Interface("res", result).Msg("hello")
	// 	// Process and print the streaming response chunks
	// 	for chunk := range stream {
	// 		// Check if Candidates and Content slices are not empty before accessing
	// 		if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
	// 			part := chunk.Candidates[0].Content.Parts[0]
	// 			fmt.Print(part.Text) // Print the model's response chunk by chunk
	// 		}
	// 	}
	// 	// Send the user's message to the GenAI model and get a streaming response
	// 	// iter := api.chat.SendMessageStream(ctx, genai.Part{Text: req.Msg.Content})

	// 	// 	for {
	// 	// 		chunk, err := iter.Next()
	// 	// 		if err != nil {
	// 	// 			if errors.Is(err, io.EOF) {
	// 	// 				break
	// 	// 			}
	// 	// 			log.Error().Err(err).Msg("Error receiving chunk from GenAI stream")
	// 	// 			return connect.NewError(connect.CodeUnknown, err)
	// 	// 		}

	// 	// 		if chunk == nil {
	// 	// 			continue
	// 	// 		}

	// 	// 		if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
	// 	// 			for _, part := range chunk.Candidates[0].Content.Parts {
	// 	// 				// Send the chunk as a gRPC stream message to the client
	// 	// 				err := st.Send(&mcpclientv1.SendMessageResponse{
	// 	// 					Content: part.Text,
	// 	// 				})
	// 	// 				if err != nil {
	// 	// 					log.Error().Err(err).Msg("Failed to send message to client")
	// 	// 					return connect.NewError(connect.CodeUnknown, err)
	// 	// 				}
	// 	// 			}
	// 	// 		}
	// 	// 	}
	return nil
}
