package whatsmeow_service

import (
	"encoding/json"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

func TestExtractReferralFromMessage(t *testing.T) {
	message := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: stringPtr("Hello! I want to know more about this ad."),
			ContextInfo: &waE2E.ContextInfo{
				ExternalAdReply: &waE2E.ContextInfo_ExternalAdReplyInfo{
					Title:                         stringPtr("Your Dream Farm"),
					Body:                          stringPtr("Discover exclusive rural properties in the countryside."),
					CtwaClid:                      stringPtr("FAKE_CLID_abc123xyz"),
					Ref:                           stringPtr("landing_page_01"),
					SourceApp:                     stringPtr("facebook"),
					SourceType:                    stringPtr("ad"),
					SourceID:                      stringPtr("123456789012345"),
					SourceURL:                     stringPtr("https://fb.me/fake-ad-link"),
					ShowAdAttribution:             boolPtr(true),
					ClickToWhatsappCall:           boolPtr(true),
					AutomatedGreetingMessageShown: boolPtr(true),
					GreetingMessageBody:           stringPtr("Hello! I want to know more about this ad."),
				},
			},
		},
	}

	referral := extractReferralFromMessage(message)
	if len(referral) == 0 {
		t.Fatal("expected referral payload, got empty")
	}

	var got map[string]any
	if err := json.Unmarshal(referral, &got); err != nil {
		t.Fatalf("unmarshal referral: %v", err)
	}

	if got["ctwaClid"] != "FAKE_CLID_abc123xyz" {
		t.Fatalf("expected ctwaClid to be preserved, got %#v", got["ctwaClid"])
	}

	if got["sourceURL"] != "https://fb.me/fake-ad-link" {
		t.Fatalf("expected sourceURL to be preserved, got %#v", got["sourceURL"])
	}

	if got["showAdAttribution"] != true {
		t.Fatalf("expected showAdAttribution to be preserved, got %#v", got["showAdAttribution"])
	}

	if got["automatedGreetingMessageShown"] != true {
		t.Fatalf("expected automatedGreetingMessageShown to be preserved, got %#v", got["automatedGreetingMessageShown"])
	}
}

func stringPtr(v string) *string {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
