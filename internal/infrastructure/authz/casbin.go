package authz

import (
	"context"
	"strings"

	"eventAI/internal/repo"

	"github.com/casbin/casbin/v2"
)

type Authorizer struct {
	enforcer *casbin.Enforcer
	events   repo.EventRepository
}

func New(modelPath string, policyPath string, events repo.EventRepository) (*Authorizer, error) {
	enforcer, err := casbin.NewEnforcer(modelPath, policyPath)
	if err != nil {
		return nil, err
	}

	return &Authorizer{
		enforcer: enforcer,
		events:   events,
	}, nil
}

func (a *Authorizer) CanEvent(ctx context.Context, userID string, eventID string, action string) (bool, error) {
	role, err := a.events.GetAccessRole(ctx, strings.TrimSpace(userID), strings.TrimSpace(eventID))
	if err != nil {
		return false, err
	}

	allowed, err := a.enforcer.Enforce(role, "event", action)
	if err != nil {
		return false, err
	}

	return allowed, nil
}
