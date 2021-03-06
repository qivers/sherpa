package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"

	"github.com/hashicorp/nomad/api"
	"github.com/jrasell/sherpa/pkg/autoscale"
	"github.com/jrasell/sherpa/pkg/client"
	"github.com/jrasell/sherpa/pkg/policy/backend"
	"github.com/jrasell/sherpa/pkg/policy/backend/consul"
	"github.com/jrasell/sherpa/pkg/policy/backend/memory"
	"github.com/jrasell/sherpa/pkg/server/router"
	"github.com/rs/zerolog"
)

type HTTPServer struct {
	addr   string
	cfg    *Config
	logger zerolog.Logger

	nomad         *api.Client
	policyBackend backend.PolicyBackend
	autoScale     *autoscale.AutoScale

	http.Server
	routes *routes
}

func New(l zerolog.Logger, cfg *Config) *HTTPServer {
	return &HTTPServer{
		addr:   fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.Port),
		cfg:    cfg,
		logger: l,
		routes: &routes{},
	}
}

func (h *HTTPServer) Start() error {
	h.logger.Info().Str("addr", h.addr).Msg("starting HTTP server")
	h.logServerConfig()

	if err := h.setup(); err != nil {
		return err
	}

	h.handleSignals(context.Background())
	return nil
}

func (h *HTTPServer) logServerConfig() {
	h.logger.Info().
		Object("server", h.cfg.Server).
		Object("tls", h.cfg.TLS).
		Msg("Sherpa server configuration")
}

func (h *HTTPServer) setup() error {
	if err := h.setupNomadClient(); err != nil {
		return err
	}

	h.setupPolicyBackend()

	// If the server has been set to enable the internal autoscaler, set this up and start the
	// process running.
	if h.cfg.Server.InternalAutoScaler {
		h.setupAutoScaling()
	}

	initialRoutes := h.setupRoutes()

	r := router.WithRoutes(h.logger, *initialRoutes)
	http.Handle("/", middlewareLogger(r, h.logger))

	// Run the TLS setup process so that if the user has configured a TLS certificate pair the
	// server uses these.
	if err := h.setupTLS(); err != nil {
		return err
	}

	// Once we have the TLS config in place, we can setup the listener which uses the TLS setup to
	// correctly start the listener.
	ln := h.setupListener()
	if ln == nil {
		return errors.New("failed to setup HTTP server, listener is nil")
	}
	h.logger.Info().Str("addr", h.addr).Msg("HTTP server successfully listening")

	go func() {
		err := h.Serve(ln)
		h.logger.Info().Err(err).Msg("HTTP server has been shutdown")
	}()

	return nil
}

func (h *HTTPServer) setupTLS() error {
	if h.cfg.TLS.CertPath != "" && h.cfg.TLS.CertKeyPath != "" {
		h.logger.Debug().Msg("setting up server TLS")

		cert, err := tls.LoadX509KeyPair(h.cfg.TLS.CertPath, h.cfg.TLS.CertKeyPath)
		if err != nil {
			return errors.Wrap(err, "failed to load certificate cert/key pair")
		}
		h.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	}
	return nil
}

func (h *HTTPServer) setupPolicyBackend() {
	if h.cfg.Server.ConsulStorageBackend {
		h.logger.Debug().Msg("setting up Consul storage backend")
		h.policyBackend = consul.NewConsulPolicyBackend(h.logger, h.cfg.Server.ConsulStorageBackendPath)
	} else {
		h.logger.Debug().Msg("setting up in-memory storage backend")
		h.policyBackend = memory.NewJobScalingPolicies()
	}
}

func (h *HTTPServer) setupNomadClient() error {
	h.logger.Debug().Msg("setting up Nomad client")

	nc, err := client.NewNomadClient()
	if err != nil {
		return err
	}
	h.nomad = nc

	return nil
}

func (h *HTTPServer) setupAutoScaling() {
	h.logger.Debug().Msg("setting up Sherpa internal auto-scaling engine")
	autoscaleCfg := &autoscale.Config{
		StrictChecking:  h.cfg.Server.StrictPolicyChecking,
		ScalingInterval: h.cfg.Server.InternalAutoScalerEvalPeriod,
	}
	h.autoScale = autoscale.NewAutoScaleServer(h.logger, h.nomad, h.policyBackend, autoscaleCfg)
	go h.autoScale.Run()
}

func (h *HTTPServer) setupListener() net.Listener {
	var (
		err error
		ln  net.Listener
	)

	if h.TLSConfig != nil {
		ln, err = tls.Listen("tcp", h.addr, h.TLSConfig)
	} else {
		ln, err = net.Listen("tcp", h.addr)
	}

	if err != nil {
		h.logger.Error().Err(err).Msg("failed to setup server HTTP listener")
	}
	return ln
}

func (h *HTTPServer) Stop(ctx context.Context) error {
	h.logger.Info().Msg("gracefully shutting down HTTP server")
	return h.Shutdown(ctx)
}

func (h *HTTPServer) handleSignals(ctx context.Context) {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	defer h.Stop(ctx) // nolint:errcheck
	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-signalCh:
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				return
			default:
				panic(fmt.Sprintf("unsupported signal: %v", sig))
			}
		}
	}
}
