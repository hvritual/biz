package application

import (
	"context"
	"sort"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
)

type BusinessEventRouter struct {
	deps     ports.RoutingDependencies
	types    *domain.MessageTypeCatalog
	channels *domain.ChannelRegistry
	lease    time.Duration
}

func NewBusinessEventRouter(deps ports.RoutingDependencies, types *domain.MessageTypeCatalog, channels *domain.ChannelRegistry, lease time.Duration) (*BusinessEventRouter, error) {
	if deps.Queue == nil || deps.Configurations == nil || deps.Recipients == nil || deps.Groups == nil || deps.Preferences == nil || lease < 5*time.Second || lease > 5*time.Minute {
		return nil, domain.ErrRoutingUnavailable
	}
	if _, err := types.List(domain.MessageTypeQuery{Page: 1, PageSize: 1}); err != nil { return nil, err }
	if _, err := channels.List(); err != nil { return nil, err }
	return &BusinessEventRouter{deps: deps, types: types, channels: channels, lease: lease}, nil
}

func (router *BusinessEventRouter) RouteOnce(ctx context.Context, workerID string) (domain.RoutingResult, error) {
	claim, err := router.deps.Queue.ClaimNextBusinessEvent(ctx, workerID, router.lease)
	if err != nil { return domain.RoutingResult{}, err }
	if claim.Event.EventID == "" { return domain.RoutingResult{}, nil }
	event := claim.Event
	registered, err := router.types.Lookup(event.TypeCode)
	if err != nil || registered.Level != event.Level {
		return router.deps.Queue.CompleteBusinessEventRoute(ctx, claim, domain.RoutePlan{Decisions: []domain.RouteDecision{{Outcome: domain.RouteOutcomeTypeUnavailable}}})
	}
	groupActive, err := router.deps.Groups.NotificationRouteGroupActive(ctx, event.TenantID, event.GroupID)
	if err != nil { return domain.RoutingResult{}, err }
	if !groupActive {
		return router.deps.Queue.CompleteBusinessEventRoute(ctx, claim, domain.RoutePlan{Decisions: []domain.RouteDecision{{Outcome: domain.RouteOutcomeGroupUnavailable}}})
	}
	config, found, err := router.deps.Configurations.FindRoutingConfiguration(ctx, event.TenantID, event.GroupID, event.Level)
	if err != nil { return domain.RoutingResult{}, err }
	if !found {
		return router.deps.Queue.CompleteBusinessEventRoute(ctx, claim, domain.RoutePlan{Decisions: []domain.RouteDecision{{Outcome: domain.RouteOutcomeNoConfiguration}}})
	}
	ids := config.Values().RecipientIDs()
	sort.Strings(ids)
	recipients, err := router.deps.Recipients.ResolveNotificationRouteRecipients(ctx, event.TenantID, ids)
	if err != nil { return domain.RoutingResult{}, err }
	active := map[string]bool{}
	for _, r := range recipients { active[r.UserID] = r.Active }
	decisions := make([]domain.RouteDecision, 0, len(ids)*len(config.Channels))
	for _, userID := range ids {
		if !active[userID] {
			for _, code := range config.Channels {
				decisions = append(decisions, domain.RouteDecision{ConfigurationID: config.ID, ConfigurationVersion: config.Version, UserID: userID, Channel: code, Outcome: domain.RouteOutcomeRecipientInactive})
			}
			continue
		}
		for _, code := range config.Channels {
			channel, err := router.channels.Lookup(code)
			if err != nil || channel.Availability != domain.ChannelConfigurable {
				decisions = append(decisions, domain.RouteDecision{ConfigurationID: config.ID, ConfigurationVersion: config.Version, UserID: userID, Channel: code, Outcome: domain.RouteOutcomeChannelUnavailable})
				continue
			}
			if code == "in_app" {
				decisions = append(decisions, domain.RouteDecision{ConfigurationID: config.ID, ConfigurationVersion: config.Version, UserID: userID, Channel: code, Outcome: domain.RouteOutcomeInAppCreated})
				continue
			}
			preferenceChannel, ok := optionalPreferenceChannel(code)
			if !ok {
				decisions = append(decisions, domain.RouteDecision{ConfigurationID: config.ID, ConfigurationVersion: config.Version, UserID: userID, Channel: code, Outcome: domain.RouteOutcomeChannelUnavailable})
				continue
			}
			preference, err := router.deps.Preferences.ReadNotificationPreference(ctx, accessdomain.NotificationPreferenceOwner{TenantID: event.TenantID, UserID: userID}, preferenceChannel)
			if err != nil { return domain.RoutingResult{}, err }
			allowed, err := preference.OptionalAllowed()
			if err != nil { return domain.RoutingResult{}, err }
			outcome := domain.RouteOutcomeExternalTask
			if !allowed { outcome = domain.RouteOutcomePreferenceDenied }
			decisions = append(decisions, domain.RouteDecision{ConfigurationID: config.ID, ConfigurationVersion: config.Version, UserID: userID, Channel: code, Outcome: outcome})
		}
	}
	plan, err := (domain.RoutePlan{Decisions: decisions}).Canonical()
	if err != nil { return domain.RoutingResult{}, err }
	return router.deps.Queue.CompleteBusinessEventRoute(ctx, claim, plan)
}

func optionalPreferenceChannel(code string) (accessdomain.NotificationPreferenceChannel, bool) {
	switch code {
	case "sms": return accessdomain.NotificationPreferenceSMS, true
	case "email": return accessdomain.NotificationPreferenceEmail, true
	default: return "", false
	}
}
