package informers

// AddEventHandler adds event handlers to informers for pushing incremental updates
// Returns a function that can be called to remove the handlers
func (k *Informers) AddEventHandler(handler EventHandler) {
	if k.pods != nil {
		k.pods.AddEventHandler(handler)
	}
	if k.nodes != nil {
		k.nodes.AddEventHandler(handler)
	}
	if k.services != nil {
		k.services.AddEventHandler(handler)
	}
}

// EventHandler defines callbacks for resource changes
// Compatible with cache.ResourceEventHandler interface
type EventHandler interface {
	OnAdd(obj interface{}, isInInitialList bool)
	OnUpdate(oldObj, newObj interface{})
	OnDelete(obj interface{})
}
