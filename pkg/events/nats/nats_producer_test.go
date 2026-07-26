package nats_producer

import "testing"

func TestNewNatsProducerDoesNotConnectWithoutURL(t *testing.T) {
	producer, ok := NewNatsProducer("  ", false, nil, nil).(*natsProducer)
	if !ok {
		t.Fatal("constructor returned an unexpected producer type")
	}
	if producer.conn != nil {
		t.Fatal("NATS connection must stay nil when NATS_URL is empty")
	}
	if producer.natsGlobalEnabled {
		t.Fatal("NATS global publishing must stay disabled without a URL")
	}
}
