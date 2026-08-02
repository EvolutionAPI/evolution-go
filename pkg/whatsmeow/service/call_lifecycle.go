package whatsmeow_service

import "go.mau.fi/whatsmeow"

// SetClientLifecycle injects the call coordinator without coupling the
// WhatsApp service package to the call implementation.
func (w *whatsmeowService) SetClientLifecycle(lifecycle ClientLifecycle) {
	w.clientLifecycle = lifecycle
}

func (w whatsmeowService) attachCallClient(instanceID string, client *whatsmeow.Client, prepareIncoming bool) {
	if w.clientLifecycle != nil {
		w.clientLifecycle.AttachClient(instanceID, client, prepareIncoming)
	}
}

func (w whatsmeowService) detachCallClient(instanceID string) {
	if w.clientLifecycle != nil {
		w.clientLifecycle.DetachClient(instanceID)
	}
}
