package informers

// AddEventHandler adds event handlers to informers for pushing incremental updates
func (k *Informers) AddEventHandler(handler EventHandler) error {
	if k.pods != nil {
		if _, err := k.pods.AddEventHandler(handler); err != nil {
			return err
		}
	}
	if k.nodes != nil {
		if _, err := k.nodes.AddEventHandler(handler); err != nil {
			return err
		}
	}
	if k.services != nil {
		if _, err := k.services.AddEventHandler(handler); err != nil {
			return err
		}
	}
	if k.replicaSets != nil {
		if _, err := k.replicaSets.AddEventHandler(handler); err != nil {
			return err
		}
	}
	if k.deployments != nil {
		if _, err := k.deployments.AddEventHandler(handler); err != nil {
			return err
		}
	}
	return nil
}

// EventHandler defines callbacks for resource changes
// Compatible with cache.ResourceEventHandler interface
type EventHandler interface {
	OnAdd(obj interface{}, isInInitialList bool)
	OnUpdate(oldObj, newObj interface{})
	OnDelete(obj interface{})
}
