package api

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"time"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"github.com/darwishdev/mcp-client-api/config"
	"github.com/darwishdev/mcp-client-api/proto_gen/mcpclient/v1/mcpclientv1connect"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/genai"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Server struct {
	config    config.Config
	validator protovalidate.Validator
	api       mcpclientv1connect.McpClientServiceHandler
	chat      *genai.Chat
}

func NewServer(config config.Config, validator protovalidate.Validator, chat *genai.Chat) (*Server, error) {
	api, err := NewApi(config, validator, chat)

	if err != nil {
		return nil, err
	}
	return &Server{
		config:    config,
		validator: validator,
		chat:      chat,
		api:       api,
	}, nil
}

const maxMessageSize = 10 * 1024 * 1024 // 10MB
func (s *Server) NewLoggerInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			startTime := time.Now()
			result, err := next(ctx, req)
			duration := time.Since(startTime)
			zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
			logger := log.Info()
			if err != nil {
				logger = log.Error().Err(err)
			}

			logger.
				Str("Procedure", req.Spec().Procedure).
				Interface("request", req.Any()).
				Interface("response", result).
				Dur("duration", duration).
				Msg("received a gRPC request")

			return result, err
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
func (s *Server) NewValidateInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			message, ok := req.Any().(protoreflect.ProtoMessage)
			if !ok {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("request is not a ProtoMessage"))
			}
			err := s.validator.Validate(message)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
func (s Server) NewGrpcHttpServer() *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", http.RedirectHandler("https://darwishdev.com", http.StatusFound))
	// here we can find examples of diffrent compression method 	https://connectrpc.com/docs/go/serialization-and-compression/#compression
	compress1KB := connect.WithCompressMinBytes(1024)
	interceptors := connect.WithInterceptors(s.NewValidateInterceptor(), s.NewLoggerInterceptor())

	mux.Handle(mcpclientv1connect.NewMcpClientServiceHandler(
		s.api,
		interceptors,
		compress1KB,
		connect.WithReadMaxBytes(maxMessageSize),
		connect.WithSendMaxBytes(maxMessageSize),
	))

	mux.Handle(grpchealth.NewHandler(
		grpchealth.NewStaticChecker(mcpclientv1connect.McpClientServiceName),
		compress1KB,
	))
	mux.Handle(grpcreflect.NewHandlerV1(
		grpcreflect.NewStaticReflector(mcpclientv1connect.McpClientServiceName),
		compress1KB,
	))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(
		grpcreflect.NewStaticReflector(mcpclientv1connect.McpClientServiceName),
		compress1KB,
	))
	cors := cors.New(cors.Options{
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowOriginFunc: func(origin string) bool {
			// Allow all origins, which effectively disables CORS.
			return true
		},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{
			// Content-Type is in the default safelist.
			"Accept",
			"Accept-Encoding",
			"Accept-Post",
			"Connect-Accept-Encoding",
			"Connect-Content-Encoding",
			"Content-Encoding",
			"Grpc-Accept-Encoding",
			"Grpc-Encoding",
			"Grpc-Message",
			"Grpc-Status",
			"Grpc-Status-Details-Bin",
		},
		// Let browsers cache CORS information for longer, which reduces the number
		// of preflight requests. Any changes to ExposedHeaders won't take effect
		// until the cached data expires. FF caps this value at 24h, and modern
		// Chrome caps it at 2h.
		MaxAge: int(2 * time.Hour / time.Second),
	})
	server := &http.Server{
		Addr:    s.config.GRPCServerAddress,
		Handler: h2c.NewHandler(cors.Handler(mux), &http2.Server{}),
	}
	return server

}
