package server

import (
	"net/http"

	policyV1 "github.com/jrasell/sherpa/pkg/policy/v1"
	watcher2 "github.com/jrasell/sherpa/pkg/policy/watcher"
	scaleV1 "github.com/jrasell/sherpa/pkg/scale/v1"
	"github.com/jrasell/sherpa/pkg/server/router"
	systemV1 "github.com/jrasell/sherpa/pkg/system/v1"
)

type routes struct {
	Health *systemV1.System
	Policy *policyV1.Policy
	Scale  *scaleV1.Scale
}

func (h *HTTPServer) setupRoutes() *router.RouteTable {
	h.logger.Debug().Msg("setting up HTTP server routes")

	// Setup our route servers with their required configuration.
	h.routes.Health = systemV1.NewSystemServer(h.logger, h.nomad, h.cfg.Server)
	h.routes.Scale = scaleV1.NewScaleServer(h.logger, h.cfg.Server.StrictPolicyChecking, h.policyBackend, h.nomad)
	h.routes.Policy = policyV1.NewPolicyServer(h.logger, h.policyBackend)

	healthRoutes := router.Routes{
		router.Route{
			Name:    routeSystemHealthName,
			Method:  http.MethodGet,
			Pattern: routeSystemHealthPattern,
			Handler: h.routes.Health.GetHealth,
		},
		router.Route{
			Name:    routeSystemInfoName,
			Method:  http.MethodGet,
			Pattern: routeSystemInfoPattern,
			Handler: h.routes.Health.GetInfo,
		},
	}

	scaleRoutes := router.Routes{
		router.Route{
			Name:    routeScaleOutJobGroupName,
			Method:  http.MethodPut,
			Pattern: routeScaleOutJobGroupPattern,
			Handler: h.routes.Scale.OutJobGroup,
		},
		router.Route{
			Name:    routeScaleInJobGroupName,
			Method:  http.MethodPut,
			Pattern: routeScaleInJobGroupPattern,
			Handler: h.routes.Scale.InJobGroup,
		},
	}

	policyRoutes := router.Routes{
		router.Route{
			Name:    routeGetJobScalingPoliciesName,
			Method:  http.MethodGet,
			Pattern: routeGetJobScalingPoliciesPattern,
			Handler: h.routes.Policy.GetJobPolicies,
		},
		router.Route{
			Name:    routeGetJobScalingPolicyName,
			Method:  http.MethodGet,
			Pattern: routeGetJobScalingPolicyPattern,
			Handler: h.routes.Policy.GetJobPolicy,
		},
		router.Route{
			Name:    routeGetJobGroupScalingPolicyName,
			Method:  http.MethodGet,
			Pattern: routeGetJobGroupScalingPolicyPattern,
			Handler: h.routes.Policy.GetJobGroupPolicy,
		},
	}

	if h.cfg.Server.NomadMetaPolicyEngine {
		watcher := watcher2.NewMetaWatcher(h.logger, h.nomad, h.policyBackend)
		go watcher.Run()
	}

	if h.cfg.Server.APIPolicyEngine {
		h.logger.Info().Msg("starting Sherpa API policy engine")

		apiPolicyEngineRoutes := router.Routes{
			router.Route{
				Name:    routePostJobScalingPolicyName,
				Method:  http.MethodPost,
				Pattern: routePutJobScalingPolicyPattern,
				Handler: h.routes.Policy.PutJobPolicy,
			},
			router.Route{
				Name:    routePostJobGroupScalingPolicyName,
				Method:  http.MethodPost,
				Pattern: routePutJobGroupScalingPolicyPattern,
				Handler: h.routes.Policy.PutJobGroupPolicy,
			},
			router.Route{
				Name:    routeDeleteJobGroupScalingPolicyName,
				Method:  http.MethodDelete,
				Pattern: routeDeleteJobGroupScalingPolicyPattern,
				Handler: h.routes.Policy.DeleteJobGroupPolicy,
			},
			router.Route{
				Name:    routeDeleteJobScalingPolicyName,
				Method:  http.MethodDelete,
				Pattern: routeDeleteJobScalingPolicyPattern,
				Handler: h.routes.Policy.DeleteJobPolicy,
			},
		}
		return &router.RouteTable{healthRoutes, scaleRoutes, policyRoutes, apiPolicyEngineRoutes}
	}

	return &router.RouteTable{healthRoutes, scaleRoutes, policyRoutes}
}
