package components

// ServiceComponent is a generic interface for YT cluster server components (like master or scheduler)
// and microservice components (like ui or chytController).
type ServiceComponent interface {
	Component
	IsPodsRemovedConditionTrue() bool
	NeedUpdate() bool
}
