package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/genai"

	"buf.build/go/protovalidate"
	"github.com/darwishdev/mcp-client-api/api"
	"github.com/darwishdev/mcp-client-api/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// operation is a clean up function on shutting down
type operation func(ctx context.Context) error

// gracefulShutdown waits for termination syscalls and doing clean up operations after received it
func gracefulShutdown(ctx context.Context, timeout time.Duration, ops map[string]operation) <-chan struct{} {
	wait := make(chan struct{})
	go func() {
		s := make(chan os.Signal, 1)

		// add any other syscalls that you want to be notified with
		signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		<-s

		log.Info().Msg("shutting down")

		// set timeout for the ops to be done to prevent system hang
		timeoutFunc := time.AfterFunc(timeout, func() {
			log.Printf("timeout %d ms has been elapsed, force exit", timeout.Milliseconds())
			os.Exit(0)
		})

		defer timeoutFunc.Stop()

		var wg sync.WaitGroup

		// Do the operations asynchronously to save time
		for key, op := range ops {
			wg.Add(1)
			innerOp := op
			innerKey := key
			go func() {
				defer wg.Done()

				log.Printf("cleaning up: %s", innerKey)
				if err := innerOp(ctx); err != nil {
					log.Printf("%s: clean up failed: %s", innerKey, err.Error())
					return
				}

				log.Printf("%s was shutdown gracefully", innerKey)
			}()
		}

		wg.Wait()

		close(wait)
	}()

	return wait
}
func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	ctx := context.Background()

	config, err := config.LoadConfig("./config")
	log.Debug().Interface("conf", config).Msg("config is")
	validator, err := protovalidate.New()
	if err != nil {
		log.Fatal().Err(err).Msg("can't get the validator")
	}
	googleaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: config.GeminiAPIKey, Backend: genai.BackendGeminiAPI})

	if err != nil {
		log.Fatal().Err(err).Msg("error creating llm client")
	}

	// Define the System Instruction content by explicitly creating Content and Part structs
	// Corrected to use []*genai.Part based on the compiler error.
	systemInstructionContent := &genai.Content{}
	history := []*genai.Content{}
	llmConfig := &genai.GenerateContentConfig{SystemInstruction: systemInstructionContent}
	chat, err := googleaiClient.Chats.Create(ctx, "gemini-2.5-pro", llmConfig, history)

	if err != nil {
		log.Fatal().Err(err).Msg("error creating llm client")
	}
	server, err := api.NewServer(config, validator, chat)
	if err != nil {
		log.Fatal().Err(err).Msg("server initialization failed")
	}
	httpServer := server.NewGrpcHttpServer()
	go func() {
		log.Info().Str("server address", config.GRPCServerAddress).Msg("GRPC server start")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("HTTP listen and serve failed")
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	wait := gracefulShutdown(ctx, 3*time.Second, map[string]operation{
		"http-server": func(ctx context.Context) error {
			return httpServer.Shutdown(ctx)
		},
	})
	<-wait
}
